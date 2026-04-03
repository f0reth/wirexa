package mqttapp

import (
	"errors"
	"sync"
	"testing"
	"time"

	cmn "github.com/f0reth/Wirexa/internal/domain"
	domain "github.com/f0reth/Wirexa/internal/domain/mqtt"
)

// mockEmitter は domain.Emitter のインメモリモック。
type mockEmitter struct {
	mu     sync.Mutex
	events []emittedEvent
}

type emittedEvent struct {
	event string
	data  any
}

func (e *mockEmitter) Emit(event string, data any) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.events = append(e.events, emittedEvent{event, data})
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
	connectFn     func() error
	disconnectFn  func(quiesce uint)
	publishFn     func(topic string, qos byte, retained bool, payload string) error
	subscribeFn   func(topic string, qos byte, handler domain.MessageHandler) error
	unsubscribeFn func(topics ...string) error
	isConnectedFn func() bool
}

func (m *mockBrokerClient) Connect() error {
	if m.connectFn != nil {
		return m.connectFn()
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

// ------- Connect -------

func TestMqttService_Connect_EmptyBroker(t *testing.T) {
	svc := NewMqttService(&mockEmitter{}, factoryWith(&mockBrokerClient{}), cmn.NoopLogger{})
	_, err := svc.Connect(domain.ConnectionConfig{Broker: ""})
	if err == nil {
		t.Fatal("expected error for empty broker, got nil")
	}
	var ve *cmn.ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("expected ValidationError, got %T", err)
	}
}

func TestMqttService_Connect_ReturnsNonEmptyID(t *testing.T) {
	done := make(chan struct{})
	client := &mockBrokerClient{
		connectFn: func() error { close(done); return nil },
	}
	svc := NewMqttService(&mockEmitter{}, factoryWith(client), cmn.NoopLogger{})

	id, err := svc.Connect(domain.ConnectionConfig{Broker: "tcp://localhost:1883"})
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if id == "" {
		t.Error("expected non-empty connection ID")
	}
	waitForEvent(t, done, time.Second, "timeout waiting for connect goroutine")
}

func TestMqttService_Connect_AutoGeneratesClientID(t *testing.T) {
	var capturedConfig domain.ConnectionConfig
	factory := func(cfg domain.ConnectionConfig, _ func(), _ func(error)) domain.BrokerClient {
		capturedConfig = cfg
		return &mockBrokerClient{}
	}
	svc := NewMqttService(&mockEmitter{}, factory, cmn.NoopLogger{})
	svc.Connect(domain.ConnectionConfig{Broker: "tcp://localhost:1883", ClientID: ""})

	time.Sleep(10 * time.Millisecond) // allow factory call to complete
	if capturedConfig.ClientID == "" {
		t.Error("expected auto-generated ClientID, got empty")
	}
}

func TestMqttService_Connect_UsesProvidedClientID(t *testing.T) {
	var capturedConfig domain.ConnectionConfig
	factory := func(cfg domain.ConnectionConfig, _ func(), _ func(error)) domain.BrokerClient {
		capturedConfig = cfg
		return &mockBrokerClient{}
	}
	svc := NewMqttService(&mockEmitter{}, factory, cmn.NoopLogger{})
	svc.Connect(domain.ConnectionConfig{Broker: "tcp://localhost:1883", ClientID: "my-client"})

	time.Sleep(10 * time.Millisecond)
	if capturedConfig.ClientID != "my-client" {
		t.Errorf("ClientID = %q, want %q", capturedConfig.ClientID, "my-client")
	}
}

func TestMqttService_Connect_FailureRemovesConnection(t *testing.T) {
	failedCh := make(chan struct{})
	emitter2 := &mockEmitterWithChan{mockEmitter: mockEmitter{}, ch: failedCh, targetEvent: eventConnectionFailed}

	client := &mockBrokerClient{
		connectFn: func() error { return errors.New("connection refused") },
	}
	svc := NewMqttService(emitter2, factoryWith(client), cmn.NoopLogger{})
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
	mockEmitter
	ch          chan struct{}
	targetEvent string
	once        sync.Once
}

func (e *mockEmitterWithChan) Emit(event string, data any) {
	e.mockEmitter.Emit(event, data)
	if event == e.targetEvent {
		e.once.Do(func() { close(e.ch) })
	}
}

// ------- Disconnect -------

func TestMqttService_Disconnect_NotFound(t *testing.T) {
	svc := NewMqttService(&mockEmitter{}, factoryWith(&mockBrokerClient{}), cmn.NoopLogger{})
	err := svc.Disconnect("nonexistent")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var nfe *cmn.NotFoundError
	if !errors.As(err, &nfe) {
		t.Errorf("expected NotFoundError, got %T", err)
	}
}

func TestMqttService_Disconnect_Success(t *testing.T) {
	done := make(chan struct{})
	client := &mockBrokerClient{
		connectFn: func() error { close(done); return nil },
	}
	emitter := &mockEmitterWithChan{
		ch:          make(chan struct{}),
		targetEvent: eventDisconnected,
	}
	svc := NewMqttService(emitter, factoryWith(client), cmn.NoopLogger{})

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

func TestMqttService_Publish_EmptyTopic(t *testing.T) {
	svc := NewMqttService(&mockEmitter{}, factoryWith(&mockBrokerClient{}), cmn.NoopLogger{})
	err := svc.Publish("connid", "", "payload", 0, false)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var ve *cmn.ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("expected ValidationError, got %T", err)
	}
}

func TestMqttService_Publish_InvalidQoS(t *testing.T) {
	svc := NewMqttService(&mockEmitter{}, factoryWith(&mockBrokerClient{}), cmn.NoopLogger{})
	err := svc.Publish("connid", "topic", "payload", 3, false)
	if err == nil {
		t.Fatal("expected error for qos=3, got nil")
	}
	var ve *cmn.ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("expected ValidationError, got %T", err)
	}
}

func TestMqttService_Publish_ConnectionNotFound(t *testing.T) {
	svc := NewMqttService(&mockEmitter{}, factoryWith(&mockBrokerClient{}), cmn.NoopLogger{})
	err := svc.Publish("nonexistent", "topic", "payload", 0, false)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var nfe *cmn.NotFoundError
	if !errors.As(err, &nfe) {
		t.Errorf("expected NotFoundError, got %T", err)
	}
}

func TestMqttService_Publish_Success(t *testing.T) {
	done := make(chan struct{})
	var publishedTopic, publishedPayload string
	var publishedQoS byte
	client := &mockBrokerClient{
		connectFn: func() error { close(done); return nil },
		publishFn: func(topic string, qos byte, _ bool, payload string) error {
			publishedTopic = topic
			publishedQoS = qos
			publishedPayload = payload
			return nil
		},
	}
	svc := NewMqttService(&mockEmitter{}, factoryWith(client), cmn.NoopLogger{})
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

func TestMqttService_Publish_ValidQoSValues(t *testing.T) {
	done := make(chan struct{})
	var once sync.Once
	client := &mockBrokerClient{
		connectFn: func() error { once.Do(func() { close(done) }); return nil },
	}
	svc := NewMqttService(&mockEmitter{}, factoryWith(client), cmn.NoopLogger{})
	id, _ := svc.Connect(domain.ConnectionConfig{Broker: "tcp://localhost:1883"})
	waitForEvent(t, done, time.Second, "connect goroutine timeout")

	for _, qos := range []byte{0, 1, 2} {
		if err := svc.Publish(id, "topic", "msg", qos, false); err != nil {
			t.Errorf("Publish with qos=%d: %v", qos, err)
		}
	}
}

// ------- Subscribe -------

func TestMqttService_Subscribe_EmptyTopic(t *testing.T) {
	svc := NewMqttService(&mockEmitter{}, factoryWith(&mockBrokerClient{}), cmn.NoopLogger{})
	err := svc.Subscribe("connid", "", 0)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestMqttService_Subscribe_InvalidQoS(t *testing.T) {
	svc := NewMqttService(&mockEmitter{}, factoryWith(&mockBrokerClient{}), cmn.NoopLogger{})
	err := svc.Subscribe("connid", "topic", 3)
	if err == nil {
		t.Fatal("expected error for qos=3, got nil")
	}
}

func TestMqttService_Subscribe_ConnectionNotFound(t *testing.T) {
	svc := NewMqttService(&mockEmitter{}, factoryWith(&mockBrokerClient{}), cmn.NoopLogger{})
	err := svc.Subscribe("nonexistent", "topic", 0)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var nfe *cmn.NotFoundError
	if !errors.As(err, &nfe) {
		t.Errorf("expected NotFoundError, got %T", err)
	}
}

func TestMqttService_Subscribe_MessageHandlerEmitsEvent(t *testing.T) {
	done := make(chan struct{})
	var capturedHandler domain.MessageHandler
	client := &mockBrokerClient{
		connectFn: func() error { close(done); return nil },
		subscribeFn: func(_ string, _ byte, handler domain.MessageHandler) error {
			capturedHandler = handler
			return nil
		},
	}
	msgCh := make(chan struct{})
	emitter := &mockEmitterWithChan{
		ch:          msgCh,
		targetEvent: eventMessage,
	}
	svc := NewMqttService(emitter, factoryWith(client), cmn.NoopLogger{})
	id, _ := svc.Connect(domain.ConnectionConfig{Broker: "tcp://localhost:1883"})
	waitForEvent(t, done, time.Second, "connect goroutine timeout")

	if err := svc.Subscribe(id, "sensors/#", 0); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	if capturedHandler == nil {
		t.Fatal("handler not set")
	}
	capturedHandler("sensors/temp", "25.5", 0, false)
	waitForEvent(t, msgCh, time.Second, "message event timeout")
	if !emitter.hasEvent(eventMessage) {
		t.Error("expected mqtt:message event")
	}
}

// ------- Unsubscribe -------

func TestMqttService_Unsubscribe_EmptyTopic(t *testing.T) {
	svc := NewMqttService(&mockEmitter{}, factoryWith(&mockBrokerClient{}), cmn.NoopLogger{})
	err := svc.Unsubscribe("connid", "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestMqttService_Unsubscribe_ConnectionNotFound(t *testing.T) {
	svc := NewMqttService(&mockEmitter{}, factoryWith(&mockBrokerClient{}), cmn.NoopLogger{})
	err := svc.Unsubscribe("nonexistent", "topic")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var nfe *cmn.NotFoundError
	if !errors.As(err, &nfe) {
		t.Errorf("expected NotFoundError, got %T", err)
	}
}

func TestMqttService_Unsubscribe_Success(t *testing.T) {
	done := make(chan struct{})
	var unsubscribedTopics []string
	client := &mockBrokerClient{
		connectFn: func() error { close(done); return nil },
		unsubscribeFn: func(topics ...string) error {
			unsubscribedTopics = topics
			return nil
		},
	}
	svc := NewMqttService(&mockEmitter{}, factoryWith(client), cmn.NoopLogger{})
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

func TestMqttService_GetConnections_Empty(t *testing.T) {
	svc := NewMqttService(&mockEmitter{}, factoryWith(&mockBrokerClient{}), cmn.NoopLogger{})
	conns := svc.GetConnections()
	if len(conns) != 0 {
		t.Errorf("expected 0 connections, got %d", len(conns))
	}
}

func TestMqttService_GetConnections_ReflectsIsConnected(t *testing.T) {
	done := make(chan struct{})
	connected := true
	client := &mockBrokerClient{
		connectFn:     func() error { close(done); return nil },
		isConnectedFn: func() bool { return connected },
	}
	svc := NewMqttService(&mockEmitter{}, factoryWith(client), cmn.NoopLogger{})
	svc.Connect(domain.ConnectionConfig{Broker: "tcp://localhost:1883", Name: "TestConn"})
	waitForEvent(t, done, time.Second, "connect goroutine timeout")

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

// ------- Shutdown -------

func TestMqttService_Shutdown_DisconnectsAll(t *testing.T) {
	done := make(chan struct{})
	disconnectCount := 0
	var mu sync.Mutex
	client := &mockBrokerClient{
		connectFn: func() error { close(done); return nil },
		disconnectFn: func(_ uint) {
			mu.Lock()
			disconnectCount++
			mu.Unlock()
		},
	}
	svc := NewMqttService(&mockEmitter{}, factoryWith(client), cmn.NoopLogger{})
	svc.Connect(domain.ConnectionConfig{Broker: "tcp://localhost:1883"})
	waitForEvent(t, done, time.Second, "connect goroutine timeout")

	svc.Shutdown()

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

func TestMqttService_Shutdown_NoConnections(_ *testing.T) {
	svc := NewMqttService(&mockEmitter{}, factoryWith(&mockBrokerClient{}), cmn.NoopLogger{})
	// Should not panic or block
	svc.Shutdown()
}
