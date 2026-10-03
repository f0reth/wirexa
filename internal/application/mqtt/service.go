package mqttapp

import (
	"cmp"
	"context"
	"errors"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"

	cmn "github.com/f0reth/Wirexa/internal/domain"
	domain "github.com/f0reth/Wirexa/internal/domain/mqtt"
)

// disconnectQuiesce は接続を切断する (Disconnect・Shutdown) ときの待機時間 (ms)。
// 不要になったクライアントを捨てるだけの経路 (割り込まれた接続・失敗したスキャン) は Disconnect(0) で待たない。
const disconnectQuiesce = 1000

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
	// closing は detach で conns から外し、Disconnect がまだ終わっていない接続 (mu で保護)。
	// Shutdown はこれらも終端状態にして、実行中の Disconnect のイベントを止める。
	closing map[*connection]struct{}
	// connWg は接続 goroutine・スキャンの開始 (= paho 側の接続試行)・実行中の Disconnect の生存を追跡する。
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
		closing:       make(map[*connection]struct{}),
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
		s.emitter.Emit(cmn.EventMQTTConnectionFailed, domain.ConnectionErrorEvent{ConnectionID: connID, Error: err.Error()})
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
	s.emitter.Emit(cmn.EventMQTTConnected, domain.ConnectionEvent{ConnectionID: connID})
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
	s.emitter.Emit(cmn.EventMQTTConnectionLost, domain.ConnectionErrorEvent{ConnectionID: connID, Error: err.Error()})
}

// Disconnect は指定した接続を切断する。実行中の client 操作があれば、その完了を待ってから切断する。
// 実行中に Shutdown が始まった場合は、切断はするが mqtt:disconnected は出さない。
func (s *MQTTService) Disconnect(connectionID string) error {
	conn, scanClient, err := s.detach(connectionID)
	if err != nil {
		return err
	}
	// opMu・stateMu を放してから mu を取る (ロック順序 mu → opMu → stateMu を守る)。
	defer s.finishClosing(conn)
	// 進行中の Connect を打ち切る。接続 goroutine の完了は待たない。
	conn.cancel()
	disconnectScan(scanClient)
	s.closeDetached(connectionID, conn)
	return nil
}

// closeDetached は detach 済みの接続の client を切断し、終端状態にして mqtt:disconnected を出す。
// Shutdown が先に終端状態にしていたら、イベントは出さない。
func (s *MQTTService) closeDetached(connectionID string, conn *connection) {
	conn.opMu.Lock()
	defer conn.opMu.Unlock()
	conn.client.Disconnect(disconnectQuiesce)

	conn.stateMu.Lock()
	defer conn.stateMu.Unlock()
	if conn.state == stateClosed {
		s.logger.Info("MQTT disconnected during shutdown", "source", "mqtt", "connection_id", connectionID)
		return
	}
	conn.state = stateClosed
	s.logger.Info("MQTT disconnected", "source", "mqtt", "connection_id", connectionID)
	s.emitter.Emit(cmn.EventMQTTDisconnected, domain.ConnectionEvent{ConnectionID: connectionID})
}

// finishClosing は Disconnect の完了を記録する (detach の計上と対になる)。
func (s *MQTTService) finishClosing(conn *connection) {
	s.mu.Lock()
	delete(s.closing, conn)
	s.mu.Unlock()
	s.connWg.Done()
}

// detach は接続を map から外し、同じ区間で stateDisconnecting へ遷移させる。
// 遷移を opMu の取得より前に行うことで、opMu 待ちの操作は状態を見て即座に弾かれ、
// Disconnect が待つのは実行中の 1 操作だけになる。
// スキャンも同じ区間で外し、切断が要るスキャン用クライアントを返す (無ければ nil)。
// 外した接続は closing に入れて connWg に計上する (呼び出し元が finishClosing する)。
// Connect と同じく mu の保持中に計上するので、Shutdown が Wait した後に計上する窓はできない
// (closed の後は conns が空なので、ここに来ない)。
func (s *MQTTService) detach(id string) (*connection, domain.BrokerClient, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	conn, ok := s.conns[id]
	if !ok {
		return nil, nil, connNotFound(id)
	}
	conn.stateMu.Lock()
	defer conn.stateMu.Unlock()
	// 接続失敗経路が stateClosed にしてから map から外すまでの隙間。
	if conn.terminal() {
		return nil, nil, connNotFound(id)
	}
	conn.state = stateDisconnecting
	scanClient := conn.detachScan()
	delete(s.conns, id)
	s.closing[conn] = struct{}{}
	s.connWg.Add(1)
	return conn, scanClient, nil
}

// connNotFound は接続が見つからない (切断が確定済みを含む) ことを表すエラーを返す。
func connNotFound(id string) error {
	return &cmn.NotFoundError{Resource: cmn.ResourceConnection, ID: id}
}

// lookup は接続を引く。無ければ connNotFound のエラーを返す。mu は引く間だけ RLock で取る。
// mu を保持したまま続きの処理をする場合 (detach・reserveScan) は使わない。
func (s *MQTTService) lookup(id string) (*connection, error) {
	s.mu.RLock()
	conn, ok := s.conns[id]
	s.mu.RUnlock()
	if !ok {
		return nil, connNotFound(id)
	}
	return conn, nil
}

// withConn は接続を引いて opMu を取り、切断が確定していないことを確認してから fn を呼ぶ。
// fn の実行中は同じ接続の Disconnect が client を切断しない。
func (s *MQTTService) withConn(id string, fn func(conn *connection) error) error {
	conn, err := s.lookup(id)
	if err != nil {
		return err
	}
	conn.opMu.Lock()
	defer conn.opMu.Unlock()
	// ロック取得を待つ間に切断が確定した接続は操作しない。
	conn.stateMu.RLock()
	closed := conn.terminal()
	conn.stateMu.RUnlock()
	if closed {
		return connNotFound(id)
	}
	return fn(conn)
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
// 切断し、接続 goroutine と実行中の Disconnect の完了を合計 timeout まで待つ。以後の Connect は拒否する。
// true は全接続 (実行中の Disconnect が切断中の接続を含む) を切断し、全接続 goroutine が
// 復帰したこと (paho 側の接続試行も終了済み)、false は上限内に排水できなかったことを表す。
// false でもイベント発行と状態変更は止まっており、残った接続試行は BrokerClient.Connect の契約により
// 接続を確立せずに終わる。
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
	// 実行中の Disconnect の接続は stateClosed にして mqtt:disconnected を止める。
	// client の切断はその Disconnect が行うので、ここでは connWg で完了を待つだけにする。
	for conn := range s.closing {
		conn.stateMu.Lock()
		conn.state = stateClosed
		conn.stateMu.Unlock()
	}
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
				conn.client.Disconnect(disconnectQuiesce)
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
		// 実行中の Disconnect も client の切断を終えるまで復帰しない。
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
