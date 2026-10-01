//go:build integration

package integration

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/hooks/auth"
	"github.com/mochi-mqtt/server/v2/listeners"
	"github.com/mochi-mqtt/server/v2/packets"

	"github.com/f0reth/Wirexa/internal/adapters"
	mqttapp "github.com/f0reth/Wirexa/internal/application/mqtt"
	cmndomain "github.com/f0reth/Wirexa/internal/domain"
	mqttdomain "github.com/f0reth/Wirexa/internal/domain/mqtt"
	mqttinfra "github.com/f0reth/Wirexa/internal/infrastructure/mqtt"
	"github.com/f0reth/Wirexa/internal/testutil"
)

// brokerAddr はテスト全体で使う embedded MQTT ブローカーのアドレス (tcp://127.0.0.1:PORT)。
var brokerAddr string

// mqttShutdownTimeout は終了時に切断と接続試行の終了を待つ上限 (app.go と同じ値)。
const mqttShutdownTimeout = 3 * time.Second

// TestMain はテスト実行前に embedded MQTT ブローカーを起動し、終了後に停止する。
func TestMain(m *testing.M) {
	server := mqtt.New(&mqtt.Options{})
	if err := server.AddHook(new(auth.AllowHook), nil); err != nil {
		fmt.Fprintf(os.Stderr, "TestMain AddHook: %v\n", err)
		os.Exit(1)
	}

	tcp := listeners.NewTCP(listeners.Config{ID: "t1", Address: ":0"})
	if err := server.AddListener(tcp); err != nil {
		fmt.Fprintf(os.Stderr, "TestMain AddListener: %v\n", err)
		os.Exit(1)
	}

	go func() { _ = server.Serve() }()

	// AddListener 内で Init() が呼ばれポートが確定するため、ここで取得可能。
	_, portStr, err := net.SplitHostPort(tcp.Address())
	if err != nil {
		fmt.Fprintf(os.Stderr, "TestMain SplitHostPort: %v\n", err)
		os.Exit(1)
	}
	brokerAddr = "tcp://127.0.0.1:" + portStr

	code := m.Run()
	_ = server.Close()
	os.Exit(code)
}

// mqttMockEmitter はテスト用の Emitter 実装。受信 MQTT メッセージとイベントをチャンネルで収集し、
// 全イベントを名前とデータの組で順に記録する。
type mqttMockEmitter struct {
	ch     chan mqttdomain.MQTTMessage
	failCh chan string // mqtt:connection-failed イベントの connectionId を受信する
	events []mqttEvent
	total  atomic.Int64
	mu     sync.Mutex
}

// mqttEvent は発行されたイベントの名前とデータ。
type mqttEvent struct {
	data any
	name string
}

// mqttMessageBuffer は受信メッセージのバッファ。溢れた分は黙って捨てるので、
// 並行 publish のテストでも溢れない大きさにする。
const mqttMessageBuffer = 256

func newMQTTMockEmitter() *mqttMockEmitter {
	return &mqttMockEmitter{
		ch:     make(chan mqttdomain.MQTTMessage, mqttMessageBuffer),
		failCh: make(chan string, 16),
	}
}

// eventConnID はライフサイクルイベント (connected など) と mqtt:scan-topic のデータから
// connectionId を取り出す。
func eventConnID(data any) string {
	switch d := data.(type) {
	case map[string]any:
		if id, ok := d["connectionId"].(string); ok {
			return id
		}
	case mqttdomain.ScannedTopic:
		return d.ConnectionID
	}
	return ""
}

// scanTopicCount は connID の mqtt:scan-topic のうち、topic のものの発行回数を返す。
func (e *mqttMockEmitter) scanTopicCount(connID, topic string) int {
	e.mu.Lock()
	defer e.mu.Unlock()
	n := 0
	for _, ev := range e.events {
		if scanned, ok := ev.data.(mqttdomain.ScannedTopic); ok && ev.name == cmndomain.EventMQTTScanTopic &&
			scanned.ConnectionID == connID && scanned.Topic == topic {
			n++
		}
	}
	return n
}

// waitScanTopic は connID のスキャンが topic を見つけるまで待つ。
func (e *mqttMockEmitter) waitScanTopic(t *testing.T, connID, topic string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if e.scanTopicCount(connID, topic) > 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for mqtt:scan-topic %s of %s", topic, connID)
}

// lifecycle は connID のライフサイクルイベント (mqtt:message 以外) の名前を発行順に返す。
func (e *mqttMockEmitter) lifecycle(connID string) []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	var names []string
	for _, ev := range e.events {
		if ev.name != cmndomain.EventMQTTMessage && eventConnID(ev.data) == connID {
			names = append(names, ev.name)
		}
	}
	return names
}

// countEvent は connID の name イベントの発行回数を返す。
func (e *mqttMockEmitter) countEvent(name, connID string) int {
	n := 0
	for _, ev := range e.lifecycle(connID) {
		if ev == name {
			n++
		}
	}
	return n
}

// waitEvent は connID の name イベントが n 回以上発行されるまで待つ。
func (e *mqttMockEmitter) waitEvent(t *testing.T, name, connID string, n int, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if e.countEvent(name, connID) >= n {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for %s #%d of %s (events: %v)", name, n, connID, e.lifecycle(connID))
}

func (e *mqttMockEmitter) Emit(event string, data any) {
	e.total.Add(1)
	e.mu.Lock()
	e.events = append(e.events, mqttEvent{name: event, data: data})
	e.mu.Unlock()
	if msg, ok := data.(mqttdomain.MQTTMessage); ok {
		select {
		case e.ch <- msg:
		default:
		}
	}
	if event == cmndomain.EventMQTTConnectionFailed {
		if m, ok := data.(map[string]any); ok {
			if id, ok := m["connectionId"].(string); ok {
				select {
				case e.failCh <- id:
				default:
				}
			}
		}
	}
}

// receiveMessage はタイムアウト付きでメッセージを待機する。
func (e *mqttMockEmitter) receiveMessage(t *testing.T, timeout time.Duration) mqttdomain.MQTTMessage {
	t.Helper()
	select {
	case msg := <-e.ch:
		return msg
	case <-time.After(timeout):
		t.Fatal("timeout waiting for MQTT message")
		return mqttdomain.MQTTMessage{}
	}
}

// noMessage はタイムアウト内にメッセージが届かないことを確認する。
func (e *mqttMockEmitter) noMessage(t *testing.T, wait time.Duration) {
	t.Helper()
	select {
	case msg := <-e.ch:
		t.Fatalf("unexpected MQTT message received: topic=%s payload=%s", msg.Topic, msg.Payload)
	case <-time.After(wait):
	}
}

// assertSilent は wait の間に emitter へイベントが 1 件も届かないことを確認する。
func (e *mqttMockEmitter) assertSilent(t *testing.T, wait time.Duration) {
	t.Helper()
	before := e.total.Load()
	time.Sleep(wait)
	if after := e.total.Load(); after != before {
		t.Errorf("events emitted after Shutdown: %d -> %d", before, after)
	}
}

// waitConnectionFailed はタイムアウト付きで接続失敗イベントを待機し connectionId を返す。
func (e *mqttMockEmitter) waitConnectionFailed(t *testing.T, timeout time.Duration) string {
	t.Helper()
	select {
	case id := <-e.failCh:
		return id
	case <-time.After(timeout):
		t.Fatal("timeout waiting for connection-failed event")
		return ""
	}
}

// newMQTTHandlerWithDir は指定ディレクトリから MQTTHandler を組み立てる（永続化テスト用）。
// 終了処理は RPC 面に無いため、サービスも返す。
func newMQTTHandlerWithDir(t *testing.T, emitter cmndomain.Emitter, dir string) (*adapters.MQTTHandler, *mqttapp.MQTTService) {
	t.Helper()
	repo, err := mqttinfra.NewProfileRepository(dir, nil)
	if err != nil {
		t.Fatalf("NewProfileRepository: %v", err)
	}
	profileSvc, err := mqttapp.NewProfileService(repo)
	if err != nil {
		t.Fatalf("NewProfileService: %v", err)
	}
	mqttSvc := mqttapp.NewMQTTService(context.Background(), emitter, mqttinfra.NewPahoClientFactory(mqttinfra.MQTTClientConfig{}), testutil.NoopLogger{})
	h := &adapters.MQTTHandler{}
	adapters.SetupMQTTHandler(h, mqttSvc, profileSvc)
	return h, mqttSvc
}

// newMQTTHandlerWithConfig は指定クライアント設定で MQTTHandler を組み立てる（タイムアウトテスト用）。
func newMQTTHandlerWithConfig(t *testing.T, emitter cmndomain.Emitter, cfg mqttinfra.MQTTClientConfig) (*adapters.MQTTHandler, *mqttapp.MQTTService) {
	t.Helper()
	repo, err := mqttinfra.NewProfileRepository(t.TempDir(), nil)
	if err != nil {
		t.Fatalf("NewProfileRepository: %v", err)
	}
	profileSvc, err := mqttapp.NewProfileService(repo)
	if err != nil {
		t.Fatalf("NewProfileService: %v", err)
	}
	mqttSvc := mqttapp.NewMQTTService(context.Background(), emitter, mqttinfra.NewPahoClientFactory(cfg), testutil.NoopLogger{})
	h := &adapters.MQTTHandler{}
	adapters.SetupMQTTHandler(h, mqttSvc, profileSvc)
	return h, mqttSvc
}

// newMQTTHandler は統合テスト用に MQTTHandler を DI で組み立てる。
func newMQTTHandler(t *testing.T, emitter cmndomain.Emitter) (*adapters.MQTTHandler, *mqttapp.MQTTService) {
	t.Helper()
	return newMQTTHandlerWithDir(t, emitter, t.TempDir())
}

// connectBroker は brokerAddr へ接続して connectionID を返すヘルパー。
func connectBroker(t *testing.T, h *adapters.MQTTHandler, name string) string {
	t.Helper()
	id, err := h.Connect(mqttdomain.ConnectionConfig{
		Name:   name,
		Broker: brokerAddr,
	})
	if err != nil {
		t.Fatalf("Connect(%q): %v", name, err)
	}
	if id == "" {
		t.Fatal("Connect returned empty connectionID")
	}
	return id
}

// waitConnected は connectionID の接続が Connected == true になるまで待機する。
func waitConnected(t *testing.T, h *adapters.MQTTHandler, connID string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conns := h.GetConnections()
		for i := range conns {
			cs := &conns[i]
			if cs.ID == connID && cs.Connected {
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for connection %q to become Connected", connID)
}

// TestMQTT_ConnectDisconnect は Connect/Disconnect が成功し connectionID が返ることを確認する。
func TestMQTT_ConnectDisconnect(t *testing.T) {
	emitter := newMQTTMockEmitter()
	h, _ := newMQTTHandler(t, emitter)

	connID := connectBroker(t, h, "test-conn")
	waitConnected(t, h, connID, 5*time.Second)

	conns := h.GetConnections()
	if len(conns) != 1 {
		t.Fatalf("expected 1 connection, got %d", len(conns))
	}
	if conns[0].ID != connID {
		t.Errorf("connection ID = %q, want %q", conns[0].ID, connID)
	}
	if !conns[0].Connected {
		t.Error("expected connection to be Connected")
	}

	if err := h.Disconnect(connID); err != nil {
		t.Fatalf("Disconnect: %v", err)
	}
	if conns := h.GetConnections(); len(conns) != 0 {
		t.Errorf("expected 0 connections after disconnect, got %d", len(conns))
	}
}

// TestMQTT_ConnectionIDUniqueness は複数接続で ID が衝突しないことを確認する。
func TestMQTT_ConnectionIDUniqueness(t *testing.T) {
	emitter := newMQTTMockEmitter()
	h, svc := newMQTTHandler(t, emitter)

	const n = 3
	ids := make([]string, n)
	for i := range n {
		ids[i] = connectBroker(t, h, fmt.Sprintf("conn-%d", i))
	}
	t.Cleanup(func() { svc.Shutdown(mqttShutdownTimeout) })

	seen := make(map[string]bool, n)
	for _, id := range ids {
		if seen[id] {
			t.Errorf("duplicate connection ID: %q", id)
		}
		seen[id] = true
	}
}

// TestMQTT_GetConnections は接続状態の一覧が正しく返ることを確認する。
func TestMQTT_GetConnections(t *testing.T) {
	emitter := newMQTTMockEmitter()
	h, svc := newMQTTHandler(t, emitter)

	if conns := h.GetConnections(); len(conns) != 0 {
		t.Fatalf("expected 0 connections initially, got %d", len(conns))
	}

	id1 := connectBroker(t, h, "c1")
	id2 := connectBroker(t, h, "c2")
	t.Cleanup(func() { svc.Shutdown(mqttShutdownTimeout) })

	conns := h.GetConnections()
	if len(conns) != 2 {
		t.Fatalf("expected 2 connections, got %d", len(conns))
	}

	idSet := map[string]bool{conns[0].ID: true, conns[1].ID: true}
	if !idSet[id1] || !idSet[id2] {
		t.Errorf("connection IDs %v do not include %q and %q", slices.Collect(maps.Keys(idSet)), id1, id2)
	}
}

// TestMQTT_SubscribePublishQoS0 は QoS 0 で購読後にメッセージが Emitter 経由で届くことを確認する。
func TestMQTT_SubscribePublishQoS0(t *testing.T) {
	emitter := newMQTTMockEmitter()
	h, _ := newMQTTHandler(t, emitter)

	connID := connectBroker(t, h, "sub0")
	t.Cleanup(func() { _ = h.Disconnect(connID) })
	waitConnected(t, h, connID, 5*time.Second)

	if err := h.Subscribe(connID, "test/qos0", 0); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	if err := h.Publish(connID, "test/qos0", "hello-qos0", 0, false); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	msg := emitter.receiveMessage(t, 5*time.Second)
	if msg.ConnectionID != connID {
		t.Errorf("ConnectionID = %q, want %q", msg.ConnectionID, connID)
	}
	if msg.Topic != "test/qos0" {
		t.Errorf("Topic = %q, want test/qos0", msg.Topic)
	}
	if msg.Payload != "hello-qos0" {
		t.Errorf("Payload = %q, want hello-qos0", msg.Payload)
	}
	if msg.QoS != 0 {
		t.Errorf("QoS = %d, want 0", msg.QoS)
	}
}

// TestMQTT_SubscribePublishQoS1 は QoS 1 でメッセージが確実に届くことを確認する。
func TestMQTT_SubscribePublishQoS1(t *testing.T) {
	emitter := newMQTTMockEmitter()
	h, _ := newMQTTHandler(t, emitter)

	connID := connectBroker(t, h, "sub1")
	t.Cleanup(func() { _ = h.Disconnect(connID) })
	waitConnected(t, h, connID, 5*time.Second)

	if err := h.Subscribe(connID, "test/qos1", 1); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	if err := h.Publish(connID, "test/qos1", "hello-qos1", 1, false); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	msg := emitter.receiveMessage(t, 5*time.Second)
	if msg.Payload != "hello-qos1" {
		t.Errorf("Payload = %q, want hello-qos1", msg.Payload)
	}
}

// TestMQTT_Unsubscribe は購読解除後にメッセージが届かないことを確認する。
func TestMQTT_Unsubscribe(t *testing.T) {
	emitter := newMQTTMockEmitter()
	h, _ := newMQTTHandler(t, emitter)

	connID := connectBroker(t, h, "unsub")
	t.Cleanup(func() { _ = h.Disconnect(connID) })
	waitConnected(t, h, connID, 5*time.Second)

	if err := h.Subscribe(connID, "test/unsub", 0); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	// 購読中はメッセージが届く
	if err := h.Publish(connID, "test/unsub", "before-unsub", 0, false); err != nil {
		t.Fatalf("Publish before unsubscribe: %v", err)
	}
	_ = emitter.receiveMessage(t, 5*time.Second)

	// 購読解除
	if err := h.Unsubscribe(connID, "test/unsub"); err != nil {
		t.Fatalf("Unsubscribe: %v", err)
	}

	// 解除後はメッセージが届かない
	if err := h.Publish(connID, "test/unsub", "after-unsub", 0, false); err != nil {
		t.Fatalf("Publish after unsubscribe: %v", err)
	}
	emitter.noMessage(t, 500*time.Millisecond)
}

// TestMQTT_ProfileCRUD はプロファイルの保存・取得・削除が永続化されることを確認する。
func TestMQTT_ProfileCRUD(t *testing.T) {
	emitter := newMQTTMockEmitter()
	h, _ := newMQTTHandler(t, emitter)

	if profiles := h.GetProfiles(); len(profiles) != 0 {
		t.Fatalf("expected 0 profiles initially, got %d", len(profiles))
	}

	// 新規作成は ID を空で渡し、採番後のプロファイルを受け取る。
	created, err := h.SaveProfile(mqttdomain.BrokerProfile{
		Name:   "LocalBroker",
		Broker: brokerAddr,
	})
	if err != nil {
		t.Fatalf("SaveProfile: %v", err)
	}
	if created.ID == "" {
		t.Fatal("expected server-generated ID, got empty")
	}

	profiles := h.GetProfiles()
	if len(profiles) != 1 {
		t.Fatalf("expected 1 profile, got %d", len(profiles))
	}
	if profiles[0].Name != "LocalBroker" {
		t.Errorf("Name = %q, want LocalBroker", profiles[0].Name)
	}

	// 更新
	saved := profiles[0]
	saved.Name = "Updated"
	if _, err := h.SaveProfile(saved); err != nil {
		t.Fatalf("SaveProfile (update): %v", err)
	}
	profiles = h.GetProfiles()
	if len(profiles) != 1 || profiles[0].Name != "Updated" {
		t.Errorf("expected updated profile Name=Updated, got %+v", profiles)
	}

	// 削除
	if err := h.DeleteProfile(saved.ID); err != nil {
		t.Fatalf("DeleteProfile: %v", err)
	}
	if profiles := h.GetProfiles(); len(profiles) != 0 {
		t.Errorf("expected 0 profiles after delete, got %d", len(profiles))
	}

	// 存在しない ID は NotFoundError
	if err := h.DeleteProfile("nonexistent"); err == nil {
		t.Error("expected error for nonexistent profile, got nil")
	}
}

// TestMQTT_SaveProfile_RejectsTraversalID は RPC 由来のトラバーサル ID が
// ハンドラ経由で拒否され、ストア外にファイルが作られないことを確認する。
func TestMQTT_SaveProfile_RejectsTraversalID(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "profiles")
	emitter := newMQTTMockEmitter()
	h, _ := newMQTTHandlerWithDir(t, emitter, dir)

	for _, id := range traversalIDs {
		t.Run(id, func(t *testing.T) {
			if _, err := h.SaveProfile(mqttdomain.BrokerProfile{
				ID:     id,
				Name:   "Attack",
				Broker: brokerAddr,
			}); err == nil {
				t.Fatalf("SaveProfile(%q) = nil, want error", id)
			}
			if err := h.DeleteProfile(id); err == nil {
				t.Errorf("DeleteProfile(%q) = nil, want error", id)
			}
			if profiles := h.GetProfiles(); len(profiles) != 0 {
				t.Errorf("expected 0 profiles, got %d", len(profiles))
			}
		})
	}

	assertNoFilesOutside(t, base, "profiles")
}

// TestMQTT_Shutdown は全接続が切断されることを確認する。
func TestMQTT_Shutdown(t *testing.T) {
	emitter := newMQTTMockEmitter()
	h, svc := newMQTTHandler(t, emitter)

	id1 := connectBroker(t, h, "s1")
	id2 := connectBroker(t, h, "s2")
	waitConnected(t, h, id1, 5*time.Second)
	waitConnected(t, h, id2, 5*time.Second)

	if len(h.GetConnections()) != 2 {
		t.Fatalf("expected 2 connections before shutdown")
	}

	if !svc.Shutdown(mqttShutdownTimeout) {
		t.Error("expected Shutdown to drain within timeout")
	}

	if conns := h.GetConnections(); len(conns) != 0 {
		t.Errorf("expected 0 connections after shutdown, got %d", len(conns))
	}
	emitter.assertSilent(t, 300*time.Millisecond)
}

// blackholeBroker は accept だけして応答しない TCP リスナーを起動し、その URL を返す。
// 閉じたポートでは dial が即座に失敗して「接続中」の窓が短いため、試行中の状態を作るのに使う。
// accepted は最初の接続を受け付けたときに閉じられる。
func blackholeBroker(t *testing.T) (broker string, accepted <-chan struct{}) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	ch := make(chan struct{})
	var mu sync.Mutex
	var conns []net.Conn
	go func() {
		var once sync.Once
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			mu.Lock()
			conns = append(conns, conn)
			mu.Unlock()
			once.Do(func() { close(ch) })
		}
	}()
	t.Cleanup(func() {
		_ = ln.Close()
		mu.Lock()
		defer mu.Unlock()
		for _, c := range conns {
			_ = c.Close()
		}
	})
	return "tcp://" + ln.Addr().String(), ch
}

// TestMQTT_Shutdown_WhileConnecting は接続試行中の Shutdown が上限内に復帰し、以後イベントが届かないことを確認する。
// 進行中の dial は ConnectTimeout まで残り得るため、戻り値は true を要求しない。
func TestMQTT_Shutdown_WhileConnecting(t *testing.T) {
	emitter := newMQTTMockEmitter()
	h, svc := newMQTTHandler(t, emitter)
	broker, accepted := blackholeBroker(t)

	if _, err := h.Connect(mqttdomain.ConnectionConfig{Name: "blackhole", Broker: broker}); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	select {
	case <-accepted:
	case <-time.After(5 * time.Second):
		t.Fatal("broker did not accept the connection")
	}

	start := time.Now()
	svc.Shutdown(mqttShutdownTimeout)
	if elapsed := time.Since(start); elapsed > mqttShutdownTimeout+time.Second {
		t.Errorf("Shutdown took %v, want <= %v", elapsed, mqttShutdownTimeout+time.Second)
	}
	emitter.assertSilent(t, 500*time.Millisecond)
}

// TestMQTT_Shutdown_WhileReceiving は受信中の Shutdown の復帰後に mqtt:message が届かないことを確認する。
func TestMQTT_Shutdown_WhileReceiving(t *testing.T) {
	emitter := newMQTTMockEmitter()
	h, svc := newMQTTHandler(t, emitter)
	subID := connectBroker(t, h, "receiver")
	waitConnected(t, h, subID, 5*time.Second)
	if err := h.Subscribe(subID, "test/shutdown-receiving", 0); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	// 別サービスの接続から連続 publish する (受信側の Shutdown の影響を受けない)。
	pubH, pubSvc := newMQTTHandler(t, newMQTTMockEmitter())
	t.Cleanup(func() { pubSvc.Shutdown(mqttShutdownTimeout) })
	pubID := connectBroker(t, pubH, "publisher")
	waitConnected(t, pubH, pubID, 5*time.Second)

	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Go(func() {
		for {
			select {
			case <-stop:
				return
			default:
			}
			_ = pubH.Publish(pubID, "test/shutdown-receiving", "tick", 0, false)
			time.Sleep(time.Millisecond)
		}
	})
	t.Cleanup(func() {
		close(stop)
		wg.Wait()
	})

	_ = emitter.receiveMessage(t, 5*time.Second)
	if !svc.Shutdown(mqttShutdownTimeout) {
		t.Error("expected Shutdown to drain within timeout")
	}
	emitter.assertSilent(t, 300*time.Millisecond)
}

// TestMQTT_ProfilePersistenceRoundTrip は SaveProfile 後に同一 dir で再作成した Handler でデータが復元されることを確認する。
func TestMQTT_ProfilePersistenceRoundTrip(t *testing.T) {
	dir := t.TempDir()
	emitter := newMQTTMockEmitter()

	// 1 回目: プロファイルを保存
	h1, _ := newMQTTHandlerWithDir(t, emitter, dir)
	profile, err := h1.SaveProfile(mqttdomain.BrokerProfile{
		Name:   "PersistProfile",
		Broker: brokerAddr,
	})
	if err != nil {
		t.Fatalf("SaveProfile: %v", err)
	}

	// 2 回目: 同一 dir から Handler を再作成してデータを確認
	h2, _ := newMQTTHandlerWithDir(t, emitter, dir)
	profiles := h2.GetProfiles()
	if len(profiles) != 1 {
		t.Fatalf("expected 1 profile after reload, got %d", len(profiles))
	}
	if profiles[0].ID != profile.ID {
		t.Errorf("profile ID = %q, want %q", profiles[0].ID, profile.ID)
	}
	if profiles[0].Name != "PersistProfile" {
		t.Errorf("profile Name = %q, want PersistProfile", profiles[0].Name)
	}
}

// TestMQTT_Connect_UnreachableBroker は到達不能ブローカーへの接続失敗後に接続エントリが削除されることを確認する。
func TestMQTT_Connect_UnreachableBroker(t *testing.T) {
	emitter := newMQTTMockEmitter()
	// 短いタイムアウトを設定してテストを高速化する
	h, _ := newMQTTHandlerWithConfig(t, emitter, mqttinfra.MQTTClientConfig{
		ConnectTimeout: 1 * time.Second,
		TokenTimeout:   3 * time.Second,
	})

	// 空きポート番号を取得し（何もリッスンしていない）、接続を試みる
	port := freePort(t)
	unreachableBroker := fmt.Sprintf("tcp://127.0.0.1:%d", port)

	connID, err := h.Connect(mqttdomain.ConnectionConfig{
		Name:   "unreachable",
		Broker: unreachableBroker,
	})
	if err != nil {
		t.Fatalf("Connect returned unexpected synchronous error: %v", err)
	}

	// 接続失敗イベントを待機し connectionId が一致することを確認する
	failedID := emitter.waitConnectionFailed(t, 10*time.Second)
	if failedID != connID {
		t.Errorf("connection-failed event ID = %q, want %q", failedID, connID)
	}

	// 接続エントリが s.conns から削除されていることを確認する
	if conns := h.GetConnections(); len(conns) != 0 {
		t.Errorf("expected 0 connections after failure, got %d", len(conns))
	}
}

// TestMQTT_Connect_EmptyBroker は Broker が空文字列のとき ValidationError が返ることを確認する。
func TestMQTT_Connect_EmptyBroker(t *testing.T) {
	emitter := newMQTTMockEmitter()
	h, _ := newMQTTHandler(t, emitter)

	_, err := h.Connect(mqttdomain.ConnectionConfig{Name: "empty", Broker: ""})
	if err == nil {
		t.Error("expected error for empty broker, got nil")
	}
}

// TestMQTT_Publish_InvalidInput は topic 空文字または qos>2 で ValidationError が返ることを確認する。
func TestMQTT_Publish_InvalidInput(t *testing.T) {
	emitter := newMQTTMockEmitter()
	h, _ := newMQTTHandler(t, emitter)

	connID := connectBroker(t, h, "pub-invalid")
	t.Cleanup(func() { _ = h.Disconnect(connID) })
	waitConnected(t, h, connID, 5*time.Second)

	t.Run("empty_topic", func(t *testing.T) {
		if err := h.Publish(connID, "", "payload", 0, false); err == nil {
			t.Error("expected error for empty topic, got nil")
		}
	})

	t.Run("qos_too_high", func(t *testing.T) {
		if err := h.Publish(connID, "test/invalid", "payload", 3, false); err == nil {
			t.Error("expected error for qos=3, got nil")
		}
	})
}

// TestMQTT_Subscribe_InvalidInput は topic 空文字または qos>2 で ValidationError が返ることを確認する。
func TestMQTT_Subscribe_InvalidInput(t *testing.T) {
	emitter := newMQTTMockEmitter()
	h, _ := newMQTTHandler(t, emitter)

	connID := connectBroker(t, h, "sub-invalid")
	t.Cleanup(func() { _ = h.Disconnect(connID) })
	waitConnected(t, h, connID, 5*time.Second)

	t.Run("empty_topic", func(t *testing.T) {
		if err := h.Subscribe(connID, "", 0); err == nil {
			t.Error("expected error for empty topic, got nil")
		}
	})

	t.Run("qos_too_high", func(t *testing.T) {
		if err := h.Subscribe(connID, "test/invalid", 3); err == nil {
			t.Error("expected error for qos=3, got nil")
		}
	})
}

// TestMQTT_Disconnect_NotFound は存在しない connectionID で Disconnect を呼ぶと error が返ることを確認する。
func TestMQTT_Disconnect_NotFound(t *testing.T) {
	emitter := newMQTTMockEmitter()
	h, _ := newMQTTHandler(t, emitter)

	if err := h.Disconnect("nonexistent-id"); err == nil {
		t.Error("expected error for nonexistent connection, got nil")
	}
}

// TestMQTT_Disconnect_Twice は同じ connectionID に対して 2 回目の Disconnect が error を返すことを確認する。
func TestMQTT_Disconnect_Twice(t *testing.T) {
	emitter := newMQTTMockEmitter()
	h, _ := newMQTTHandler(t, emitter)

	connID := connectBroker(t, h, "twice")
	waitConnected(t, h, connID, 5*time.Second)

	if err := h.Disconnect(connID); err != nil {
		t.Fatalf("first Disconnect: %v", err)
	}

	if err := h.Disconnect(connID); err == nil {
		t.Error("expected error on second Disconnect, got nil")
	}
}

// TestMQTT_Subscribe_Wildcard はワイルドカードトピックでサブトピックのメッセージが届くことを確認する。
func TestMQTT_Subscribe_Wildcard(t *testing.T) {
	emitter := newMQTTMockEmitter()
	h, _ := newMQTTHandler(t, emitter)

	connID := connectBroker(t, h, "wildcard")
	t.Cleanup(func() { _ = h.Disconnect(connID) })
	waitConnected(t, h, connID, 5*time.Second)

	// ワイルドカードトピックを購読
	if err := h.Subscribe(connID, "test/wild/#", 0); err != nil {
		t.Fatalf("Subscribe wildcard: %v", err)
	}

	// サブトピックに Publish
	if err := h.Publish(connID, "test/wild/specific", "wildcard-msg", 0, false); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	msg := emitter.receiveMessage(t, 5*time.Second)
	if msg.Topic != "test/wild/specific" {
		t.Errorf("Topic = %q, want test/wild/specific", msg.Topic)
	}
	if msg.Payload != "wildcard-msg" {
		t.Errorf("Payload = %q, want wildcard-msg", msg.Payload)
	}
}

// TestMQTT_SubscribePublishQoS2 は QoS 2 でメッセージが確実に届くことを確認する。
func TestMQTT_SubscribePublishQoS2(t *testing.T) {
	emitter := newMQTTMockEmitter()
	h, _ := newMQTTHandler(t, emitter)

	connID := connectBroker(t, h, "qos2")
	t.Cleanup(func() { _ = h.Disconnect(connID) })
	waitConnected(t, h, connID, 5*time.Second)

	if err := h.Subscribe(connID, "test/qos2", 2); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	if err := h.Publish(connID, "test/qos2", "hello-qos2", 2, false); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	msg := emitter.receiveMessage(t, 5*time.Second)
	if msg.Payload != "hello-qos2" {
		t.Errorf("Payload = %q, want hello-qos2", msg.Payload)
	}
}

// TestMQTT_Unsubscribe_EmptyTopic は topic が空文字列のとき ValidationError が返ることを確認する。
func TestMQTT_Unsubscribe_EmptyTopic(t *testing.T) {
	emitter := newMQTTMockEmitter()
	h, _ := newMQTTHandler(t, emitter)

	connID := connectBroker(t, h, "unsub-empty")
	t.Cleanup(func() { _ = h.Disconnect(connID) })
	waitConnected(t, h, connID, 5*time.Second)

	if err := h.Unsubscribe(connID, ""); err == nil {
		t.Error("expected error for empty topic, got nil")
	}
}

// TestMQTT_Connect_Concurrent は複数 goroutine から並行して Connect / Disconnect を呼んでも、
// 全接続が一意な ID で確立し、全て切断できることを確認する。
func TestMQTT_Connect_Concurrent(t *testing.T) {
	emitter := newMQTTMockEmitter()
	h, svc := newMQTTHandler(t, emitter)
	t.Cleanup(func() { svc.Shutdown(mqttShutdownTimeout) })

	const n = 5
	var wg sync.WaitGroup
	ids := make(chan string, n)
	errs := make(chan error, n)

	for range n {
		wg.Go(func() {
			id, err := h.Connect(mqttdomain.ConnectionConfig{
				Name:   "concurrent",
				Broker: brokerAddr,
			})
			if err != nil {
				errs <- err
				return
			}
			ids <- id
		})
	}
	wg.Wait()
	close(ids)
	close(errs)
	for err := range errs {
		t.Errorf("Connect: %v", err)
	}

	seen := map[string]bool{}
	for id := range ids {
		if seen[id] {
			t.Errorf("duplicate connection ID %q", id)
		}
		seen[id] = true
		waitConnected(t, h, id, 5*time.Second)
	}
	if len(seen) != n {
		t.Fatalf("connections = %d, want %d", len(seen), n)
	}

	for id := range seen {
		wg.Go(func() {
			if err := h.Disconnect(id); err != nil {
				t.Errorf("Disconnect(%s): %v", id, err)
			}
		})
	}
	wg.Wait()
	if conns := h.GetConnections(); len(conns) != 0 {
		t.Errorf("connections after Disconnect = %d, want 0", len(conns))
	}
}

// testBroker はテスト専用の埋め込みブローカー。
type testBroker struct {
	*mqtt.Server
	addr      string // 実アドレス (host:port)
	closeOnce sync.Once
}

// Close はブローカーを止める。mochi の Close は 2 回目で panic するので 1 回に限る。
func (b *testBroker) Close() {
	b.closeOnce.Do(func() { _ = b.Server.Close() })
}

// startMQTTBroker は専用の埋め込みブローカーを addr で起動する。
// addr に "127.0.0.1:0" を渡すと空きポートを使う。hook が nil なら全てを許可する。
// 共有ブローカーを止められないテスト (接続断) や、ACL を変えるテストで使う。
func startMQTTBroker(t *testing.T, addr string, hook mqtt.Hook) *testBroker {
	t.Helper()
	server := mqtt.New(&mqtt.Options{InlineClient: true})
	if hook == nil {
		hook = new(auth.AllowHook)
	}
	if err := server.AddHook(hook, nil); err != nil {
		t.Fatalf("AddHook: %v", err)
	}
	tcp := listeners.NewTCP(listeners.Config{ID: "dedicated", Address: addr})
	if err := server.AddListener(tcp); err != nil {
		t.Fatalf("AddListener(%s): %v", addr, err)
	}
	go func() { _ = server.Serve() }()
	b := &testBroker{Server: server, addr: tcp.Address()}
	t.Cleanup(b.Close)
	return b
}

// denyFilterHook は接続を全て許可し、filter への購読と publish だけを ACL で拒否する。
type denyFilterHook struct {
	mqtt.HookBase
	filter string
}

func (h *denyFilterHook) ID() string { return "deny-filter" }

func (h *denyFilterHook) Provides(b byte) bool {
	return b == mqtt.OnConnectAuthenticate || b == mqtt.OnACLCheck
}

func (h *denyFilterHook) OnConnectAuthenticate(*mqtt.Client, packets.Packet) bool { return true }

func (h *denyFilterHook) OnACLCheck(_ *mqtt.Client, topic string, _ bool) bool {
	return topic != h.filter
}

// TestMQTT_InvalidWildcardTopics は、位置違反のワイルドカードの購読と、ワイルドカードを含む
// トピックへの publish が送信前に ValidationError になり、購読一覧に残らず、接続も切れないこと、
// ブローカーが SUBACK で拒否した購読がエラーになることを確認する。
func TestMQTT_InvalidWildcardTopics(t *testing.T) {
	emitter := newMQTTMockEmitter()
	h, svc := newMQTTHandlerWithConfig(t, emitter, mqttinfra.MQTTClientConfig{TokenTimeout: 2 * time.Second})
	t.Cleanup(func() { svc.Shutdown(mqttShutdownTimeout) })
	id := connectBroker(t, h, "wildcards")
	waitConnected(t, h, id, 5*time.Second)

	for _, filter := range []string{"a/#/b", "a/b#", "a+/b"} {
		var ve *cmndomain.ValidationError
		if err := h.Subscribe(id, filter, 0); !errors.As(err, &ve) {
			t.Errorf("Subscribe(%q): want ValidationError, got %v", filter, err)
		}
	}
	if subs := h.GetConnections()[0].Subscriptions; len(subs) != 0 {
		t.Errorf("subscriptions = %v, want none", subs)
	}

	// ブローカーはワイルドカードを含む PUBLISH をプロトコル違反として接続ごと切るので、送る前に拒否する。
	start := time.Now()
	var ve *cmndomain.ValidationError
	if err := h.Publish(id, "a/+", "x", 1, false); !errors.As(err, &ve) {
		t.Errorf("Publish(a/+): want ValidationError, got %v", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("Publish(a/+) took %v, want it rejected before sending", elapsed)
	}
	if err := h.Subscribe(id, "wildcards/ok", 1); err != nil {
		t.Fatalf("Subscribe after the rejected publish: %v", err)
	}
	if err := h.Publish(id, "wildcards/ok", "still connected", 1, false); err != nil {
		t.Fatalf("Publish after the rejected publish: %v", err)
	}
	if msg := emitter.receiveMessage(t, 5*time.Second); msg.Payload != "still connected" {
		t.Errorf("payload = %q, want still connected", msg.Payload)
	}

	// 形式は正しいがブローカーが拒否する購読 (ここでは ACL) は SUBACK の結果でエラーにする。
	acl := startMQTTBroker(t, "127.0.0.1:0", &denyFilterHook{filter: "denied/topic"})
	denied, err := h.Connect(mqttdomain.ConnectionConfig{Name: "acl", Broker: "tcp://" + acl.addr})
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	waitConnected(t, h, denied, 5*time.Second)
	if err := h.Subscribe(denied, "denied/topic", 0); !errors.Is(err, mqttdomain.ErrSubscriptionRejected) {
		t.Errorf("Subscribe(denied/topic): want ErrSubscriptionRejected, got %v", err)
	}
	for _, c := range h.GetConnections() {
		if c.ID == denied && len(c.Subscriptions) != 0 {
			t.Errorf("subscriptions = %v, want the rejected one not listed", c.Subscriptions)
		}
	}
}

// connectionStatus は connID の ConnectionStatus を返す。無ければ ok = false。
func connectionStatus(h *adapters.MQTTHandler, connID string) (mqttdomain.ConnectionStatus, bool) {
	conns := h.GetConnections()
	for i := range conns {
		if conns[i].ID == connID {
			return conns[i], true
		}
	}
	return mqttdomain.ConnectionStatus{}, false
}

// TestMQTT_ConnectionLostAndReconnect は、ブローカーとの接続が切れると mqtt:connection-lost が出て
// Connected が false になり、同じアドレスでブローカーが戻ると自動再接続して mqtt:connected が
// 再び出て、購読が張り直されてメッセージが届くことを確認する。
// 共有ブローカーは止められないので、専用のブローカーを同じポートで起動し直す。
func TestMQTT_ConnectionLostAndReconnect(t *testing.T) {
	broker := startMQTTBroker(t, "127.0.0.1:0", nil)
	emitter := newMQTTMockEmitter()
	h, svc := newMQTTHandler(t, emitter)
	t.Cleanup(func() { svc.Shutdown(mqttShutdownTimeout) })

	id, err := h.Connect(mqttdomain.ConnectionConfig{Name: "reconnect", Broker: "tcp://" + broker.addr})
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	emitter.waitEvent(t, cmndomain.EventMQTTConnected, id, 1, 5*time.Second)
	const topic = "reconnect/topic"
	if err := h.Subscribe(id, topic, 1); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	broker.Close()
	emitter.waitEvent(t, cmndomain.EventMQTTConnectionLost, id, 1, 5*time.Second)
	status, ok := connectionStatus(h, id)
	if !ok {
		t.Fatal("the connection should be kept for auto-reconnect")
	}
	if status.Connected {
		t.Error("Connected = true while the broker is down")
	}
	if len(status.Subscriptions) != 1 || status.Subscriptions[0].Topic != topic {
		t.Errorf("subscriptions = %v, want [%s] kept", status.Subscriptions, topic)
	}

	restarted := startMQTTBroker(t, broker.addr, nil)
	emitter.waitEvent(t, cmndomain.EventMQTTConnected, id, 2, 15*time.Second)
	waitConnected(t, h, id, 5*time.Second)

	// 張り直しは再接続の通知の後に非同期で行うので、ブローカー側の購読数で完了を待つ。
	deadline := time.Now().Add(5 * time.Second)
	for atomic.LoadInt64(&restarted.Info.Subscriptions) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("the subscription was not restored on the restarted broker")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := restarted.Publish(topic, []byte("after reconnect"), false, 1); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if msg := emitter.receiveMessage(t, 5*time.Second); msg.Topic != topic || msg.Payload != "after reconnect" {
		t.Errorf("message = %+v, want the one published after reconnect", msg)
	}
	if got := emitter.lifecycle(id); !slices.Equal(got, []string{
		cmndomain.EventMQTTConnected, cmndomain.EventMQTTConnectionLost, cmndomain.EventMQTTConnected,
	}) {
		t.Errorf("lifecycle events = %v", got)
	}
}

// waitBrokerSubscriptions はブローカー側の購読数が n 以上になるまで待つ。
// mqtt:connected も GetConnections の購読も SUBACK の完了を意味しないので、受信の検証の前に使う。
func waitBrokerSubscriptions(t *testing.T, b *testBroker, n int64) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for atomic.LoadInt64(&b.Info.Subscriptions) < n {
		if time.Now().After(deadline) {
			t.Fatalf("broker subscriptions = %d, want >= %d", atomic.LoadInt64(&b.Info.Subscriptions), n)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestMQTT_SubscribeBeforeConnected は、Connect の直後 (接続の確立を待たずに) 購読しても成功し、
// 接続の確立時に購読されてメッセージが届くことを確認する。
func TestMQTT_SubscribeBeforeConnected(t *testing.T) {
	broker := startMQTTBroker(t, "127.0.0.1:0", nil)
	emitter := newMQTTMockEmitter()
	h, svc := newMQTTHandler(t, emitter)
	t.Cleanup(func() { svc.Shutdown(mqttShutdownTimeout) })

	id, err := h.Connect(mqttdomain.ConnectionConfig{Name: "early", Broker: "tcp://" + broker.addr})
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	const topic = "early/topic"
	if err := h.Subscribe(id, topic, 1); err != nil {
		t.Fatalf("Subscribe before connected: %v", err)
	}

	waitConnected(t, h, id, 5*time.Second)
	waitBrokerSubscriptions(t, broker, 1)
	if err := broker.Publish(topic, []byte("early"), false, 1); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if msg := emitter.receiveMessage(t, 5*time.Second); msg.ConnectionID != id || msg.Payload != "early" {
		t.Errorf("message = %+v, want the one published on %s", msg, id)
	}
}

// TestMQTT_ReconnectAfterDisconnect_Resubscribes は、フロントエンドの手動再接続と同じ順に
// Disconnect → 新しい Connect → 直後に同じトピックを Subscribe すると、新しい接続でメッセージが届くことを確認する。
func TestMQTT_ReconnectAfterDisconnect_Resubscribes(t *testing.T) {
	broker := startMQTTBroker(t, "127.0.0.1:0", nil)
	emitter := newMQTTMockEmitter()
	h, svc := newMQTTHandler(t, emitter)
	t.Cleanup(func() { svc.Shutdown(mqttShutdownTimeout) })
	config := mqttdomain.ConnectionConfig{Name: "manual-reconnect", Broker: "tcp://" + broker.addr}
	const topic = "manual/reconnect"

	oldID, err := h.Connect(config)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	waitConnected(t, h, oldID, 5*time.Second)
	if err = h.Subscribe(oldID, topic, 1); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	waitBrokerSubscriptions(t, broker, 1)
	if err = h.Disconnect(oldID); err != nil {
		t.Fatalf("Disconnect: %v", err)
	}
	// 旧接続の購読が消えたのを見てから張り直す (残っていると購読数で新しい購読の成立を判定できない)。
	deadline := time.Now().Add(5 * time.Second)
	for atomic.LoadInt64(&broker.Info.Subscriptions) != 0 {
		if time.Now().After(deadline) {
			t.Fatal("the old connection's subscription was not removed from the broker")
		}
		time.Sleep(10 * time.Millisecond)
	}

	newID, err := h.Connect(config)
	if err != nil {
		t.Fatalf("Connect again: %v", err)
	}
	if err := h.Subscribe(newID, topic, 1); err != nil {
		t.Fatalf("Subscribe right after reconnect: %v", err)
	}

	waitConnected(t, h, newID, 5*time.Second)
	waitBrokerSubscriptions(t, broker, 1)
	if err := broker.Publish(topic, []byte("after manual reconnect"), false, 1); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if msg := emitter.receiveMessage(t, 5*time.Second); msg.ConnectionID != newID || msg.Payload != "after manual reconnect" {
		t.Errorf("message = %+v, want the one published on %s", msg, newID)
	}
}

// TestMQTT_ConcurrentPublish は同じ接続から並行に QoS 1 で publish しても、全件がブローカーへ届き
// 購読側で受信できることを確認する (client 操作は opMu で直列化される)。
func TestMQTT_ConcurrentPublish(t *testing.T) {
	emitter := newMQTTMockEmitter()
	h, svc := newMQTTHandler(t, emitter)
	t.Cleanup(func() { svc.Shutdown(mqttShutdownTimeout) })
	subID := connectBroker(t, h, "sub")
	pubID := connectBroker(t, h, "pub")
	waitConnected(t, h, subID, 5*time.Second)
	waitConnected(t, h, pubID, 5*time.Second)
	const topic = "concurrent/publish"
	if err := h.Subscribe(subID, topic, 1); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	const n = 50
	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() {
			if err := h.Publish(pubID, topic, strconv.Itoa(i), 1, false); err != nil {
				t.Errorf("Publish(%d): %v", i, err)
			}
		})
	}
	wg.Wait()

	got := map[string]bool{}
	for range n {
		msg := emitter.receiveMessage(t, 10*time.Second)
		got[msg.Payload] = true
	}
	for i := range n {
		if !got[strconv.Itoa(i)] {
			t.Errorf("message %d was not received", i)
		}
	}
}

// TestMQTT_Connect_InvalidBrokerURL は、未対応のスキームと解析できない URL への接続が
// mqtt:connected を出さずに失敗して接続一覧から消えることと、スキームを省いたアドレスは
// tcp:// として接続できることを確認する。
func TestMQTT_Connect_InvalidBrokerURL(t *testing.T) {
	emitter := newMQTTMockEmitter()
	h, svc := newMQTTHandlerWithConfig(t, emitter, mqttinfra.MQTTClientConfig{
		ConnectTimeout: time.Second,
		TokenTimeout:   3 * time.Second,
	})
	t.Cleanup(func() { svc.Shutdown(mqttShutdownTimeout) })

	for _, broker := range []string{
		fmt.Sprintf("http://127.0.0.1:%d", freePort(t)), // 解析はできるが接続時に unknown protocol
		"tcp://[::1", // 解析できず、接続先が 1 つも無い
	} {
		t.Run(broker, func(t *testing.T) {
			id, err := h.Connect(mqttdomain.ConnectionConfig{Name: "invalid", Broker: broker})
			if err != nil {
				return // 同期エラーで拒否するのでもよい
			}
			emitter.waitEvent(t, cmndomain.EventMQTTConnectionFailed, id, 1, 10*time.Second)
			if n := emitter.countEvent(cmndomain.EventMQTTConnected, id); n != 0 {
				t.Errorf("mqtt:connected emitted %d times for an invalid URL", n)
			}
			if _, ok := connectionStatus(h, id); ok {
				t.Error("the failed connection should be removed")
			}
		})
	}

	id, err := h.Connect(mqttdomain.ConnectionConfig{Name: "no-scheme", Broker: strings.TrimPrefix(brokerAddr, "tcp://")})
	if err != nil {
		t.Fatalf("Connect without a scheme: %v", err)
	}
	waitConnected(t, h, id, 5*time.Second)
}

// TestMQTT_OperationsAfterDisconnectAndShutdown は、切断した接続への操作が NotFoundError になり、
// Shutdown 後の Connect が拒否されてイベントも出ないことを確認する。
func TestMQTT_OperationsAfterDisconnectAndShutdown(t *testing.T) {
	emitter := newMQTTMockEmitter()
	h, svc := newMQTTHandler(t, emitter)
	id := connectBroker(t, h, "after")
	waitConnected(t, h, id, 5*time.Second)
	if err := h.Subscribe(id, "after/disconnect", 0); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	if err := h.Disconnect(id); err != nil {
		t.Fatalf("Disconnect: %v", err)
	}

	ops := map[string]func() error{
		"Publish":     func() error { return h.Publish(id, "after/disconnect", "x", 0, false) },
		"Subscribe":   func() error { return h.Subscribe(id, "after/disconnect", 0) },
		"Unsubscribe": func() error { return h.Unsubscribe(id, "after/disconnect") },
	}
	for name, op := range ops {
		var nf *cmndomain.NotFoundError
		if err := op(); !errors.As(err, &nf) {
			t.Errorf("%s after Disconnect: want NotFoundError, got %v", name, err)
		}
	}

	if !svc.Shutdown(mqttShutdownTimeout) {
		t.Error("expected Shutdown to drain within timeout")
	}
	if _, err := h.Connect(mqttdomain.ConnectionConfig{Name: "late", Broker: brokerAddr}); err == nil {
		t.Error("Connect after Shutdown should be rejected")
	}
	emitter.assertSilent(t, 300*time.Millisecond)
	if conns := h.GetConnections(); len(conns) != 0 {
		t.Errorf("connections after Shutdown = %d, want 0", len(conns))
	}
}

// TestMQTT_OverlappingSubscriptions は、同じ接続で重なる購読 (overlap/# と overlap/specific) が
// あっても、受信した 1 件のメッセージで mqtt:message を 1 回だけ発行することを確認する。
// Broker Topics のスキャンは # を購読するので、スキャン中は全ての購読がこれと重なる。
// 一致した購読の数だけ発行すると、同じメッセージが重複して表示される。
func TestMQTT_OverlappingSubscriptions(t *testing.T) {
	emitter := newMQTTMockEmitter()
	h, svc := newMQTTHandler(t, emitter)
	t.Cleanup(func() { svc.Shutdown(mqttShutdownTimeout) })
	subID := connectBroker(t, h, "sub")
	pubID := connectBroker(t, h, "pub")
	waitConnected(t, h, subID, 5*time.Second)
	waitConnected(t, h, pubID, 5*time.Second)
	for _, filter := range []string{"#", "overlap/#", "overlap/+", "overlap/specific"} {
		if err := h.Subscribe(subID, filter, 1); err != nil {
			t.Fatalf("Subscribe(%s): %v", filter, err)
		}
	}

	publishOnce := func(topic string) {
		t.Helper()
		if err := h.Publish(pubID, topic, "x", 1, false); err != nil {
			t.Fatalf("Publish(%s): %v", topic, err)
		}
		if msg := emitter.receiveMessage(t, 5*time.Second); msg.Topic != topic || msg.ConnectionID != subID {
			t.Fatalf("message = %+v, want %s on %s", msg, topic, subID)
		}
		emitter.noMessage(t, 300*time.Millisecond)
	}
	// 4 つ全て・2 つ (# と overlap/#)・1 つ (#) に一致するトピック。
	for _, topic := range []string{"overlap/specific", "overlap/a/b", "elsewhere"} {
		publishOnce(topic)
	}

	// スキャンを止めた (# を解除した) あとも、残りの購読で 1 回ずつ届く。
	if err := h.Unsubscribe(subID, "#"); err != nil {
		t.Fatalf("Unsubscribe(#): %v", err)
	}
	publishOnce("overlap/specific")
	if err := h.Publish(pubID, "elsewhere", "x", 1, false); err != nil {
		t.Fatalf("Publish(elsewhere): %v", err)
	}
	emitter.noMessage(t, 300*time.Millisecond)
}

// TestMQTT_TopicScan は、スキャンを始めると publish されたトピックが mqtt:scan-topic で届き、
// スキャンが購読一覧に現れず mqtt:message も発行しないこと、止めると届かなくなることを確認する。
func TestMQTT_TopicScan(t *testing.T) {
	emitter := newMQTTMockEmitter()
	h, svc := newMQTTHandler(t, emitter)
	t.Cleanup(func() { svc.Shutdown(mqttShutdownTimeout) })
	id := connectBroker(t, h, "scan")
	waitConnected(t, h, id, 5*time.Second)

	if err := h.StartTopicScan(id); err != nil {
		t.Fatalf("StartTopicScan: %v", err)
	}
	if status, _ := connectionStatus(h, id); !status.Scanning || len(status.Subscriptions) != 0 {
		t.Errorf("status = %+v, want Scanning without subscriptions", status)
	}
	if err := h.Publish(id, "scan/found", "x", 0, false); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	emitter.waitScanTopic(t, id, "scan/found", 5*time.Second)
	// スキャンはトピックを集めるだけで、購読していないトピックのメッセージは発行しない。
	emitter.noMessage(t, 300*time.Millisecond)

	if err := h.StopTopicScan(id); err != nil {
		t.Fatalf("StopTopicScan: %v", err)
	}
	if status, _ := connectionStatus(h, id); status.Scanning {
		t.Error("Scanning = true after StopTopicScan")
	}
	if err := h.Publish(id, "scan/after-stop", "x", 0, false); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	time.Sleep(300 * time.Millisecond)
	if n := emitter.scanTopicCount(id, "scan/after-stop"); n != 0 {
		t.Errorf("mqtt:scan-topic emitted %d times after StopTopicScan, want 0", n)
	}
	// 元の接続はスキャンを止めても続く。
	if status, ok := connectionStatus(h, id); !ok || !status.Connected {
		t.Errorf("status = %+v, want the connection kept", status)
	}
}

// TestMQTT_TopicScan_ConnectionLost は、スキャン用の接続が切れると mqtt:scan-stopped を 1 回発行して
// スキャンを終え、ブローカーが戻っても元の接続だけが再接続することを確認する。
func TestMQTT_TopicScan_ConnectionLost(t *testing.T) {
	broker := startMQTTBroker(t, "127.0.0.1:0", nil)
	emitter := newMQTTMockEmitter()
	h, svc := newMQTTHandler(t, emitter)
	t.Cleanup(func() { svc.Shutdown(mqttShutdownTimeout) })

	id, err := h.Connect(mqttdomain.ConnectionConfig{Name: "scan-lost", Broker: "tcp://" + broker.addr})
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	emitter.waitEvent(t, cmndomain.EventMQTTConnected, id, 1, 5*time.Second)
	if err := h.StartTopicScan(id); err != nil {
		t.Fatalf("StartTopicScan: %v", err)
	}

	broker.Close()
	emitter.waitEvent(t, cmndomain.EventMQTTScanStopped, id, 1, 5*time.Second)
	if status, _ := connectionStatus(h, id); status.Scanning {
		t.Error("Scanning = true after the scan connection was lost")
	}

	restarted := startMQTTBroker(t, broker.addr, nil)
	emitter.waitEvent(t, cmndomain.EventMQTTConnected, id, 2, 15*time.Second)
	// スキャン用のクライアントが自動再接続していれば、ここで 2 本目が繋がる。
	time.Sleep(2 * time.Second)
	if n := atomic.LoadInt64(&restarted.Info.ClientsConnected); n != 1 {
		t.Errorf("clients on the restarted broker = %d, want 1 (the scan client must not reconnect)", n)
	}
	if n := emitter.countEvent(cmndomain.EventMQTTScanStopped, id); n != 1 {
		t.Errorf("mqtt:scan-stopped emitted %d times, want 1", n)
	}
}

// connectDuplicatingBroker は、重なる購読ごとに PUBLISH を送るブローカーへ購読用と publish 用の
// 接続を張り、確立を待つ。
func connectDuplicatingBroker(t *testing.T, h *adapters.MQTTHandler) (subID, pubID string) {
	t.Helper()
	broker := testutil.StartDuplicatingBroker(t)
	ids := make([]string, 0, 2)
	for _, name := range []string{"sub", "pub"} {
		id, err := h.Connect(mqttdomain.ConnectionConfig{Name: name, Broker: broker.URL()})
		if err != nil {
			t.Fatalf("Connect(%s): %v", name, err)
		}
		waitConnected(t, h, id, 5*time.Second)
		ids = append(ids, id)
	}
	return ids[0], ids[1]
}

// TestMQTT_TopicScan_DuplicatingBroker_DeliversOnce は、重なる購読ごとに PUBLISH を送るブローカーでも、
// スキャン中に 1 件 publish すると mqtt:message が 1 回だけ発行されることを確認する。
// スキャンの # を同じ接続で購読すると、ユーザーの購読と重なって 2 件届く。
func TestMQTT_TopicScan_DuplicatingBroker_DeliversOnce(t *testing.T) {
	emitter := newMQTTMockEmitter()
	h, svc := newMQTTHandler(t, emitter)
	t.Cleanup(func() { svc.Shutdown(mqttShutdownTimeout) })
	subID, pubID := connectDuplicatingBroker(t, h)

	const topic = "dup/topic"
	if err := h.Subscribe(subID, topic, 0); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	if err := h.StartTopicScan(subID); err != nil {
		t.Fatalf("StartTopicScan: %v", err)
	}

	if err := h.Publish(pubID, topic, "x", 0, false); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	if msg := emitter.receiveMessage(t, 5*time.Second); msg.Topic != topic || msg.ConnectionID != subID {
		t.Fatalf("message = %+v, want %s on %s", msg, topic, subID)
	}
	emitter.waitScanTopic(t, subID, topic, 5*time.Second)
	emitter.noMessage(t, 300*time.Millisecond)
	if n := emitter.scanTopicCount(subID, topic); n != 1 {
		t.Errorf("mqtt:scan-topic emitted %d times, want 1", n)
	}
}

// TestMQTT_DuplicatingBroker_OverlappingUserSubscriptions は、ユーザーが同じ接続に重なる購読
// (# と個別のトピック) を張ったときは、ブローカーが送った 2 件をそのまま発行することを確認する。
// 偽ブローカーが実際に重複して送ることの確認と、重複排除をしないことの固定を兼ねる。
func TestMQTT_DuplicatingBroker_OverlappingUserSubscriptions(t *testing.T) {
	emitter := newMQTTMockEmitter()
	h, svc := newMQTTHandler(t, emitter)
	t.Cleanup(func() { svc.Shutdown(mqttShutdownTimeout) })
	subID, pubID := connectDuplicatingBroker(t, h)

	const topic = "dup/topic"
	for _, filter := range []string{topic, "#"} {
		if err := h.Subscribe(subID, filter, 0); err != nil {
			t.Fatalf("Subscribe(%s): %v", filter, err)
		}
	}

	if err := h.Publish(pubID, topic, "x", 0, false); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	for range 2 {
		if msg := emitter.receiveMessage(t, 5*time.Second); msg.Topic != topic || msg.ConnectionID != subID {
			t.Fatalf("message = %+v, want %s on %s", msg, topic, subID)
		}
	}
	emitter.noMessage(t, 300*time.Millisecond)
}

// TestMQTT_CorruptProfileAmongValidOnes は、壊れたプロファイルだけを退避して正常なもので起動し、
// 中身の ID が不正なファイルは退避せずに読み飛ばすこと、プロファイルのディレクトリを
// 作れなければ起動に失敗することを確認する。
func TestMQTT_CorruptProfileAmongValidOnes(t *testing.T) {
	dir := t.TempDir()
	emitter := newMQTTMockEmitter()
	h1, _ := newMQTTHandlerWithDir(t, emitter, dir)
	valid, err := h1.SaveProfile(mqttdomain.BrokerProfile{Name: "Valid", Broker: brokerAddr})
	if err != nil {
		t.Fatalf("SaveProfile: %v", err)
	}
	const corrupt = "{ not json"
	escape := `{"id":"../escape","name":"Escape","broker":"tcp://localhost:1883","clientId":"","username":"","password":"","useTls":false}`
	for name, content := range map[string]string{"x.json": corrupt, "escape.json": escape} {
		if err = os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatalf("WriteFile(%s): %v", name, err)
		}
	}

	h2, _ := newMQTTHandlerWithDir(t, emitter, dir)
	profiles := h2.GetProfiles()
	if len(profiles) != 1 || profiles[0].ID != valid.ID {
		t.Fatalf("profiles = %v, want only %s", profiles, valid.ID)
	}
	got, err := os.ReadFile(filepath.Join(dir, "x.json.corrupt"))
	if err != nil {
		t.Fatalf("x.json.corrupt should exist: %v", err)
	}
	if string(got) != corrupt {
		t.Errorf("x.json.corrupt = %q, want the original bytes", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "escape.json")); err != nil {
		t.Errorf("a file with an invalid ID must be left in place: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "escape.json.corrupt")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a file with an invalid ID must not be quarantined: %v", err)
	}
	if _, err := h2.SaveProfile(mqttdomain.BrokerProfile{Name: "New", Broker: brokerAddr}); err != nil {
		t.Errorf("SaveProfile after skipping broken files: %v", err)
	}

	// ディレクトリの位置に通常のファイルがあると作成できず、起動に失敗する。
	blocked := filepath.Join(t.TempDir(), "mqtt-profiles")
	if err := os.WriteFile(blocked, []byte("not a dir"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := mqttinfra.NewProfileRepository(blocked, nil); err == nil {
		t.Error("NewProfileRepository should fail when the directory cannot be created")
	}
}

// TestMQTT_LifecycleEvents は、Connect で mqtt:connected、Disconnect で mqtt:disconnected が
// connectionId 付きで 1 回ずつ出てその後は何も出ないことと、接続失敗では
// mqtt:connection-failed だけが出て mqtt:connected が出ないことを確認する。
func TestMQTT_LifecycleEvents(t *testing.T) {
	emitter := newMQTTMockEmitter()
	h, svc := newMQTTHandlerWithConfig(t, emitter, mqttinfra.MQTTClientConfig{
		ConnectTimeout: time.Second,
		TokenTimeout:   3 * time.Second,
	})
	t.Cleanup(func() { svc.Shutdown(mqttShutdownTimeout) })

	id := connectBroker(t, h, "lifecycle")
	emitter.waitEvent(t, cmndomain.EventMQTTConnected, id, 1, 5*time.Second)
	if err := h.Disconnect(id); err != nil {
		t.Fatalf("Disconnect: %v", err)
	}
	emitter.waitEvent(t, cmndomain.EventMQTTDisconnected, id, 1, 5*time.Second)
	emitter.assertSilent(t, 300*time.Millisecond)
	if got, want := emitter.lifecycle(id), []string{cmndomain.EventMQTTConnected, cmndomain.EventMQTTDisconnected}; !slices.Equal(got, want) {
		t.Errorf("events = %v, want %v", got, want)
	}

	failed, err := h.Connect(mqttdomain.ConnectionConfig{Name: "unreachable", Broker: fmt.Sprintf("tcp://127.0.0.1:%d", freePort(t))})
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	emitter.waitEvent(t, cmndomain.EventMQTTConnectionFailed, failed, 1, 10*time.Second)
	emitter.assertSilent(t, 300*time.Millisecond)
	if got, want := emitter.lifecycle(failed), []string{cmndomain.EventMQTTConnectionFailed}; !slices.Equal(got, want) {
		t.Errorf("events = %v, want %v", got, want)
	}
}
