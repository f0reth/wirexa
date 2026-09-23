// Package mqttapp は MQTT 接続管理ユースケースを提供する。
package mqttapp

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	cmn "github.com/f0reth/Wirexa/internal/domain"
	domain "github.com/f0reth/Wirexa/internal/domain/mqtt"
)

const (
	keyConnectionID = "connectionId"
	fieldTopic      = "topic"
)

// errShuttingDown は終了処理の開始後に接続しようとした場合に返す。
var errShuttingDown = errors.New("application is shutting down")

// connState は接続のライフサイクル状態。フロントエンドには公開せず、
// ConnectionStatus.Connected の導出とイベント発行の判定にだけ使う。
type connState int

const (
	stateConnecting connState = iota
	stateConnected
	stateDisconnecting
	stateClosed
)

// connection は 1 本の MQTT 接続。ロック順序は MQTTService.mu → opMu → stateMu の一方向に固定する。
type connection struct {
	client domain.BrokerClient
	// cancel は進行中の Connect を打ち切る。生成時に確定し以後不変なのでロック無しで読める。
	cancel context.CancelFunc
	// subs は現在購読中のトピック→QoS。リロード後の状態復元のためサーバー側で保持する。
	subs   map[string]byte
	config domain.ConnectionConfig
	// opMu は client 操作 (Publish / Subscribe / Unsubscribe / Disconnect) を直列化する。
	// ネットワーク I/O を含むため長時間保持されうる。
	opMu sync.Mutex
	// stateMu は state・subs と、この接続に関するイベント発行を保護する。
	// 状態遷移・ライフサイクルイベントは Lock、メッセージイベントと参照系は RLock で取る。
	// 「状態を見てからイベントを出す」までを 1 区間に収めるためのロックなので、
	// 保持中に client 操作 (ネットワーク I/O) を行ってはならない。
	stateMu sync.RWMutex
	state   connState
}

// terminal は切断が確定済みかを返す (stateMu 保持中に呼ぶ)。
func (c *connection) terminal() bool {
	return c.state == stateDisconnecting || c.state == stateClosed
}

// MQTTService は複数の MQTT 接続を管理するアプリケーションサービス。
//
// emitter は接続の stateMu を保持したまま呼ばれるため、MQTTService を呼び返してはならず、
// ブロックしてもならない。
type MQTTService struct {
	emitter       cmn.Emitter
	logger        cmn.Logger
	clientFactory domain.BrokerClientFactory
	// root は接続ごとの Connect 用 context の親。Shutdown で cancel する。
	root context.Context
	stop context.CancelFunc
	// conns と closed は mu で保護する。closed は mu 配下以外で読まない
	// (shutdown は各接続の terminal 状態として観測する)。
	conns map[string]*connection
	// connWg は接続 goroutine (= paho 側の接続試行) の生存を追跡する。
	connWg sync.WaitGroup
	mu     sync.RWMutex
	closed bool
}

// NewMQTTService は MQTTService を生成する。
// parent から cancel 可能なルート context を派生させ、接続ごとの Connect 用 context をその子にする。
func NewMQTTService(parent context.Context, emitter cmn.Emitter, clientFactory domain.BrokerClientFactory, logger cmn.Logger) *MQTTService {
	root, stop := context.WithCancel(parent)
	return &MQTTService{
		emitter:       emitter,
		clientFactory: clientFactory,
		logger:        logger,
		root:          root,
		stop:          stop,
		conns:         make(map[string]*connection),
	}
}

// Connect は MQTT ブローカーへの接続を開始し、接続 ID を返す。接続自体は非同期に行う。
func (s *MQTTService) Connect(config domain.ConnectionConfig) (string, error) {
	if config.Broker == "" {
		return "", &cmn.ValidationError{Field: "broker URL", Message: cmn.MsgRequired}
	}

	connID := uuid.NewString()

	// ClientID が未指定の場合は自動生成
	if config.ClientID == "" {
		config.ClientID = "wirexa-" + connID[:8]
	}

	s.logger.Info("MQTT connecting", "source", "mqtt", "broker", config.Broker, "client_id", config.ClientID)

	ctx, cancel := context.WithCancel(s.root)
	conn := &connection{config: config, subs: make(map[string]byte), cancel: cancel}
	// callback は map を引き直さず conn を直接捕捉する。引き直すと、参照後・状態確認前に
	// shutdown が割り込む窓ができる。paho は client.Connect まで callback を起動しないので、
	// callback から見て conn.client は確定済み。
	conn.client = s.clientFactory(
		config,
		func() { s.onConnected(connID, conn) },
		func(err error) { s.onConnectionLost(connID, conn, err) },
	)

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		cancel()
		return "", errShuttingDown
	}
	s.conns[connID] = conn
	// 登録と connWg への計上を同じ区間で行う。分けると、Shutdown が map を空にして
	// Wait した後に接続 goroutine が走り出す窓ができる。
	s.connWg.Go(func() { s.runConnect(ctx, connID, conn) })
	return connID, nil
}

// runConnect は接続 goroutine の本体。ロックを持たずに client.Connect を呼び、
// 結果を conn.state だけを見て確定させる。
func (s *MQTTService) runConnect(ctx context.Context, connID string, conn *connection) {
	// Connect 復帰後は ctx を使わないので、ここで資源を解放する。
	defer conn.cancel()
	err := conn.client.Connect(ctx)

	conn.stateMu.Lock()
	if conn.terminal() {
		// Disconnect / Shutdown が割り込んだ。イベントは出さない。
		conn.stateMu.Unlock()
		if err != nil {
			s.logger.Info("MQTT connect aborted", "source", "mqtt", "connection_id", connID, "error", err)
			return
		}
		// 追跡外の接続を残さないための保険。割り込んだ側も切断するので二重になるが安全。
		s.logger.Info("MQTT connected after disconnect, discarding", "source", "mqtt", "connection_id", connID)
		conn.opMu.Lock()
		conn.client.Disconnect(0)
		conn.opMu.Unlock()
		return
	}
	if err != nil {
		conn.state = stateClosed
		s.logger.Error("MQTT connection failed", "source", "mqtt", "connection_id", connID, "error", err)
		s.emitter.Emit(cmn.EventMQTTConnectionFailed, map[string]any{
			keyConnectionID: connID,
			"error":         err.Error(),
		})
		conn.stateMu.Unlock()

		s.mu.Lock()
		if s.conns[connID] == conn {
			delete(s.conns, connID)
		}
		s.mu.Unlock()
		return
	}
	// paho は onConnected を Connect の復帰より先に起動し得るため、既に stateConnected なのが正常系。
	if conn.state == stateConnecting {
		conn.state = stateConnected
	}
	conn.stateMu.Unlock()
}

// onConnected は接続確立 (自動再接続を含む) のたびに呼ばれる。
func (s *MQTTService) onConnected(connID string, conn *connection) {
	conn.stateMu.Lock()
	defer conn.stateMu.Unlock()
	if conn.terminal() {
		return
	}
	conn.state = stateConnected
	s.logger.Info("MQTT connected", "source", "mqtt", "connection_id", connID, "broker", conn.config.Broker)
	s.emitter.Emit(cmn.EventMQTTConnected, map[string]any{
		keyConnectionID: connID,
	})
}

// onConnectionLost は確立済み接続が予期せず切れたときに呼ばれる。paho が自動再接続するので状態は変えない。
func (s *MQTTService) onConnectionLost(connID string, conn *connection, err error) {
	conn.stateMu.Lock()
	defer conn.stateMu.Unlock()
	if conn.terminal() {
		return
	}
	s.logger.Error("MQTT connection lost", "source", "mqtt", "connection_id", connID, "error", err)
	s.emitter.Emit(cmn.EventMQTTConnectionLost, map[string]any{
		keyConnectionID: connID,
		"error":         err.Error(),
	})
}

// Disconnect は指定した接続を切断する。実行中の client 操作があれば、その完了を待ってから切断する。
func (s *MQTTService) Disconnect(connectionID string) error {
	conn, err := s.detach(connectionID)
	if err != nil {
		return err
	}
	// 進行中の Connect を打ち切る。接続 goroutine の完了は待たない。
	conn.cancel()

	conn.opMu.Lock()
	defer conn.opMu.Unlock()
	conn.client.Disconnect(1000)

	conn.stateMu.Lock()
	defer conn.stateMu.Unlock()
	conn.state = stateClosed
	s.logger.Info("MQTT disconnected", "source", "mqtt", "connection_id", connectionID)
	s.emitter.Emit(cmn.EventMQTTDisconnected, map[string]any{
		keyConnectionID: connectionID,
	})
	return nil
}

// detach は接続を map から外し、同じ区間で stateDisconnecting へ遷移させる。
// 遷移を opMu の取得より前に行うことで、opMu 待ちの操作は状態を見て即座に弾かれ、
// Disconnect が待つのは実行中の 1 操作だけになる。
func (s *MQTTService) detach(id string) (*connection, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	conn, ok := s.conns[id]
	if !ok {
		return nil, &cmn.NotFoundError{Resource: cmn.ResourceConnection, ID: id}
	}
	conn.stateMu.Lock()
	defer conn.stateMu.Unlock()
	// 接続失敗経路が stateClosed にしてから map から外すまでの隙間。
	if conn.terminal() {
		return nil, &cmn.NotFoundError{Resource: cmn.ResourceConnection, ID: id}
	}
	conn.state = stateDisconnecting
	delete(s.conns, id)
	return conn, nil
}

// withConn は接続を引いて opMu を取り、切断が確定していないことを確認してから fn を呼ぶ。
// fn の実行中は同じ接続の Disconnect が client を切断しない。
func (s *MQTTService) withConn(id string, fn func(conn *connection) error) error {
	s.mu.RLock()
	conn, ok := s.conns[id]
	s.mu.RUnlock()
	if !ok {
		return &cmn.NotFoundError{Resource: cmn.ResourceConnection, ID: id}
	}
	conn.opMu.Lock()
	defer conn.opMu.Unlock()
	// ロック取得を待つ間に切断が確定した接続は操作しない。
	conn.stateMu.RLock()
	closed := conn.terminal()
	conn.stateMu.RUnlock()
	if closed {
		return &cmn.NotFoundError{Resource: cmn.ResourceConnection, ID: id}
	}
	return fn(conn)
}

// Publish は指定トピックへメッセージを送信する。
func (s *MQTTService) Publish(connectionID, topic, payload string, qos byte, retain bool) error {
	if topic == "" {
		return &cmn.ValidationError{Field: fieldTopic, Message: cmn.MsgRequired}
	}
	if qos > 2 {
		return &cmn.ValidationError{Field: "qos", Message: "must be 0, 1, or 2"}
	}
	return s.withConn(connectionID, func(conn *connection) error {
		if err := conn.client.Publish(topic, qos, retain, payload); err != nil {
			return fmt.Errorf("failed to publish: %w", err)
		}
		return nil
	})
}

// Subscribe は指定トピックの購読を開始する。
func (s *MQTTService) Subscribe(connectionID, topic string, qos byte) error {
	if topic == "" {
		return &cmn.ValidationError{Field: fieldTopic, Message: cmn.MsgRequired}
	}
	if qos > 2 {
		return &cmn.ValidationError{Field: "qos", Message: "must be 0, 1, or 2"}
	}
	return s.withConn(connectionID, func(conn *connection) error {
		handler := func(msgTopic string, msgPayload []byte, msgQoS byte, retained bool) {
			// RLock なのでメッセージ同士は直列化しない。状態遷移 (Lock) は実行中の発行の完了を待つ。
			conn.stateMu.RLock()
			defer conn.stateMu.RUnlock()
			if conn.terminal() {
				return // 切断・shutdown 済みの接続のメッセージは捨てる
			}
			s.logger.Info("MQTT message received", "source", "mqtt", "connection_id", connectionID, "topic", msgTopic, "payload_bytes", len(msgPayload))
			// 非 UTF-8 のバイナリペイロードは string 変換で壊れるため base64 で渡す。
			payloadStr, payloadBase64 := cmn.EncodeMaybeBase64(msgPayload)
			s.emitter.Emit(cmn.EventMQTTMessage, domain.MQTTMessage{
				ConnectionID:  connectionID,
				Topic:         msgTopic,
				Payload:       payloadStr,
				PayloadBase64: payloadBase64,
				QoS:           msgQoS,
				Retained:      retained,
				Timestamp:     time.Now().UnixMilli(),
			})
		}
		if err := conn.client.Subscribe(topic, qos, handler); err != nil {
			return fmt.Errorf("failed to subscribe: %w", err)
		}
		conn.stateMu.Lock()
		conn.subs[topic] = qos
		conn.stateMu.Unlock()
		return nil
	})
}

// Unsubscribe は指定トピックの購読を解除する。
func (s *MQTTService) Unsubscribe(connectionID, topic string) error {
	if topic == "" {
		return &cmn.ValidationError{Field: fieldTopic, Message: cmn.MsgRequired}
	}
	return s.withConn(connectionID, func(conn *connection) error {
		if err := conn.client.Unsubscribe(topic); err != nil {
			return fmt.Errorf("failed to unsubscribe: %w", err)
		}
		conn.stateMu.Lock()
		delete(conn.subs, topic)
		conn.stateMu.Unlock()
		return nil
	})
}

// GetConnections は全接続の現在状態を返す。切断が確定した接続は含めない。
// opMu を取らないので、実行中の client 操作があっても待たされない。
func (s *MQTTService) GetConnections() []domain.ConnectionStatus {
	s.mu.RLock()
	ids := make([]string, 0, len(s.conns))
	conns := make([]*connection, 0, len(s.conns))
	for id, conn := range s.conns {
		ids = append(ids, id)
		conns = append(conns, conn)
	}
	s.mu.RUnlock()

	statuses := make([]domain.ConnectionStatus, 0, len(conns))
	for i, conn := range conns {
		conn.stateMu.RLock()
		if conn.terminal() {
			conn.stateMu.RUnlock()
			continue
		}
		established := conn.state == stateConnected
		subs := make([]domain.SubscriptionInfo, 0, len(conn.subs))
		for topic, qos := range conn.subs {
			subs = append(subs, domain.SubscriptionInfo{Topic: topic, QoS: qos})
		}
		conn.stateMu.RUnlock()
		statuses = append(statuses, domain.ConnectionStatus{
			ID:            ids[i],
			Name:          conn.config.Name,
			Broker:        conn.config.Broker,
			Connected:     established && conn.client.IsConnected(),
			ProfileID:     conn.config.ProfileID,
			Subscriptions: subs,
		})
	}
	return statuses
}

// Shutdown は全接続を先に終端状態にしてイベントを止め、進行中の Connect を打ち切ってから
// 切断し、接続 goroutine の完了を合計 timeout まで待つ。以後の Connect は拒否する。
// true は全接続を切断し全接続 goroutine が復帰したこと (paho 側の接続試行も終了済み)、
// false は上限内に排水できなかったことを表す。false でもイベント発行と状態変更は止まっており、
// 残った接続試行は BrokerClient.Connect の契約により接続を確立せずに終わる。
// アプリケーションのライフサイクルは合成ルートの責務なので、RPC 面には公開しない。
func (s *MQTTService) Shutdown(timeout time.Duration) bool {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return true
	}
	s.closed = true
	// map から外すのと terminal への遷移を同じ区間で行う。遷移の Lock は実行中のイベント発行の
	// 完了を待つので、この区間を抜けた後はこの service からイベントは発行されない。
	conns := make([]*connection, 0, len(s.conns))
	for _, conn := range s.conns {
		conn.stateMu.Lock()
		if !conn.terminal() {
			conn.state = stateDisconnecting
			conns = append(conns, conn)
		}
		conn.stateMu.Unlock()
	}
	clear(s.conns)
	s.mu.Unlock()

	// 先に全 Connect を打ち切るので、接続試行の完了を待ってから切断する必要はない。
	s.stop()

	done := make(chan struct{})
	go func() {
		var wg sync.WaitGroup
		for _, conn := range conns {
			wg.Go(func() {
				conn.opMu.Lock()
				defer conn.opMu.Unlock()
				conn.client.Disconnect(1000)
				conn.stateMu.Lock()
				conn.state = stateClosed
				conn.stateMu.Unlock()
			})
		}
		wg.Wait()
		// 接続 goroutine は paho の接続試行の終了まで復帰しないので、
		// これが「paho 側に接続試行が残っていない」ことの根拠になる。
		s.connWg.Wait()
		close(done)
	}()

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-done:
		return true
	case <-timer.C:
		return false
	}
}
