package mqttapp

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"

	cmn "github.com/f0reth/Wirexa/internal/domain"
	domain "github.com/f0reth/Wirexa/internal/domain/mqtt"
)

const keyConnectionID = "connectionId"

const (
	// scanFilter は Broker Topics のスキャンが購読するフィルター。
	scanFilter = "#"
	// scanClientIDPrefix はスキャン用クライアントのクライアント ID の接頭辞。
	scanClientIDPrefix = "wirexa-scan-"
	// scanQuiesce はスキャン用クライアントを切断するときの待機時間 (ms)。
	scanQuiesce = 250
)

// errShuttingDown は終了処理の開始後に接続しようとした場合に返す。
var errShuttingDown = errors.New("application is shutting down")

// errScanStopped は、開始の途中で止められたスキャン (StopTopicScan・切断・終了処理) の開始が返す。
var errScanStopped = errors.New("topic scan was stopped")

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
	// subs は購読中のトピック→QoS。リロード後の状態復元と、接続確立時・再接続時の張り直しのために
	// サーバー側で保持する。接続確立前に受け付けた購読 (確立時に購読する) も含む。
	subs map[string]byte
	// subOrder は subs のトピックを購読した順に並べたもの。GetConnections と張り直しの順序を決める
	// (map の反復順に任せると、リロードのたびに購読一覧の並びが変わる)。subs と一緒に setSub / deleteSub で変える。
	subOrder []string
	config   domain.ConnectionConfig
	// seq は接続の作成順。登録時に確定し以後不変なのでロック無しで読める。
	seq uint64
	// opMu は client 操作 (Publish / Subscribe / Unsubscribe / Disconnect と onConnected の張り直し) を直列化する。
	// ネットワーク I/O を含むため長時間保持されうる。
	opMu sync.Mutex
	// stateMu は state・subs と、この接続に関するイベント発行を保護する。
	// 状態遷移・ライフサイクルイベントは Lock、メッセージイベントと参照系は RLock で取る。
	// 「状態を見てからイベントを出す」までを 1 区間に収めるためのロックなので、
	// 保持中に client 操作 (ネットワーク I/O) を行ってはならない。
	stateMu sync.RWMutex
	state   connState
	// scan は Broker Topics のスキャン。nil はスキャンなし、非 nil は開始中か稼働中 (topicScan.started)。
	// stateMu で保護する。終端状態の接続では常に nil。
	scan *topicScan
}

// topicScan は Broker Topics のスキャン 1 回分。元の接続とは別のクライアント (別のクライアント ID) で
// # を購読する。同じ接続で購読すると全ての購読と重なり、重なる購読ごとに PUBLISH を送る
// ブローカーではメッセージが重複して届く。
type topicScan struct {
	// client は開始した goroutine が I/O の前に入れる。他の goroutine は started が true の間だけ触る。
	client domain.BrokerClient
	// cancel は進行中の Connect を打ち切る。
	cancel context.CancelFunc
	// done は開始 (接続と購読) の完了で閉じる。err はその結果で、done を閉じた後にだけ読む。
	done chan struct{}
	err  error
	// lostErr は開始中に接続が切れたときの原因 (stateMu で保護)。
	lostErr error
	// pending は開始中に届いたトピック。稼働中になったときに発行する (stateMu の保持中に pendingMu を取る)。
	pending   []string
	pendingMu sync.Mutex
	// started は稼働中 (接続と購読が済んだ) かを表す (stateMu で保護)。
	started bool
}

// detachScan はスキャンを外して進行中の接続を打ち切り、切断が要るクライアントを返す (stateMu 保持中に呼ぶ)。
// 稼働中だったらそのクライアントを返し、呼び出し元が stateMu を放してから切断する。
// 開始中だったら nil を返す。接続試行の途中のクライアントは別の goroutine から切断せず、
// StartTopicScan が conn.scan から外されたのを見て自分で切断する。
func (c *connection) detachScan() domain.BrokerClient {
	scan := c.scan
	if scan == nil {
		return nil
	}
	c.scan = nil
	scan.cancel()
	if !scan.started {
		return nil
	}
	return scan.client
}

// disconnectScan は detachScan が返したスキャン用クライアントを切断する。nil なら何もしない。
func disconnectScan(client domain.BrokerClient) {
	if client != nil {
		client.Disconnect(scanQuiesce)
	}
}

// setSub は購読を登録する (stateMu 保持中に呼ぶ)。新しいトピックは購読順の末尾に足し、
// 購読中のトピックは QoS だけを変えて位置を保つ。
func (c *connection) setSub(topic string, qos byte) {
	if _, ok := c.subs[topic]; !ok {
		c.subOrder = append(c.subOrder, topic)
	}
	c.subs[topic] = qos
}

// deleteSub は購読を外す (stateMu 保持中に呼ぶ)。
func (c *connection) deleteSub(topic string) {
	if _, ok := c.subs[topic]; !ok {
		return
	}
	delete(c.subs, topic)
	c.subOrder = slices.DeleteFunc(c.subOrder, func(t string) bool { return t == topic })
}

// orderedSubs は購読を購読した順に複製して返す (stateMu 保持中に呼ぶ)。
func (c *connection) orderedSubs() []domain.SubscriptionInfo {
	subs := make([]domain.SubscriptionInfo, 0, len(c.subOrder))
	for _, topic := range c.subOrder {
		subs = append(subs, domain.SubscriptionInfo{Topic: topic, QoS: c.subs[topic]})
	}
	return subs
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
	// connWg は接続 goroutine とスキャンの開始 (= paho 側の接続試行) の生存を追跡する。
	connWg sync.WaitGroup
	mu     sync.RWMutex
	// nextSeq は次に登録する接続の作成順 (mu で保護)。
	nextSeq uint64
	closed  bool
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
	if err := domain.ValidateBroker(config.Broker, config.UseTLS); err != nil {
		return "", err
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
	conn.seq = s.nextSeq
	s.nextSeq++
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
		scanClient := conn.detachScan()
		s.logger.Error("MQTT connection failed", "source", "mqtt", "connection_id", connID, "error", err)
		s.emitter.Emit(cmn.EventMQTTConnectionFailed, map[string]any{
			keyConnectionID: connID,
			"error":         err.Error(),
		})
		conn.stateMu.Unlock()
		disconnectScan(scanClient)

		s.mu.Lock()
		if s.conns[connID] == conn {
			delete(s.conns, connID)
		}
		s.mu.Unlock()
		return
	}
	// paho は onConnected を Connect の復帰より先に起動し得るため、既に stateConnected なのが正常系。
	// 逆に onConnected より先にここで遷移した場合、その間の Subscribe は client を直接呼び、
	// onConnected が同じ購読をもう一度送る (QoS は揃うが、retained メッセージが重ねて届き得る)。
	if conn.state == stateConnecting {
		conn.state = stateConnected
	}
	conn.stateMu.Unlock()
}

// onConnected は接続確立 (自動再接続を含む) のたびに呼ばれ、subs の購読を張り直す。
// 初回の確立では、確立前に受け付けた購読をここで初めて購読する。
// 状態遷移から張り直しの完了まで opMu を保持し、Subscribe / Unsubscribe と直列化する。
// 間に Subscribe が割り込むと、複製した古い QoS で張り直したり、同じ購読を重ねて送ったりするため。
func (s *MQTTService) onConnected(connID string, conn *connection) {
	conn.opMu.Lock()
	defer conn.opMu.Unlock()
	conn.stateMu.Lock()
	if conn.terminal() {
		conn.stateMu.Unlock()
		return
	}
	conn.state = stateConnected
	s.logger.Info("MQTT connected", "source", "mqtt", "connection_id", connID, "broker", conn.config.Broker)
	s.emitter.Emit(cmn.EventMQTTConnected, map[string]any{
		keyConnectionID: connID,
	})
	subs := conn.orderedSubs()
	conn.stateMu.Unlock()

	// stateMu を放してから張り直す (stateMu の保持中は client 操作をしない)。
	if len(subs) > 0 {
		s.resubscribe(connID, conn, subs)
	}
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
	conn, scanClient, err := s.detach(connectionID)
	if err != nil {
		return err
	}
	// 進行中の Connect を打ち切る。接続 goroutine の完了は待たない。
	conn.cancel()
	disconnectScan(scanClient)

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
// スキャンも同じ区間で外し、切断が要るスキャン用クライアントを返す (無ければ nil)。
func (s *MQTTService) detach(id string) (*connection, domain.BrokerClient, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	conn, ok := s.conns[id]
	if !ok {
		return nil, nil, &cmn.NotFoundError{Resource: cmn.ResourceConnection, ID: id}
	}
	conn.stateMu.Lock()
	defer conn.stateMu.Unlock()
	// 接続失敗経路が stateClosed にしてから map から外すまでの隙間。
	if conn.terminal() {
		return nil, nil, &cmn.NotFoundError{Resource: cmn.ResourceConnection, ID: id}
	}
	conn.state = stateDisconnecting
	scanClient := conn.detachScan()
	delete(s.conns, id)
	return conn, scanClient, nil
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
	if err := domain.ValidateTopicName(topic); err != nil {
		return err
	}
	if err := domain.ValidateQoS(qos); err != nil {
		return err
	}
	return s.withConn(connectionID, func(conn *connection) error {
		if err := conn.client.Publish(topic, qos, retain, payload); err != nil {
			return fmt.Errorf("failed to publish: %w", err)
		}
		return nil
	})
}

// Subscribe は指定トピックの購読を開始する。接続の確立前なら登録だけ行い、確立時に購読する。
func (s *MQTTService) Subscribe(connectionID, topic string, qos byte) error {
	if err := domain.ValidateTopicFilter(topic); err != nil {
		return err
	}
	if err := domain.ValidateQoS(qos); err != nil {
		return err
	}
	return s.withConn(connectionID, func(conn *connection) error {
		if conn.updatePendingSubs(func() { conn.setSub(topic, qos) }) {
			return nil
		}
		if err := conn.client.Subscribe(topic, qos, s.messageHandler(connectionID, conn)); err != nil {
			return fmt.Errorf("failed to subscribe: %w", err)
		}
		conn.stateMu.Lock()
		conn.setSub(topic, qos)
		conn.stateMu.Unlock()
		return nil
	})
}

// updatePendingSubs は接続の確立前 (stateConnecting) なら client を呼ばずに update で subs だけを変え、
// true を返す。確立前の client は購読を受け付けない (paho は ErrNotConnected を返す) ので、
// 実際の購読・解除は onConnected の張り直しに任せる。確立済みなら何もせず false を返す。
// opMu の保持中 (withConn の中) に呼ぶ。
func (c *connection) updatePendingSubs(update func()) bool {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()
	if c.state != stateConnecting {
		return false
	}
	update()
	return true
}

// messageHandler は受信したメッセージを mqtt:message イベントとして発行するハンドラを返す。
func (s *MQTTService) messageHandler(connectionID string, conn *connection) domain.MessageHandler {
	return func(msgTopic string, msgPayload []byte, msgQoS byte, retained bool) {
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
}

// resubscribe は接続の確立後に subs の購読を購読した順に張り直す。client は CleanSession で接続するので、
// 再接続したブローカー側には前回の購読が残っていない。張り直さないと、GetConnections は
// 購読中と返し続けるのにメッセージが届かなくなる。確立前に受け付けた購読もここで購読する。
// onConnected (client のコールバック用 goroutine) から、opMu を保持したまま呼ぶ。
func (s *MQTTService) resubscribe(connID string, conn *connection, subs []domain.SubscriptionInfo) {
	for _, sub := range subs {
		topic, qos := sub.Topic, sub.QoS
		// 途中で切断された接続 (Disconnect の detach は opMu を取らずに遷移させる) と、
		// Unsubscribe で外された購読は張り直さない。
		conn.stateMu.RLock()
		_, still := conn.subs[topic]
		closed := conn.terminal()
		conn.stateMu.RUnlock()
		if closed {
			return
		}
		if !still {
			continue
		}
		err := conn.client.Subscribe(topic, qos, s.messageHandler(connID, conn))
		if err == nil {
			continue
		}
		s.logger.Error("MQTT resubscribe failed", "source", "mqtt", "connection_id", connID, "topic", topic, "error", err)
		// ブローカーが拒否した購読と、接続が開いたまま失敗した購読 (SUBACK を時間内に確認できなかった等) は
		// 表示から外す。後者は次の再接続が来ないので、残すと購読中と表示されたままメッセージが届かない。
		// 接続が切れて失敗した購読は残し、次の再接続で張り直す。
		if errors.Is(err, domain.ErrSubscriptionRejected) || conn.client.IsConnected() {
			conn.stateMu.Lock()
			conn.deleteSub(topic)
			conn.stateMu.Unlock()
			// client は張り直しの失敗では振り分け先を残すので、解除して片付ける。ブローカーが購読を
			// 受理していた場合も、これでブローカー側の購読が消える。
			// 失敗しても (直後にまた切断した等) 表示からは外したままにする。振り分け先が無ければ
			// 届いたメッセージは捨てられるので、表示とは食い違わない。
			if err := conn.client.Unsubscribe(topic); err != nil {
				s.logger.Error("MQTT unsubscribe of a failed subscription failed", "source", "mqtt", "connection_id", connID, "topic", topic, "error", err)
			}
		}
	}
}

// Unsubscribe は指定トピックの購読を解除する。接続の確立前なら登録を外すだけで client は呼ばない。
// ブローカーの応答を確認できなかった場合 (domain.ErrAckTimeout) はエラーを返すが、購読は外す。
func (s *MQTTService) Unsubscribe(connectionID, topic string) error {
	if err := domain.ValidateTopicFilter(topic); err != nil {
		return err
	}
	return s.withConn(connectionID, func(conn *connection) error {
		if conn.updatePendingSubs(func() { conn.deleteSub(topic) }) {
			return nil
		}
		err := conn.client.Unsubscribe(topic)
		// 応答を確認できなかった解除は、client が振り分け先を外しているので購読も外す
		// (残すと購読中と表示されたままメッセージが届かない)。それ以外の失敗では購読を残す。
		if err == nil || errors.Is(err, domain.ErrAckTimeout) {
			conn.stateMu.Lock()
			conn.deleteSub(topic)
			conn.stateMu.Unlock()
		}
		if err != nil {
			return fmt.Errorf("failed to unsubscribe: %w", err)
		}
		return nil
	})
}

// StartTopicScan は Broker Topics のスキャンを始める。専用のクライアントで接続して # を購読し、
// 受信したトピックを mqtt:scan-topic で発行する。接続と購読が済んでから返る。
// 元の接続が確立前でも始められる (スキャン用の接続は独立している)。
// 既にスキャンがあれば (開始中・稼働中) クライアントを増やさず、その開始の結果を返す。
// opMu は取らないので、スキャンの接続待ちが Publish / Subscribe を止めない。
func (s *MQTTService) StartTopicScan(connectionID string) error {
	conn, scan, ctx, err := s.reserveScan(connectionID)
	if err != nil {
		return err
	}
	if ctx == nil {
		<-scan.done
		return scan.err
	}
	defer s.connWg.Done()

	// 同じクライアント ID で接続すると、ブローカーが元の接続を切る。
	cfg := conn.config
	cfg.ClientID = scanClientIDPrefix + uuid.NewString()[:8]
	// スキャン用クライアントは張り直しをしないので、onConnected では何もしない。
	scan.client = s.clientFactory(
		cfg,
		func() {},
		func(err error) { s.onScanConnectionLost(connectionID, conn, scan, err) },
	)
	err = scan.client.Connect(ctx)
	// Connect 復帰後は ctx を使わないので、ここで資源を解放する。
	scan.cancel()
	if err != nil {
		err = fmt.Errorf("failed to connect for topic scan: %w", err)
	} else if subErr := scan.client.Subscribe(scanFilter, 0, s.scanHandler(connectionID, conn, scan)); subErr != nil {
		err = fmt.Errorf("failed to subscribe for topic scan: %w", subErr)
	}

	conn.stateMu.Lock()
	switch {
	case conn.scan != scan:
		// StopTopicScan・切断・終了処理が外した。I/O が成功していても捨てる。
		err = errScanStopped
	case err == nil && scan.lostErr != nil:
		err = fmt.Errorf("topic scan connection lost: %w", scan.lostErr)
	}
	if err == nil {
		scan.started = true
		s.logger.Info("MQTT topic scan started", "source", "mqtt", "connection_id", connectionID)
		// 開始中に届いたトピック (SUBACK の直後に届く retained メッセージなど) を発行する。
		for _, topic := range scan.pending {
			s.emitter.Emit(cmn.EventMQTTScanTopic, domain.ScannedTopic{ConnectionID: connectionID, Topic: topic})
		}
	} else if conn.scan == scan {
		conn.scan = nil
	}
	scan.pending = nil
	conn.stateMu.Unlock()

	if err != nil {
		s.logger.Error("MQTT topic scan failed to start", "source", "mqtt", "connection_id", connectionID, "error", err)
		scan.client.Disconnect(0)
	}
	scan.err = err
	close(scan.done)
	return err
}

// reserveScan は I/O の前にスキャンを予約する。予約できたら Connect 用の ctx を返し、connWg に計上する
// (呼び出し元が Done する)。既にスキャンがあればそれを返し、ctx は nil。
// 予約と connWg への計上は、Connect と同じく mu の保持中に closed を確かめてから行う。
// 分けると、Shutdown が Wait した後に計上する窓ができる。
func (s *MQTTService) reserveScan(connectionID string) (*connection, *topicScan, context.Context, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil, nil, nil, errShuttingDown
	}
	conn, ok := s.conns[connectionID]
	if !ok {
		return nil, nil, nil, &cmn.NotFoundError{Resource: cmn.ResourceConnection, ID: connectionID}
	}
	conn.stateMu.Lock()
	defer conn.stateMu.Unlock()
	if conn.terminal() {
		return nil, nil, nil, &cmn.NotFoundError{Resource: cmn.ResourceConnection, ID: connectionID}
	}
	if conn.scan != nil {
		return conn, conn.scan, nil, nil
	}
	ctx, cancel := context.WithCancel(s.root)
	scan := &topicScan{cancel: cancel, done: make(chan struct{})}
	conn.scan = scan
	s.connWg.Add(1)
	return conn, scan, ctx, nil
}

// StopTopicScan は Broker Topics のスキャンを止める。スキャンしていなければ何もしない。
// 開始中のスキャンは打ち切り、その StartTopicScan はエラーを返す。
func (s *MQTTService) StopTopicScan(connectionID string) error {
	s.mu.RLock()
	conn, ok := s.conns[connectionID]
	s.mu.RUnlock()
	if !ok {
		return &cmn.NotFoundError{Resource: cmn.ResourceConnection, ID: connectionID}
	}
	conn.stateMu.Lock()
	stopped := conn.scan != nil
	scanClient := conn.detachScan()
	conn.stateMu.Unlock()
	if stopped {
		s.logger.Info("MQTT topic scan stopped", "source", "mqtt", "connection_id", connectionID)
	}
	disconnectScan(scanClient)
	return nil
}

// scanHandler はスキャン用クライアントが受信したメッセージのトピックを mqtt:scan-topic として発行する
// ハンドラを返す。ペイロードは運ばない。止めた後 (StopTopicScan・切断・終了処理) は発行しない。
func (s *MQTTService) scanHandler(connectionID string, conn *connection, scan *topicScan) domain.MessageHandler {
	return func(msgTopic string, _ []byte, _ byte, _ bool) {
		conn.stateMu.RLock()
		defer conn.stateMu.RUnlock()
		if conn.scan != scan || conn.terminal() {
			return
		}
		if !scan.started {
			// 開始が確定するまでは発行しない。稼働中になったときに StartTopicScan が発行する。
			scan.pendingMu.Lock()
			scan.pending = append(scan.pending, msgTopic)
			scan.pendingMu.Unlock()
			return
		}
		s.emitter.Emit(cmn.EventMQTTScanTopic, domain.ScannedTopic{ConnectionID: connectionID, Topic: msgTopic})
	}
}

// onScanConnectionLost はスキャン用の接続が切れたときに呼ばれ、スキャンを終える (自動では再開しない)。
// client は自動再接続を始めるので、Disconnect して止める。
func (s *MQTTService) onScanConnectionLost(connID string, conn *connection, scan *topicScan, err error) {
	conn.stateMu.Lock()
	if conn.scan != scan {
		// 止めた後 (StopTopicScan・切断・終了処理)。切断は止めた側が行う。
		conn.stateMu.Unlock()
		return
	}
	if !scan.started {
		// 開始中。イベントは出さず、StartTopicScan が失敗として扱って切断する。
		scan.lostErr = err
		conn.stateMu.Unlock()
		return
	}
	conn.scan = nil
	s.logger.Error("MQTT topic scan connection lost", "source", "mqtt", "connection_id", connID, "error", err)
	s.emitter.Emit(cmn.EventMQTTScanStopped, map[string]any{
		keyConnectionID: connID,
		"error":         err.Error(),
	})
	conn.stateMu.Unlock()
	scan.client.Disconnect(0)
}

// GetConnections は全接続の現在状態を、接続は作成順、購読は購読した順で返す。
// 切断が確定した接続は含めない。
// Subscriptions には接続確立前に受け付けた購読 (確立時に購読する) も含む。
// Scanning はスキャンが稼働中のときだけ true (開始中は false)。
// opMu を取らないので、実行中の client 操作があっても待たされない。
func (s *MQTTService) GetConnections() []domain.ConnectionStatus {
	type entry struct {
		conn *connection
		id   string
	}
	s.mu.RLock()
	entries := make([]entry, 0, len(s.conns))
	for id, conn := range s.conns {
		entries = append(entries, entry{conn: conn, id: id})
	}
	s.mu.RUnlock()
	slices.SortFunc(entries, func(a, b entry) int { return cmp.Compare(a.conn.seq, b.conn.seq) })

	statuses := make([]domain.ConnectionStatus, 0, len(entries))
	for _, e := range entries {
		conn := e.conn
		conn.stateMu.RLock()
		if conn.terminal() {
			conn.stateMu.RUnlock()
			continue
		}
		established := conn.state == stateConnected
		scanning := conn.scan != nil && conn.scan.started
		subs := conn.orderedSubs()
		conn.stateMu.RUnlock()
		statuses = append(statuses, domain.ConnectionStatus{
			ID:            e.id,
			Name:          conn.config.Name,
			Broker:        conn.config.Broker,
			Connected:     established && conn.client.IsConnected(),
			ProfileID:     conn.config.ProfileID,
			Subscriptions: subs,
			Scanning:      scanning,
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
	var scanClients []domain.BrokerClient
	for _, conn := range s.conns {
		conn.stateMu.Lock()
		if !conn.terminal() {
			conn.state = stateDisconnecting
			conns = append(conns, conn)
			if scanClient := conn.detachScan(); scanClient != nil {
				scanClients = append(scanClients, scanClient)
			}
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
		for _, scanClient := range scanClients {
			wg.Go(func() { disconnectScan(scanClient) })
		}
		wg.Wait()
		// 接続 goroutine とスキャンの開始は paho の接続試行の終了まで復帰しないので、
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
