package mqttapp

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/google/uuid"

	cmn "github.com/f0reth/Wirexa/internal/domain"
	domain "github.com/f0reth/Wirexa/internal/domain/mqtt"
)

const (
	// scanFilter は Broker Topics のスキャンが購読するフィルター。
	scanFilter = "#"
	// scanClientIDPrefix はスキャン用クライアントのクライアント ID の接頭辞。
	scanClientIDPrefix = "wirexa-scan-"
	// scanQuiesce はスキャン用クライアントを切断するときの待機時間 (ms)。
	scanQuiesce = 250
)

// errScanStopped は、開始の途中で止められたスキャン (StopTopicScan・切断・終了処理) の開始が返す。
var errScanStopped = errors.New("topic scan was stopped")

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

// StartTopicScan は Broker Topics のスキャンを始める。専用のクライアントで接続して # を購読し、
// 受信したトピックを mqtt:scan-topic で発行する。接続と購読が済んでから返る。
// 元の接続が確立前でも始められる (スキャン用の接続は独立している)。
// 既にスキャンがあれば (開始中・稼働中) クライアントを増やさず、その開始の結果を返す。
// opMu は取らないので、スキャンの接続待ちが Publish / Subscribe を止めない。
func (s *MQTTService) StartTopicScan(connectionID string) error {
	r, err := s.reserveScan(connectionID)
	if err != nil {
		return err
	}
	conn, scan := r.conn, r.scan
	if r.joined {
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
	err = scan.client.Connect(r.ctx)
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

// scanReservation は reserveScan の結果。
type scanReservation struct {
	// ctx は新しく予約したスキャンの Connect 用 context。joined なら nil。
	ctx  context.Context
	conn *connection
	scan *topicScan
	// joined は既にスキャンがあり (開始中・稼働中)、新しく始めずにその開始の結果を待つことを表す。
	joined bool
}

// reserveScan は I/O の前にスキャンを予約する。予約できたら Connect 用の ctx を返し、connWg に計上する
// (呼び出し元が Done する)。既にスキャンがあればそれを joined で返し、connWg には計上しない。
// 予約と connWg への計上は、Connect と同じく mu の保持中に closed を確かめてから行う。
// 分けると、Shutdown が Wait した後に計上する窓ができる。
func (s *MQTTService) reserveScan(connectionID string) (scanReservation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return scanReservation{}, errShuttingDown
	}
	conn, ok := s.conns[connectionID]
	if !ok {
		return scanReservation{}, connNotFound(connectionID)
	}
	conn.stateMu.Lock()
	defer conn.stateMu.Unlock()
	if conn.terminal() {
		return scanReservation{}, connNotFound(connectionID)
	}
	if conn.scan != nil {
		return scanReservation{conn: conn, scan: conn.scan, joined: true}, nil
	}
	ctx, cancel := context.WithCancel(s.root)
	scan := &topicScan{cancel: cancel, done: make(chan struct{})}
	conn.scan = scan
	s.connWg.Add(1)
	return scanReservation{ctx: ctx, conn: conn, scan: scan}, nil
}

// StopTopicScan は Broker Topics のスキャンを止める。スキャンしていなければ何もしない。
// 開始中のスキャンは打ち切り、その StartTopicScan はエラーを返す。
func (s *MQTTService) StopTopicScan(connectionID string) error {
	conn, err := s.lookup(connectionID)
	if err != nil {
		return err
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
	s.emitter.Emit(cmn.EventMQTTScanStopped, domain.ConnectionErrorEvent{ConnectionID: connID, Error: err.Error()})
	conn.stateMu.Unlock()
	scan.client.Disconnect(0)
}
