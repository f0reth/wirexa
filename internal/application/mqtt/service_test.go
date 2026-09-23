package mqttapp

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"slices"
	"sync"
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
