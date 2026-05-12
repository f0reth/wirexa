//go:build integration

package integration

import (
	"fmt"
	"maps"
	"net"
	"os"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/hooks/auth"
	"github.com/mochi-mqtt/server/v2/listeners"

	"github.com/f0reth/Wirexa/internal/adapters"
	mqttapp "github.com/f0reth/Wirexa/internal/application/mqtt"
	cmndomain "github.com/f0reth/Wirexa/internal/domain"
	mqttdomain "github.com/f0reth/Wirexa/internal/domain/mqtt"
	infra "github.com/f0reth/Wirexa/internal/infrastructure"
	mqttinfra "github.com/f0reth/Wirexa/internal/infrastructure/mqtt"
	"github.com/f0reth/Wirexa/internal/testutil"
)

// brokerAddr はテスト全体で使う embedded MQTT ブローカーのアドレス (tcp://127.0.0.1:PORT)。
var brokerAddr string

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

// mqttMockEmitter はテスト用の Emitter 実装。受信 MQTT メッセージとイベントをチャンネルで収集する。
type mqttMockEmitter struct {
	ch     chan mqttdomain.MqttMessage
	failCh chan string // mqtt:connection-failed イベントの connectionId を受信する
}

func newMqttMockEmitter() *mqttMockEmitter {
	return &mqttMockEmitter{
		ch:     make(chan mqttdomain.MqttMessage, 16),
		failCh: make(chan string, 16),
	}
}

func (e *mqttMockEmitter) Emit(event string, data any) {
	if msg, ok := data.(mqttdomain.MqttMessage); ok {
		select {
		case e.ch <- msg:
		default:
		}
	}
	if event == "mqtt:connection-failed" {
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
func (e *mqttMockEmitter) receiveMessage(t *testing.T, timeout time.Duration) mqttdomain.MqttMessage {
	t.Helper()
	select {
	case msg := <-e.ch:
		return msg
	case <-time.After(timeout):
		t.Fatal("timeout waiting for MQTT message")
		return mqttdomain.MqttMessage{}
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

// newMQTTHandlerWithDir は指定ディレクトリから MqttHandler を組み立てる（永続化テスト用）。
func newMQTTHandlerWithDir(t *testing.T, emitter cmndomain.Emitter, dir string) *adapters.MqttHandler {
	t.Helper()
	repo, err := infra.NewJSONStore(dir, func(p *mqttdomain.BrokerProfile) string { return p.ID })
	if err != nil {
		t.Fatalf("NewJSONStore: %v", err)
	}
	profileSvc, err := mqttapp.NewProfileService(repo)
	if err != nil {
		t.Fatalf("NewProfileService: %v", err)
	}
	mqttSvc := mqttapp.NewMqttService(emitter, mqttinfra.NewPahoClientFactory(mqttinfra.MqttClientConfig{}), testutil.NoopLogger{})
	h := &adapters.MqttHandler{}
	adapters.SetupMqttHandler(h, mqttSvc, profileSvc)
	return h
}

// newMQTTHandlerWithConfig は指定クライアント設定で MqttHandler を組み立てる（タイムアウトテスト用）。
func newMQTTHandlerWithConfig(t *testing.T, emitter cmndomain.Emitter, cfg mqttinfra.MqttClientConfig) *adapters.MqttHandler {
	t.Helper()
	repo, err := infra.NewJSONStore(t.TempDir(), func(p *mqttdomain.BrokerProfile) string { return p.ID })
	if err != nil {
		t.Fatalf("NewJSONStore: %v", err)
	}
	profileSvc, err := mqttapp.NewProfileService(repo)
	if err != nil {
		t.Fatalf("NewProfileService: %v", err)
	}
	mqttSvc := mqttapp.NewMqttService(emitter, mqttinfra.NewPahoClientFactory(cfg), testutil.NoopLogger{})
	h := &adapters.MqttHandler{}
	adapters.SetupMqttHandler(h, mqttSvc, profileSvc)
	return h
}

// newMQTTHandler は統合テスト用に MqttHandler を DI で組み立てる。
func newMQTTHandler(t *testing.T, emitter cmndomain.Emitter) *adapters.MqttHandler {
	t.Helper()
	return newMQTTHandlerWithDir(t, emitter, t.TempDir())
}

// connectBroker は brokerAddr へ接続して connectionID を返すヘルパー。
func connectBroker(t *testing.T, h *adapters.MqttHandler, name string) string {
	t.Helper()
	id, err := h.Connect(adapters.ConnectionConfig{
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
func waitConnected(t *testing.T, h *adapters.MqttHandler, connID string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		for _, cs := range h.GetConnections() {
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
	emitter := newMqttMockEmitter()
	h := newMQTTHandler(t, emitter)

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
	emitter := newMqttMockEmitter()
	h := newMQTTHandler(t, emitter)

	const n = 3
	ids := make([]string, n)
	for i := range n {
		ids[i] = connectBroker(t, h, fmt.Sprintf("conn-%d", i))
	}
	t.Cleanup(func() { h.Shutdown() })

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
	emitter := newMqttMockEmitter()
	h := newMQTTHandler(t, emitter)

	if conns := h.GetConnections(); len(conns) != 0 {
		t.Fatalf("expected 0 connections initially, got %d", len(conns))
	}

	id1 := connectBroker(t, h, "c1")
	id2 := connectBroker(t, h, "c2")
	t.Cleanup(func() { h.Shutdown() })

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
	emitter := newMqttMockEmitter()
	h := newMQTTHandler(t, emitter)

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
	emitter := newMqttMockEmitter()
	h := newMQTTHandler(t, emitter)

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
	emitter := newMqttMockEmitter()
	h := newMQTTHandler(t, emitter)

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
	emitter := newMqttMockEmitter()
	h := newMQTTHandler(t, emitter)

	if profiles := h.GetProfiles(); len(profiles) != 0 {
		t.Fatalf("expected 0 profiles initially, got %d", len(profiles))
	}

	profile := adapters.BrokerProfile{
		ID:     uuid.New().String(),
		Name:   "LocalBroker",
		Broker: brokerAddr,
	}
	if err := h.SaveProfile(profile); err != nil {
		t.Fatalf("SaveProfile: %v", err)
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
	if err := h.SaveProfile(saved); err != nil {
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

// TestMQTT_Shutdown は全接続が切断されることを確認する。
func TestMQTT_Shutdown(t *testing.T) {
	emitter := newMqttMockEmitter()
	h := newMQTTHandler(t, emitter)

	id1 := connectBroker(t, h, "s1")
	id2 := connectBroker(t, h, "s2")
	waitConnected(t, h, id1, 5*time.Second)
	waitConnected(t, h, id2, 5*time.Second)

	if len(h.GetConnections()) != 2 {
		t.Fatalf("expected 2 connections before shutdown")
	}

	h.Shutdown()

	if conns := h.GetConnections(); len(conns) != 0 {
		t.Errorf("expected 0 connections after shutdown, got %d", len(conns))
	}
}

// TestMQTT_ProfilePersistenceRoundTrip は SaveProfile 後に同一 dir で再作成した Handler でデータが復元されることを確認する。
func TestMQTT_ProfilePersistenceRoundTrip(t *testing.T) {
	dir := t.TempDir()
	emitter := newMqttMockEmitter()

	// 1 回目: プロファイルを保存
	h1 := newMQTTHandlerWithDir(t, emitter, dir)
	profile := adapters.BrokerProfile{
		ID:     uuid.New().String(),
		Name:   "PersistProfile",
		Broker: brokerAddr,
	}
	if err := h1.SaveProfile(profile); err != nil {
		t.Fatalf("SaveProfile: %v", err)
	}

	// 2 回目: 同一 dir から Handler を再作成してデータを確認
	h2 := newMQTTHandlerWithDir(t, emitter, dir)
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
	emitter := newMqttMockEmitter()
	// 短いタイムアウトを設定してテストを高速化する
	h := newMQTTHandlerWithConfig(t, emitter, mqttinfra.MqttClientConfig{
		ConnectTimeout: 1 * time.Second,
		TokenTimeout:   3 * time.Second,
	})

	// 空きポート番号を取得し（何もリッスンしていない）、接続を試みる
	port := freePort(t)
	unreachableBroker := fmt.Sprintf("tcp://127.0.0.1:%d", port)

	connID, err := h.Connect(adapters.ConnectionConfig{
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
	emitter := newMqttMockEmitter()
	h := newMQTTHandler(t, emitter)

	_, err := h.Connect(adapters.ConnectionConfig{Name: "empty", Broker: ""})
	if err == nil {
		t.Error("expected error for empty broker, got nil")
	}
}

// TestMQTT_Publish_InvalidInput は topic 空文字または qos>2 で ValidationError が返ることを確認する。
func TestMQTT_Publish_InvalidInput(t *testing.T) {
	emitter := newMqttMockEmitter()
	h := newMQTTHandler(t, emitter)

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
	emitter := newMqttMockEmitter()
	h := newMQTTHandler(t, emitter)

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
	emitter := newMqttMockEmitter()
	h := newMQTTHandler(t, emitter)

	if err := h.Disconnect("nonexistent-id"); err == nil {
		t.Error("expected error for nonexistent connection, got nil")
	}
}

// TestMQTT_Disconnect_Twice は同じ connectionID に対して 2 回目の Disconnect が error を返すことを確認する。
func TestMQTT_Disconnect_Twice(t *testing.T) {
	emitter := newMqttMockEmitter()
	h := newMQTTHandler(t, emitter)

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
	emitter := newMqttMockEmitter()
	h := newMQTTHandler(t, emitter)

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
	emitter := newMqttMockEmitter()
	h := newMQTTHandler(t, emitter)

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
	emitter := newMqttMockEmitter()
	h := newMQTTHandler(t, emitter)

	connID := connectBroker(t, h, "unsub-empty")
	t.Cleanup(func() { _ = h.Disconnect(connID) })
	waitConnected(t, h, connID, 5*time.Second)

	if err := h.Unsubscribe(connID, ""); err == nil {
		t.Error("expected error for empty topic, got nil")
	}
}

// TestMQTT_Connect_Concurrent は複数 goroutine から並行して Connect / Disconnect を呼んでも安全であることを確認する。
func TestMQTT_Connect_Concurrent(t *testing.T) {
	emitter := newMqttMockEmitter()
	h := newMQTTHandler(t, emitter)
	t.Cleanup(func() { h.Shutdown() })

	const n = 5
	var wg sync.WaitGroup
	ids := make(chan string, n)

	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id, err := h.Connect(adapters.ConnectionConfig{
				Name:   "concurrent",
				Broker: brokerAddr,
			})
			if err == nil {
				ids <- id
			}
		}()
	}
	wg.Wait()
	close(ids)

	// 全接続を切断する
	for id := range ids {
		_ = h.Disconnect(id)
	}
}
