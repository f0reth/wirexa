package mqttapp

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	cmn "github.com/f0reth/Wirexa/internal/domain"
	domain "github.com/f0reth/Wirexa/internal/domain/mqtt"
	"github.com/f0reth/Wirexa/internal/testutil"
)

// mockEmitter は domain.Emitter のインメモリモック。
type mockEmitter struct {
	events []emittedEvent
	mu     sync.Mutex
}

type emittedEvent struct {
	data  any
	event string
}

func (e *mockEmitter) Emit(event string, data any) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.events = append(e.events, emittedEvent{data, event})
}

func (e *mockEmitter) hasEvent(event string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, ev := range e.events {
		if ev.event == event {
			return true
		}
	}
	return false
}

// mockBrokerClient は domain.BrokerClient のモック。
type mockBrokerClient struct {
	connectFn     func(ctx context.Context) error
	disconnectFn  func(quiesce uint)
	publishFn     func(topic string, qos byte, retained bool, payload string) error
	subscribeFn   func(topic string, qos byte, handler domain.MessageHandler) error
	unsubscribeFn func(topics ...string) error
	isConnectedFn func() bool
}

func (m *mockBrokerClient) Connect(ctx context.Context) error {
	if m.connectFn != nil {
		return m.connectFn(ctx)
	}
	return nil
}

func (m *mockBrokerClient) Disconnect(quiesce uint) {
	if m.disconnectFn != nil {
		m.disconnectFn(quiesce)
	}
}

func (m *mockBrokerClient) Publish(topic string, qos byte, retained bool, payload string) error {
	if m.publishFn != nil {
		return m.publishFn(topic, qos, retained, payload)
	}
	return nil
}

func (m *mockBrokerClient) Subscribe(topic string, qos byte, handler domain.MessageHandler) error {
	if m.subscribeFn != nil {
		return m.subscribeFn(topic, qos, handler)
	}
	return nil
}

func (m *mockBrokerClient) Unsubscribe(topics ...string) error {
	if m.unsubscribeFn != nil {
		return m.unsubscribeFn(topics...)
	}
	return nil
}

func (m *mockBrokerClient) IsConnected() bool {
	if m.isConnectedFn != nil {
		return m.isConnectedFn()
	}
	return true
}

// factoryWith は常に client を返す BrokerClientFactory を生成する。
func factoryWith(client domain.BrokerClient) domain.BrokerClientFactory {
	return func(_ domain.ConnectionConfig, _ func(), _ func(error)) domain.BrokerClient {
		return client
	}
}

// waitForEvent はチャンネルからイベントを受信するか、タイムアウトで失敗する。
func waitForEvent(t *testing.T, ch <-chan struct{}, timeout time.Duration, msg string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(timeout):
		t.Fatal(msg)
	}
}

// newTestService はテストの context をルートに持つ MQTTService を生成する。
func newTestService(t *testing.T, emitter cmn.Emitter, factory domain.BrokerClientFactory) *MQTTService {
	t.Helper()
	return NewMQTTService(t.Context(), emitter, factory, testutil.NoopLogger{})
}

// waitEstablished は n 本の接続がすべて Connected になるまで待つ。
// connectFn の復帰だけを待つと、接続 goroutine が結果を状態に反映する前に次の操作が走る。
func waitEstablished(t *testing.T, svc *MQTTService, n int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		conns := svc.GetConnections()
		established := 0
		for i := range conns {
			if conns[i].Connected {
				established++
			}
		}
		if established == n {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("timeout waiting for %d established connections", n)
}

// ------- Connect -------

func TestMQTTService_Connect_EmptyBroker(t *testing.T) {
	svc := newTestService(t, &mockEmitter{}, factoryWith(&mockBrokerClient{}))
	_, err := svc.Connect(domain.ConnectionConfig{Broker: ""})
	if err == nil {
		t.Fatal("expected error for empty broker, got nil")
	}
	if _, ok := errors.AsType[*cmn.ValidationError](err); !ok {
		t.Errorf("expected ValidationError, got %T", err)
	}
}

func TestMQTTService_Connect_ReturnsNonEmptyID(t *testing.T) {
	done := make(chan struct{})
	client := &mockBrokerClient{
		connectFn: func(context.Context) error { close(done); return nil },
	}
	svc := newTestService(t, &mockEmitter{}, factoryWith(client))

	id, err := svc.Connect(domain.ConnectionConfig{Broker: "tcp://localhost:1883"})
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if id == "" {
		t.Error("expected non-empty connection ID")
	}
	waitForEvent(t, done, time.Second, "timeout waiting for connect goroutine")
}

// capturingFactory は factory に渡された設定をチャンネル経由で返す。
// factory は Connect が起動したゴルーチンから呼ばれるため、変数への直接代入だと
// 読み出し側とデータ競合になる (時間で待つ必要も出る)。
func capturingFactory() (domain.BrokerClientFactory, <-chan domain.ConnectionConfig) {
	ch := make(chan domain.ConnectionConfig, 1)
	return func(cfg domain.ConnectionConfig, _ func(), _ func(error)) domain.BrokerClient {
		ch <- cfg
		return &mockBrokerClient{}
	}, ch
}

func waitForConfig(t *testing.T, ch <-chan domain.ConnectionConfig) domain.ConnectionConfig {
	t.Helper()
	select {
	case cfg := <-ch:
		return cfg
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for factory call")
		return domain.ConnectionConfig{}
	}
}

func TestMQTTService_Connect_AutoGeneratesClientID(t *testing.T) {
	factory, configs := capturingFactory()
	svc := newTestService(t, &mockEmitter{}, factory)
	svc.Connect(domain.ConnectionConfig{Broker: "tcp://localhost:1883", ClientID: ""})

	if cfg := waitForConfig(t, configs); cfg.ClientID == "" {
		t.Error("expected auto-generated ClientID, got empty")
	}
}

func TestMQTTService_Connect_UsesProvidedClientID(t *testing.T) {
	factory, configs := capturingFactory()
	svc := newTestService(t, &mockEmitter{}, factory)
	svc.Connect(domain.ConnectionConfig{Broker: "tcp://localhost:1883", ClientID: "my-client"})

	if cfg := waitForConfig(t, configs); cfg.ClientID != "my-client" {
		t.Errorf("ClientID = %q, want %q", cfg.ClientID, "my-client")
	}
}

func TestMQTTService_Connect_FailureRemovesConnection(t *testing.T) {
	failedCh := make(chan struct{})
	emitter2 := &mockEmitterWithChan{mockEmitter: mockEmitter{}, ch: failedCh, targetEvent: cmn.EventMQTTConnectionFailed}

	client := &mockBrokerClient{
		connectFn: func(context.Context) error { return errors.New("connection refused") },
	}
	svc := newTestService(t, emitter2, factoryWith(client))
	id, err := svc.Connect(domain.ConnectionConfig{Broker: "tcp://localhost:1883"})
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if id == "" {
		t.Error("expected non-empty ID")
	}

	waitForEvent(t, failedCh, time.Second, "timeout waiting for connection-failed event")

	// After failure, connection should be removed
	conns := svc.GetConnections()
	if len(conns) != 0 {
		t.Errorf("expected 0 connections after failure, got %d", len(conns))
	}
}

// mockEmitterWithChan は特定イベントを検出してチャンネルに通知するモック。
type mockEmitterWithChan struct {
	ch          chan struct{}
	targetEvent string
	mockEmitter
	once sync.Once
}

func (e *mockEmitterWithChan) Emit(event string, data any) {
	e.mockEmitter.Emit(event, data)
	if event == e.targetEvent {
		e.once.Do(func() { close(e.ch) })
	}
}

// ------- Disconnect -------

func TestMQTTService_Disconnect_NotFound(t *testing.T) {
	svc := newTestService(t, &mockEmitter{}, factoryWith(&mockBrokerClient{}))
	err := svc.Disconnect("nonexistent")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if _, ok := errors.AsType[*cmn.NotFoundError](err); !ok {
		t.Errorf("expected NotFoundError, got %T", err)
	}
}

func TestMQTTService_Disconnect_Success(t *testing.T) {
	done := make(chan struct{})
	client := &mockBrokerClient{
		connectFn: func(context.Context) error { close(done); return nil },
	}
	emitter := &mockEmitterWithChan{
		ch:          make(chan struct{}),
		targetEvent: cmn.EventMQTTDisconnected,
	}
	svc := newTestService(t, emitter, factoryWith(client))

	id, _ := svc.Connect(domain.ConnectionConfig{Broker: "tcp://localhost:1883"})
	waitForEvent(t, done, time.Second, "connect goroutine timeout")

	if err := svc.Disconnect(id); err != nil {
		t.Fatalf("Disconnect: %v", err)
	}
	waitForEvent(t, emitter.ch, time.Second, "disconnected event timeout")

	if len(svc.GetConnections()) != 0 {
		t.Error("expected 0 connections after disconnect")
	}
}

// ------- Publish -------

func TestMQTTService_Publish_EmptyTopic(t *testing.T) {
	svc := newTestService(t, &mockEmitter{}, factoryWith(&mockBrokerClient{}))
	err := svc.Publish("connid", "", "payload", 0, false)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if _, ok := errors.AsType[*cmn.ValidationError](err); !ok {
		t.Errorf("expected ValidationError, got %T", err)
	}
}

func TestMQTTService_Publish_InvalidQoS(t *testing.T) {
	svc := newTestService(t, &mockEmitter{}, factoryWith(&mockBrokerClient{}))
	err := svc.Publish("connid", "topic", "payload", 3, false)
	if err == nil {
		t.Fatal("expected error for qos=3, got nil")
	}
	if _, ok := errors.AsType[*cmn.ValidationError](err); !ok {
		t.Errorf("expected ValidationError, got %T", err)
	}
}

func TestMQTTService_Publish_ConnectionNotFound(t *testing.T) {
	svc := newTestService(t, &mockEmitter{}, factoryWith(&mockBrokerClient{}))
	err := svc.Publish("nonexistent", "topic", "payload", 0, false)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if _, ok := errors.AsType[*cmn.NotFoundError](err); !ok {
		t.Errorf("expected NotFoundError, got %T", err)
	}
}

func TestMQTTService_Publish_Success(t *testing.T) {
	done := make(chan struct{})
	var publishedTopic, publishedPayload string
	var publishedQoS byte
	client := &mockBrokerClient{
		connectFn: func(context.Context) error { close(done); return nil },
		publishFn: func(topic string, qos byte, _ bool, payload string) error {
			publishedTopic = topic
			publishedQoS = qos
			publishedPayload = payload
			return nil
		},
	}
	svc := newTestService(t, &mockEmitter{}, factoryWith(client))
	id, _ := svc.Connect(domain.ConnectionConfig{Broker: "tcp://localhost:1883"})
	waitForEvent(t, done, time.Second, "connect goroutine timeout")

	if err := svc.Publish(id, "sensors/temp", "25.5", 1, false); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if publishedTopic != "sensors/temp" {
		t.Errorf("topic = %q, want %q", publishedTopic, "sensors/temp")
	}
	if publishedPayload != "25.5" {
		t.Errorf("payload = %q, want %q", publishedPayload, "25.5")
	}
	if publishedQoS != 1 {
		t.Errorf("qos = %d, want 1", publishedQoS)
	}
}

func TestMQTTService_Publish_ValidQoSValues(t *testing.T) {
	done := make(chan struct{})
	var once sync.Once
	client := &mockBrokerClient{
		connectFn: func(context.Context) error { once.Do(func() { close(done) }); return nil },
	}
	svc := newTestService(t, &mockEmitter{}, factoryWith(client))
	id, _ := svc.Connect(domain.ConnectionConfig{Broker: "tcp://localhost:1883"})
	waitForEvent(t, done, time.Second, "connect goroutine timeout")

	for _, qos := range []byte{0, 1, 2} {
		if err := svc.Publish(id, "topic", "msg", qos, false); err != nil {
			t.Errorf("Publish with qos=%d: %v", qos, err)
		}
	}
}

// ------- Subscribe -------

func TestMQTTService_Subscribe_EmptyTopic(t *testing.T) {
	svc := newTestService(t, &mockEmitter{}, factoryWith(&mockBrokerClient{}))
	err := svc.Subscribe("connid", "", 0)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestMQTTService_Subscribe_InvalidQoS(t *testing.T) {
	svc := newTestService(t, &mockEmitter{}, factoryWith(&mockBrokerClient{}))
	err := svc.Subscribe("connid", "topic", 3)
	if err == nil {
		t.Fatal("expected error for qos=3, got nil")
	}
}

func TestMQTTService_Subscribe_ConnectionNotFound(t *testing.T) {
	svc := newTestService(t, &mockEmitter{}, factoryWith(&mockBrokerClient{}))
	err := svc.Subscribe("nonexistent", "topic", 0)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if _, ok := errors.AsType[*cmn.NotFoundError](err); !ok {
		t.Errorf("expected NotFoundError, got %T", err)
	}
}

func TestMQTTService_Subscribe_MessageHandlerEmitsEvent(t *testing.T) {
	done := make(chan struct{})
	var capturedHandler domain.MessageHandler
	client := &mockBrokerClient{
		connectFn: func(context.Context) error { close(done); return nil },
		subscribeFn: func(_ string, _ byte, handler domain.MessageHandler) error {
			capturedHandler = handler
			return nil
		},
	}
	msgCh := make(chan struct{})
	emitter := &mockEmitterWithChan{
		ch:          msgCh,
		targetEvent: cmn.EventMQTTMessage,
	}
	svc := newTestService(t, emitter, factoryWith(client))
	id, _ := svc.Connect(domain.ConnectionConfig{Broker: "tcp://localhost:1883"})
	waitForEvent(t, done, time.Second, "connect goroutine timeout")

	if err := svc.Subscribe(id, "sensors/#", 0); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	if capturedHandler == nil {
		t.Fatal("handler not set")
	}
	capturedHandler("sensors/temp", []byte("25.5"), 0, false)
	waitForEvent(t, msgCh, time.Second, "message event timeout")
	if !emitter.hasEvent(cmn.EventMQTTMessage) {
		t.Error("expected mqtt:message event")
	}
	if msg := lastMessage(t, emitter); msg.PayloadBase64 || msg.Payload != "25.5" {
		t.Errorf("expected UTF-8 payload passthrough, got %+v", msg)
	}
}

func TestMQTTService_Subscribe_BinaryPayloadBase64(t *testing.T) {
	done := make(chan struct{})
	var capturedHandler domain.MessageHandler
	client := &mockBrokerClient{
		connectFn: func(context.Context) error { close(done); return nil },
		subscribeFn: func(_ string, _ byte, handler domain.MessageHandler) error {
			capturedHandler = handler
			return nil
		},
	}
	msgCh := make(chan struct{})
	emitter := &mockEmitterWithChan{ch: msgCh, targetEvent: cmn.EventMQTTMessage}
	svc := newTestService(t, emitter, factoryWith(client))
	id, _ := svc.Connect(domain.ConnectionConfig{Broker: "tcp://localhost:1883"})
	waitForEvent(t, done, time.Second, "connect goroutine timeout")

	if err := svc.Subscribe(id, "sensors/#", 0); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	if capturedHandler == nil {
		t.Fatal("handler not set")
	}

	// 不正な UTF-8 を含むバイナリペイロード
	binary := []byte{0x00, 0xff, 0xfe, 0x80}
	capturedHandler("sensors/raw", binary, 0, false)
	waitForEvent(t, msgCh, time.Second, "message event timeout")

	msg := lastMessage(t, emitter)
	if !msg.PayloadBase64 {
		t.Fatal("expected PayloadBase64=true for non-UTF-8 payload")
	}
	got, err := base64.StdEncoding.DecodeString(msg.Payload)
	if err != nil {
		t.Fatalf("payload is not valid base64: %v", err)
	}
	if !bytes.Equal(got, binary) {
		t.Errorf("decoded payload mismatch: got %v want %v", got, binary)
	}
}

// lastMessage は emitter に記録された最後の mqtt:message イベントの MQTTMessage を返す。
func lastMessage(t *testing.T, e *mockEmitterWithChan) domain.MQTTMessage {
	t.Helper()
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, event := range slices.Backward(e.events) {
		if event.event == cmn.EventMQTTMessage {
			msg, ok := event.data.(domain.MQTTMessage)
			if !ok {
				t.Fatalf("message event data is not MQTTMessage: %T", event.data)
			}
			return msg
		}
	}
	t.Fatal("no mqtt:message event recorded")
	return domain.MQTTMessage{}
}

// ------- Unsubscribe -------

func TestMQTTService_Unsubscribe_EmptyTopic(t *testing.T) {
	svc := newTestService(t, &mockEmitter{}, factoryWith(&mockBrokerClient{}))
	err := svc.Unsubscribe("connid", "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestMQTTService_Unsubscribe_ConnectionNotFound(t *testing.T) {
	svc := newTestService(t, &mockEmitter{}, factoryWith(&mockBrokerClient{}))
	err := svc.Unsubscribe("nonexistent", "topic")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if _, ok := errors.AsType[*cmn.NotFoundError](err); !ok {
		t.Errorf("expected NotFoundError, got %T", err)
	}
}

func TestMQTTService_Unsubscribe_Success(t *testing.T) {
	done := make(chan struct{})
	var unsubscribedTopics []string
	client := &mockBrokerClient{
		connectFn: func(context.Context) error { close(done); return nil },
		unsubscribeFn: func(topics ...string) error {
			unsubscribedTopics = topics
			return nil
		},
	}
	svc := newTestService(t, &mockEmitter{}, factoryWith(client))
	id, _ := svc.Connect(domain.ConnectionConfig{Broker: "tcp://localhost:1883"})
	waitForEvent(t, done, time.Second, "connect goroutine timeout")

	if err := svc.Unsubscribe(id, "sensors/temp"); err != nil {
		t.Fatalf("Unsubscribe: %v", err)
	}
	if len(unsubscribedTopics) != 1 || unsubscribedTopics[0] != "sensors/temp" {
		t.Errorf("unsubscribed topics = %v, want [sensors/temp]", unsubscribedTopics)
	}
}

// ------- GetConnections -------

func TestMQTTService_GetConnections_Empty(t *testing.T) {
	svc := newTestService(t, &mockEmitter{}, factoryWith(&mockBrokerClient{}))
	conns := svc.GetConnections()
	if len(conns) != 0 {
		t.Errorf("expected 0 connections, got %d", len(conns))
	}
}

func TestMQTTService_GetConnections_ReflectsIsConnected(t *testing.T) {
	connected := true
	client := &mockBrokerClient{
		isConnectedFn: func() bool { return connected },
	}
	svc := newTestService(t, &mockEmitter{}, factoryWith(client))
	svc.Connect(domain.ConnectionConfig{Broker: "tcp://localhost:1883", Name: "TestConn"})
	waitEstablished(t, svc, 1)

	conns := svc.GetConnections()
	if len(conns) != 1 {
		t.Fatalf("expected 1 connection, got %d", len(conns))
	}
	if !conns[0].Connected {
		t.Error("expected Connected=true")
	}
	if conns[0].Name != "TestConn" {
		t.Errorf("Name = %q, want TestConn", conns[0].Name)
	}
}

func TestMQTTService_GetConnections_TracksProfileIDAndSubscriptions(t *testing.T) {
	done := make(chan struct{})
	client := &mockBrokerClient{
		connectFn: func(context.Context) error { close(done); return nil },
	}
	svc := newTestService(t, &mockEmitter{}, factoryWith(client))
	id, _ := svc.Connect(domain.ConnectionConfig{
		Broker:    "tcp://localhost:1883",
		ProfileID: "profile-123",
	})
	waitForEvent(t, done, time.Second, "connect goroutine timeout")

	if err := svc.Subscribe(id, "sensors/temp", 1); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	if err := svc.Subscribe(id, "sensors/#", 0); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	conns := svc.GetConnections()
	if len(conns) != 1 {
		t.Fatalf("expected 1 connection, got %d", len(conns))
	}
	if conns[0].ProfileID != "profile-123" {
		t.Errorf("ProfileID = %q, want profile-123", conns[0].ProfileID)
	}
	subs := map[string]byte{}
	for _, s := range conns[0].Subscriptions {
		subs[s.Topic] = s.QoS
	}
	if len(subs) != 2 {
		t.Fatalf("expected 2 subscriptions, got %d (%v)", len(subs), conns[0].Subscriptions)
	}
	if subs["sensors/temp"] != 1 {
		t.Errorf("sensors/temp qos = %d, want 1", subs["sensors/temp"])
	}
	if _, ok := subs["sensors/#"]; !ok {
		t.Error("expected sensors/# subscription to be tracked")
	}

	// Unsubscribe removes the topic from tracking.
	if err := svc.Unsubscribe(id, "sensors/temp"); err != nil {
		t.Fatalf("Unsubscribe: %v", err)
	}
	conns = svc.GetConnections()
	for _, s := range conns[0].Subscriptions {
		if s.Topic == "sensors/temp" {
			t.Error("sensors/temp should be removed after Unsubscribe")
		}
	}
	if len(conns[0].Subscriptions) != 1 {
		t.Errorf("expected 1 subscription after unsubscribe, got %d", len(conns[0].Subscriptions))
	}
}

// ------- Shutdown -------

func TestMQTTService_Shutdown_DisconnectsAll(t *testing.T) {
	disconnectCount := 0
	var mu sync.Mutex
	client := &mockBrokerClient{
		disconnectFn: func(_ uint) {
			mu.Lock()
			disconnectCount++
			mu.Unlock()
		},
	}
	svc := newTestService(t, &mockEmitter{}, factoryWith(client))
	svc.Connect(domain.ConnectionConfig{Broker: "tcp://localhost:1883"})
	waitEstablished(t, svc, 1)

	if !svc.Shutdown(time.Second) {
		t.Error("expected Shutdown to drain within timeout")
	}

	mu.Lock()
	count := disconnectCount
	mu.Unlock()
	if count != 1 {
		t.Errorf("expected 1 disconnect call, got %d", count)
	}
	if len(svc.GetConnections()) != 0 {
		t.Error("expected 0 connections after shutdown")
	}
}

func TestMQTTService_Shutdown_NoConnections(t *testing.T) {
	svc := newTestService(t, &mockEmitter{}, factoryWith(&mockBrokerClient{}))
	// パニックもブロックもせずに true を返す
	if !svc.Shutdown(time.Second) {
		t.Error("expected Shutdown to drain within timeout")
	}
}

func TestMQTTService_Shutdown_DisconnectsMultipleConnections(t *testing.T) {
	connCount := 2
	disconnectCount := 0
	var countMu sync.Mutex

	factory := func(_ domain.ConnectionConfig, _ func(), _ func(error)) domain.BrokerClient {
		return &mockBrokerClient{
			disconnectFn: func(_ uint) {
				countMu.Lock()
				disconnectCount++
				countMu.Unlock()
			},
		}
	}
	svc := newTestService(t, &mockEmitter{}, factory)
	for range connCount {
		svc.Connect(domain.ConnectionConfig{Broker: "tcp://localhost:1883"})
	}
	waitEstablished(t, svc, connCount)

	if !svc.Shutdown(time.Second) {
		t.Error("expected Shutdown to drain within timeout")
	}

	countMu.Lock()
	count := disconnectCount
	countMu.Unlock()
	if count != connCount {
		t.Errorf("expected %d disconnect calls, got %d", connCount, count)
	}
	if len(svc.GetConnections()) != 0 {
		t.Error("expected 0 connections after shutdown")
	}
}

// ------- Publish / Subscribe / Unsubscribe: client errors -------

func TestMQTTService_Publish_ClientError(t *testing.T) {
	done := make(chan struct{})
	wantErr := errors.New("publish failed")
	client := &mockBrokerClient{
		connectFn: func(context.Context) error { close(done); return nil },
		publishFn: func(_ string, _ byte, _ bool, _ string) error { return wantErr },
	}
	svc := newTestService(t, &mockEmitter{}, factoryWith(client))
	id, _ := svc.Connect(domain.ConnectionConfig{Broker: "tcp://localhost:1883"})
	waitForEvent(t, done, time.Second, "connect goroutine timeout")

	err := svc.Publish(id, "topic", "msg", 0, false)
	if !errors.Is(err, wantErr) {
		t.Errorf("expected publish error, got %v", err)
	}
}

func TestMQTTService_Subscribe_ClientError(t *testing.T) {
	done := make(chan struct{})
	wantErr := errors.New("subscribe failed")
	client := &mockBrokerClient{
		connectFn:   func(context.Context) error { close(done); return nil },
		subscribeFn: func(_ string, _ byte, _ domain.MessageHandler) error { return wantErr },
	}
	svc := newTestService(t, &mockEmitter{}, factoryWith(client))
	id, _ := svc.Connect(domain.ConnectionConfig{Broker: "tcp://localhost:1883"})
	waitForEvent(t, done, time.Second, "connect goroutine timeout")

	err := svc.Subscribe(id, "topic", 0)
	if !errors.Is(err, wantErr) {
		t.Errorf("expected subscribe error, got %v", err)
	}
}

func TestMQTTService_Unsubscribe_ClientError(t *testing.T) {
	done := make(chan struct{})
	wantErr := errors.New("unsubscribe failed")
	client := &mockBrokerClient{
		connectFn:     func(context.Context) error { close(done); return nil },
		unsubscribeFn: func(_ ...string) error { return wantErr },
	}
	svc := newTestService(t, &mockEmitter{}, factoryWith(client))
	id, _ := svc.Connect(domain.ConnectionConfig{Broker: "tcp://localhost:1883"})
	waitForEvent(t, done, time.Second, "connect goroutine timeout")

	err := svc.Unsubscribe(id, "topic")
	if !errors.Is(err, wantErr) {
		t.Errorf("expected unsubscribe error, got %v", err)
	}
}

func TestMQTTService_Subscribe_InvalidQoS_ReturnsValidationError(t *testing.T) {
	svc := newTestService(t, &mockEmitter{}, factoryWith(&mockBrokerClient{}))
	err := svc.Subscribe("connid", "topic", 3)
	if err == nil {
		t.Fatal("expected error for qos=3, got nil")
	}
	if _, ok := errors.AsType[*cmn.ValidationError](err); !ok {
		t.Errorf("expected ValidationError, got %T", err)
	}
}

// ------- 接続ライフサイクルの競合 -------

// barrierEmitter は発行されたイベントを記録し、blockOn の最初の発行中に barrier で停止する。
// Emit は接続の stateMu 保持中に呼ばれるため、MQTTService を呼び返してはならない。
type barrierEmitter struct {
	entered chan struct{}
	release chan struct{}
	blockOn string
	mockEmitter
	once sync.Once
}

func newBarrierEmitter(blockOn string) *barrierEmitter {
	return &barrierEmitter{
		entered: make(chan struct{}),
		release: make(chan struct{}),
		blockOn: blockOn,
	}
}

func (e *barrierEmitter) Emit(event string, data any) {
	e.mockEmitter.Emit(event, data)
	if event != e.blockOn {
		return
	}
	first := false
	e.once.Do(func() { first = true })
	if first {
		close(e.entered)
		<-e.release
	}
}

// count は記録されたイベントのうち event に一致するものの数を返す。空文字なら全件数を返す。
func (e *mockEmitter) count(event string) int {
	e.mu.Lock()
	defer e.mu.Unlock()
	n := 0
	for _, ev := range e.events {
		if event == "" || ev.event == event {
			n++
		}
	}
	return n
}

func (e *mockEmitter) snapshot() []emittedEvent {
	e.mu.Lock()
	defer e.mu.Unlock()
	return slices.Clone(e.events)
}

// eventConnID はイベントの対象接続 ID を返す。
func eventConnID(ev emittedEvent) string {
	switch data := ev.data.(type) {
	case map[string]any:
		id, _ := data[keyConnectionID].(string)
		return id
	case domain.MQTTMessage:
		return data.ConnectionID
	}
	return ""
}

// callbackRecorder は factory に渡された callback と Subscribe に渡されたハンドラを記録し、
// 任意のタイミングで叩けるようにする。
type callbackRecorder struct {
	connected []func()
	lost      []func(error)
	handlers  []domain.MessageHandler
	mu        sync.Mutex
}

func (r *callbackRecorder) factory(client *mockBrokerClient) domain.BrokerClientFactory {
	return func(_ domain.ConnectionConfig, onConnected func(), onConnectionLost func(error)) domain.BrokerClient {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.connected = append(r.connected, onConnected)
		r.lost = append(r.lost, onConnectionLost)
		return client
	}
}

// subscribeFn は受け取ったハンドラを記録する mockBrokerClient.subscribeFn。
func (r *callbackRecorder) subscribeFn(_ string, _ byte, handler domain.MessageHandler) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers = append(r.handlers, handler)
	return nil
}

func (r *callbackRecorder) onConnected(i int) func() {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.connected[i]
}

func (r *callbackRecorder) onConnectionLost(i int) func(error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.lost[i]
}

func (r *callbackRecorder) handler(i int) domain.MessageHandler {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.handlers[i]
}

// fireAll は記録済みの全 callback と全ハンドラを 1 回ずつ呼ぶ。
func (r *callbackRecorder) fireAll() {
	r.mu.Lock()
	connected := slices.Clone(r.connected)
	lost := slices.Clone(r.lost)
	handlers := slices.Clone(r.handlers)
	r.mu.Unlock()
	for _, f := range connected {
		f()
	}
	for _, f := range lost {
		f(errors.New("connection lost"))
	}
	for _, h := range handlers {
		h("sensors/temp", []byte("1"), 0, false)
	}
}

// assertBlocked は短い猶予の間に ch へ結果が届かない (処理が待たされている) ことを確認する。
func assertBlocked[T any](t *testing.T, ch <-chan T, msg string) {
	t.Helper()
	select {
	case <-ch:
		t.Fatal(msg)
	case <-time.After(50 * time.Millisecond):
	}
}

// waitConnGoroutines は接続 goroutine がすべて復帰するまで待つ。
// 打ち切りが効かない退行でテストがハングしないよう、上限を設ける。
func waitConnGoroutines(t *testing.T, svc *MQTTService) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		svc.connWg.Wait()
		close(done)
	}()
	waitForEvent(t, done, 2*time.Second, "connect goroutine did not return")
}

// waitDetached は Disconnect が接続を map から外すまで待つ。
func waitDetached(t *testing.T, svc *MQTTService) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for len(svc.GetConnections()) != 0 {
		if time.Now().After(deadline) {
			t.Fatal("timeout waiting for connection to be detached")
		}
		time.Sleep(time.Millisecond)
	}
}

// paho と同じく onConnected が Connect の復帰より先に走っても、正常な接続として扱うこと。
func TestMQTTService_Connect_CallbackBeforeToken_KeepsConnection(t *testing.T) {
	rec := &callbackRecorder{}
	var disconnects atomic.Int32
	client := &mockBrokerClient{
		connectFn: func(context.Context) error {
			rec.onConnected(0)()
			return nil
		},
		disconnectFn: func(uint) { disconnects.Add(1) },
	}
	emitter := &mockEmitter{}
	svc := newTestService(t, emitter, rec.factory(client))
	if _, err := svc.Connect(domain.ConnectionConfig{Broker: "tcp://localhost:1883"}); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	waitConnGoroutines(t, svc)

	if n := disconnects.Load(); n != 0 {
		t.Errorf("client.Disconnect called %d times, want 0", n)
	}
	if n := emitter.count(cmn.EventMQTTConnected); n != 1 {
		t.Errorf("mqtt:connected emitted %d times, want 1", n)
	}
	conns := svc.GetConnections()
	if len(conns) != 1 || !conns[0].Connected {
		t.Errorf("expected 1 connected connection, got %+v", conns)
	}
}

// Disconnect の前に opMu を待っていた操作は、Disconnect の後に client を触らないこと。
func TestMQTTService_Disconnect_RejectsQueuedOperation(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	var publishes atomic.Int32
	client := &mockBrokerClient{
		publishFn: func(string, byte, bool, string) error {
			if publishes.Add(1) == 1 {
				close(entered)
				<-release
			}
			return nil
		},
	}
	svc := newTestService(t, &mockEmitter{}, factoryWith(client))
	id, _ := svc.Connect(domain.ConnectionConfig{Broker: "tcp://localhost:1883"})
	waitEstablished(t, svc, 1)

	first := make(chan error, 1)
	go func() { first <- svc.Publish(id, "t", "1", 0, false) }()
	waitForEvent(t, entered, time.Second, "first publish did not start")

	second := make(chan error, 1)
	go func() { second <- svc.Publish(id, "t", "2", 0, false) }()
	// 2 件目が opMu 待ちに入る猶予。間に合わず map 参照で弾かれても期待結果は同じ。
	time.Sleep(20 * time.Millisecond)

	disconnected := make(chan error, 1)
	go func() { disconnected <- svc.Disconnect(id) }()
	waitDetached(t, svc)
	close(release)

	if err := <-first; err != nil {
		t.Errorf("first publish: %v", err)
	}
	if err := <-second; err == nil {
		t.Error("expected queued publish to fail after disconnect")
	} else if _, ok := errors.AsType[*cmn.NotFoundError](err); !ok {
		t.Errorf("expected NotFoundError, got %T (%v)", err, err)
	}
	if err := <-disconnected; err != nil {
		t.Errorf("Disconnect: %v", err)
	}
	if n := publishes.Load(); n != 1 {
		t.Errorf("client.Publish called %d times, want 1", n)
	}
}

// Connect の直後に Shutdown しても、接続 goroutine の復帰を待ってから true を返すこと。
func TestMQTTService_Shutdown_DrainsAcceptedConnect(t *testing.T) {
	var returned atomic.Bool
	client := &mockBrokerClient{
		connectFn: func(ctx context.Context) error {
			<-ctx.Done()
			returned.Store(true)
			return ctx.Err()
		},
	}
	emitter := &mockEmitter{}
	svc := newTestService(t, emitter, factoryWith(client))
	if _, err := svc.Connect(domain.ConnectionConfig{Broker: "tcp://localhost:1883"}); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if !svc.Shutdown(time.Second) {
		t.Fatal("expected Shutdown to drain within timeout")
	}
	if !returned.Load() {
		t.Error("Shutdown returned before the connect goroutine")
	}
	if n := emitter.count(""); n != 0 {
		t.Errorf("expected no events, got %d (%v)", n, emitter.snapshot())
	}
}

// Disconnect の完了後に届いた callback はイベントを発行しないこと。
func TestMQTTService_ConnectedCallback_AfterDisconnect_NoEvent(t *testing.T) {
	rec := &callbackRecorder{}
	emitter := &mockEmitter{}
	svc := newTestService(t, emitter, rec.factory(&mockBrokerClient{}))
	id, _ := svc.Connect(domain.ConnectionConfig{Broker: "tcp://localhost:1883"})
	waitEstablished(t, svc, 1)
	rec.onConnected(0)()

	if err := svc.Disconnect(id); err != nil {
		t.Fatalf("Disconnect: %v", err)
	}
	rec.onConnected(0)()
	rec.onConnectionLost(0)(errors.New("lost"))

	events := emitter.snapshot()
	if last := events[len(events)-1]; last.event != cmn.EventMQTTDisconnected {
		t.Errorf("last event = %s, want %s (%v)", last.event, cmn.EventMQTTDisconnected, events)
	}
	if n := emitter.count(cmn.EventMQTTConnected); n != 1 {
		t.Errorf("mqtt:connected emitted %d times, want 1", n)
	}
	if n := emitter.count(cmn.EventMQTTConnectionLost); n != 0 {
		t.Errorf("mqtt:connection-lost emitted %d times, want 0", n)
	}
}

// Shutdown は実行中の callback の発行を待ってから終端状態に遷移し、以後の callback は捨てること。
func TestMQTTService_Shutdown_WaitsForInFlightCallback(t *testing.T) {
	rec := &callbackRecorder{}
	var disconnects atomic.Int32
	client := &mockBrokerClient{disconnectFn: func(uint) { disconnects.Add(1) }}
	emitter := newBarrierEmitter(cmn.EventMQTTConnected)
	svc := newTestService(t, emitter, rec.factory(client))
	svc.Connect(domain.ConnectionConfig{Broker: "tcp://localhost:1883"})
	waitEstablished(t, svc, 1)

	go rec.onConnected(0)()
	waitForEvent(t, emitter.entered, time.Second, "connected callback did not start emitting")

	result := make(chan bool, 1)
	go func() { result <- svc.Shutdown(time.Second) }()
	assertBlocked(t, result, "Shutdown returned while a callback was emitting")
	if n := disconnects.Load(); n != 0 {
		t.Errorf("client.Disconnect called %d times before the callback finished, want 0", n)
	}
	close(emitter.release)

	if !<-result {
		t.Error("expected Shutdown to drain within timeout")
	}
	before := emitter.count("")
	rec.onConnected(0)()
	rec.onConnectionLost(0)(errors.New("lost"))
	if after := emitter.count(""); after != before {
		t.Errorf("events emitted after Shutdown: %d -> %d", before, after)
	}
}

// subscribeWithHandler は接続して購読し、ハンドラを記録した callbackRecorder を返す。
func subscribeWithHandler(t *testing.T, emitter cmn.Emitter) (*MQTTService, string, *callbackRecorder) {
	t.Helper()
	rec := &callbackRecorder{}
	client := &mockBrokerClient{subscribeFn: rec.subscribeFn}
	svc := newTestService(t, emitter, rec.factory(client))
	id, _ := svc.Connect(domain.ConnectionConfig{Broker: "tcp://localhost:1883"})
	waitEstablished(t, svc, 1)
	if err := svc.Subscribe(id, "sensors/#", 0); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	return svc, id, rec
}

// Disconnect は実行中のメッセージ発行の完了を待ち、復帰後のメッセージは発行しないこと。
func TestMQTTService_Disconnect_WaitsForInFlightMessage(t *testing.T) {
	emitter := newBarrierEmitter(cmn.EventMQTTMessage)
	svc, id, rec := subscribeWithHandler(t, emitter)

	go rec.handler(0)("sensors/temp", []byte("1"), 0, false)
	waitForEvent(t, emitter.entered, time.Second, "message handler did not start emitting")

	result := make(chan error, 1)
	go func() { result <- svc.Disconnect(id) }()
	assertBlocked(t, result, "Disconnect returned while a message was emitting")
	close(emitter.release)

	if err := <-result; err != nil {
		t.Fatalf("Disconnect: %v", err)
	}
	rec.handler(0)("sensors/temp", []byte("2"), 0, false)
	if n := emitter.count(cmn.EventMQTTMessage); n != 1 {
		t.Errorf("mqtt:message emitted %d times, want 1", n)
	}
}

// Shutdown の復帰後はメッセージを発行しないこと。
func TestMQTTService_Shutdown_SuppressesMessages(t *testing.T) {
	emitter := newBarrierEmitter(cmn.EventMQTTMessage)
	svc, _, rec := subscribeWithHandler(t, emitter)

	go rec.handler(0)("sensors/temp", []byte("1"), 0, false)
	waitForEvent(t, emitter.entered, time.Second, "message handler did not start emitting")

	result := make(chan bool, 1)
	go func() { result <- svc.Shutdown(time.Second) }()
	assertBlocked(t, result, "Shutdown returned while a message was emitting")
	close(emitter.release)

	if !<-result {
		t.Error("expected Shutdown to drain within timeout")
	}
	rec.handler(0)("sensors/temp", []byte("2"), 0, false)
	if n := emitter.count(cmn.EventMQTTMessage); n != 1 {
		t.Errorf("mqtt:message emitted %d times, want 1", n)
	}
}

// timeout エラーで失敗した接続は一覧から消え、以後の callback もイベントを出さないこと。
func TestMQTTService_Connect_TimeoutErrorRemovesEntry(t *testing.T) {
	rec := &callbackRecorder{}
	client := &mockBrokerClient{
		connectFn: func(context.Context) error { return errors.New("connection timed out") },
	}
	emitter := &mockEmitter{}
	svc := newTestService(t, emitter, rec.factory(client))
	svc.Connect(domain.ConnectionConfig{Broker: "tcp://localhost:1883"})
	waitConnGoroutines(t, svc)

	if n := emitter.count(cmn.EventMQTTConnectionFailed); n != 1 {
		t.Errorf("mqtt:connection-failed emitted %d times, want 1", n)
	}
	if conns := svc.GetConnections(); len(conns) != 0 {
		t.Errorf("expected 0 connections, got %d", len(conns))
	}
	rec.onConnected(0)()
	if n := emitter.count(cmn.EventMQTTConnected); n != 0 {
		t.Errorf("mqtt:connected emitted %d times, want 0", n)
	}
}

// Disconnect は client 内でブロック中の Publish の完了を待ってから client を切断すること。
func TestMQTTService_Disconnect_WaitsForInFlightPublish(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	var mu sync.Mutex
	var calls []string
	record := func(name string) {
		mu.Lock()
		defer mu.Unlock()
		calls = append(calls, name)
	}
	client := &mockBrokerClient{
		publishFn: func(string, byte, bool, string) error {
			close(entered)
			<-release
			record("publish")
			return nil
		},
		disconnectFn: func(uint) { record("disconnect") },
	}
	svc := newTestService(t, &mockEmitter{}, factoryWith(client))
	id, _ := svc.Connect(domain.ConnectionConfig{Broker: "tcp://localhost:1883"})
	waitEstablished(t, svc, 1)

	published := make(chan error, 1)
	go func() { published <- svc.Publish(id, "t", "1", 0, false) }()
	waitForEvent(t, entered, time.Second, "publish did not start")

	disconnected := make(chan error, 1)
	go func() { disconnected <- svc.Disconnect(id) }()
	waitDetached(t, svc)
	assertBlocked(t, disconnected, "Disconnect returned while a publish was in flight")
	close(release)

	if err := <-published; err != nil {
		t.Errorf("Publish: %v", err)
	}
	if err := <-disconnected; err != nil {
		t.Errorf("Disconnect: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if !slices.Equal(calls, []string{"publish", "disconnect"}) {
		t.Errorf("calls = %v, want [publish disconnect]", calls)
	}
}

// Disconnect は進行中の Connect を打ち切り、接続失敗イベントを出さないこと。
func TestMQTTService_Disconnect_CancelsInFlightConnect(t *testing.T) {
	started := make(chan struct{})
	client := &mockBrokerClient{
		connectFn: func(ctx context.Context) error {
			close(started)
			<-ctx.Done()
			return ctx.Err()
		},
	}
	emitter := &mockEmitter{}
	svc := newTestService(t, emitter, factoryWith(client))
	id, _ := svc.Connect(domain.ConnectionConfig{Broker: "tcp://localhost:1883"})
	waitForEvent(t, started, time.Second, "connect did not start")

	if err := svc.Disconnect(id); err != nil {
		t.Fatalf("Disconnect: %v", err)
	}
	waitConnGoroutines(t, svc)

	if emitter.hasEvent(cmn.EventMQTTConnectionFailed) {
		t.Error("unexpected mqtt:connection-failed after Disconnect")
	}
	if !emitter.hasEvent(cmn.EventMQTTDisconnected) {
		t.Error("expected mqtt:disconnected")
	}
}

// Disconnect の後に成功した接続は破棄され、接続済みイベントを出さないこと。
func TestMQTTService_Connect_LateSuccessAfterDisconnect_Discards(t *testing.T) {
	rec := &callbackRecorder{}
	started := make(chan struct{})
	proceed := make(chan struct{})
	var disconnects atomic.Int32
	client := &mockBrokerClient{
		// ctx を無視して遅れて成功する (paho と同じく onConnected を先に呼ぶ)。
		connectFn: func(context.Context) error {
			close(started)
			<-proceed
			rec.onConnected(0)()
			return nil
		},
		disconnectFn: func(uint) { disconnects.Add(1) },
	}
	emitter := &mockEmitter{}
	svc := newTestService(t, emitter, rec.factory(client))
	id, _ := svc.Connect(domain.ConnectionConfig{Broker: "tcp://localhost:1883"})
	waitForEvent(t, started, time.Second, "connect did not start")

	if err := svc.Disconnect(id); err != nil {
		t.Fatalf("Disconnect: %v", err)
	}
	close(proceed)
	waitConnGoroutines(t, svc)

	if n := disconnects.Load(); n != 2 {
		t.Errorf("client.Disconnect called %d times, want 2 (Disconnect + discard)", n)
	}
	if emitter.hasEvent(cmn.EventMQTTConnected) {
		t.Error("unexpected mqtt:connected after Disconnect")
	}
}

func TestMQTTService_Connect_AfterShutdown_Rejected(t *testing.T) {
	var connects atomic.Int32
	client := &mockBrokerClient{
		connectFn: func(context.Context) error { connects.Add(1); return nil },
	}
	svc := newTestService(t, &mockEmitter{}, factoryWith(client))
	if !svc.Shutdown(time.Second) {
		t.Fatal("expected Shutdown to drain within timeout")
	}

	if _, err := svc.Connect(domain.ConnectionConfig{Broker: "tcp://localhost:1883"}); !errors.Is(err, errShuttingDown) {
		t.Errorf("err = %v, want errShuttingDown", err)
	}
	if conns := svc.GetConnections(); len(conns) != 0 {
		t.Errorf("expected 0 connections, got %d", len(conns))
	}
	waitConnGoroutines(t, svc)
	if n := connects.Load(); n != 0 {
		t.Errorf("client.Connect called %d times, want 0", n)
	}
}

// ctx を無視して止まり続ける Connect では false を返し、その後に成功してもイベントを出さないこと。
func TestMQTTService_Shutdown_TimesOutWhenConnectHangs(t *testing.T) {
	rec := &callbackRecorder{}
	started := make(chan struct{})
	proceed := make(chan struct{})
	client := &mockBrokerClient{
		connectFn: func(context.Context) error {
			close(started)
			<-proceed
			rec.onConnected(0)()
			return nil
		},
	}
	emitter := &mockEmitter{}
	svc := newTestService(t, emitter, rec.factory(client))
	svc.Connect(domain.ConnectionConfig{Broker: "tcp://localhost:1883"})
	waitForEvent(t, started, time.Second, "connect did not start")

	if svc.Shutdown(50 * time.Millisecond) {
		t.Error("expected Shutdown to time out while Connect hangs")
	}
	close(proceed)
	waitConnGoroutines(t, svc)

	if n := emitter.count(""); n != 0 {
		t.Errorf("expected no events, got %d (%v)", n, emitter.snapshot())
	}
}

// -race 下で全操作・全 callback を並行に実行し、データ競合が無く、切断後・終了後にイベントが出ないこと。
func TestMQTTService_ConcurrentOperations(t *testing.T) {
	const connCount = 4
	rec := &callbackRecorder{}
	client := &mockBrokerClient{subscribeFn: rec.subscribeFn}
	emitter := &mockEmitter{}
	svc := newTestService(t, emitter, rec.factory(client))

	ids := make([]string, connCount)
	for i := range ids {
		ids[i], _ = svc.Connect(domain.ConnectionConfig{Broker: "tcp://localhost:1883"})
	}
	waitEstablished(t, svc, connCount)
	for _, id := range ids {
		if err := svc.Subscribe(id, "sensors/#", 0); err != nil {
			t.Fatalf("Subscribe: %v", err)
		}
	}

	var wg sync.WaitGroup
	for i, id := range ids {
		wg.Go(func() {
			for range 20 {
				_ = svc.Publish(id, "sensors/temp", "1", 0, false)
				_ = svc.Subscribe(id, "sensors/humidity", 1)
				_ = svc.Unsubscribe(id, "sensors/humidity")
				_ = svc.GetConnections()
			}
		})
		wg.Go(func() {
			for range 20 {
				rec.fireAll()
			}
		})
		// 半分の接続は操作と並行に切断する。
		if i%2 == 0 {
			wg.Go(func() { _ = svc.Disconnect(id) })
		}
	}
	shutdownDone := make(chan bool, 1)
	wg.Go(func() {
		time.Sleep(5 * time.Millisecond)
		shutdownDone <- svc.Shutdown(time.Second)
	})
	wg.Wait()

	if !<-shutdownDone {
		t.Error("expected Shutdown to drain within timeout")
	}
	before := emitter.count("")
	rec.fireAll()
	if after := emitter.count(""); after != before {
		t.Errorf("events emitted after Shutdown: %d -> %d", before, after)
	}

	// mqtt:disconnected の後に、その接続のイベントが続かないこと。
	disconnected := make(map[string]bool)
	for _, ev := range emitter.snapshot() {
		id := eventConnID(ev)
		if disconnected[id] {
			t.Errorf("event %s for %s emitted after mqtt:disconnected", ev.event, id)
		}
		if ev.event == cmn.EventMQTTDisconnected {
			disconnected[id] = true
		}
	}
}
