//go:build integration

package integration

import (
	"fmt"
	"maps"
	"net"
	"os"
	"slices"
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

// mqttMockEmitter はテスト用の Emitter 実装。受信 MQTT メッセージをチャンネルで収集する。
type mqttMockEmitter struct {
	ch chan mqttdomain.MqttMessage
}

func newMqttMockEmitter() *mqttMockEmitter {
	return &mqttMockEmitter{ch: make(chan mqttdomain.MqttMessage, 16)}
}

func (e *mqttMockEmitter) Emit(_ string, data any) {
	if msg, ok := data.(mqttdomain.MqttMessage); ok {
		select {
		case e.ch <- msg:
		default:
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

// newMQTTHandler は統合テスト用に MqttHandler を DI で組み立てる。
func newMQTTHandler(t *testing.T, emitter cmndomain.Emitter) *adapters.MqttHandler {
	t.Helper()
	repo, err := mqttinfra.NewJSONProfileRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewJSONProfileRepository: %v", err)
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

// connectBroker は brokerAddr へ接続して connectionID を返すヘルパー。
func connectBroker(t *testing.T, h *adapters.MqttHandler, name string) string {
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

	profile := mqttdomain.BrokerProfile{
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
