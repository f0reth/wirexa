package mqttapp

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
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
	unsubscribeFn func(topic string) error
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

func (m *mockBrokerClient) Unsubscribe(topic string) error {
	if m.unsubscribeFn != nil {
		return m.unsubscribeFn(topic)
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

// UseTLS で TLS にできないスキームの Broker は、client を作る前に拒否する
// (そのまま渡すと平文で繋がるか、接続失敗のイベントでしか分からない)。
func TestMQTTService_Connect_UseTLS_RejectsUnsupportedScheme(t *testing.T) {
	var factoryCalls atomic.Int32
	factory := func(domain.ConnectionConfig, func(), func(error)) domain.BrokerClient {
		factoryCalls.Add(1)
		return &mockBrokerClient{}
	}
	svc := newTestService(t, &mockEmitter{}, factory)

	_, err := svc.Connect(domain.ConnectionConfig{Broker: "http://localhost:1883", UseTLS: true})

	if _, ok := errors.AsType[*cmn.ValidationError](err); !ok {
		t.Fatalf("err = %v, want ValidationError", err)
	}
	if n := factoryCalls.Load(); n != 0 {
		t.Errorf("factory called %d times, want 0", n)
	}
	if conns := svc.GetConnections(); len(conns) != 0 {
		t.Errorf("connections = %v, want none", conns)
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

// ワイルドカードを含むトピックへの publish は、クライアントへ渡す前に拒否する。
// ブローカーはプロトコル違反として接続ごと切断するため。
func TestMQTTService_InvalidTopics_RejectedBeforeClient(t *testing.T) {
	var calls atomic.Int32
	client := &mockBrokerClient{
		publishFn: func(string, byte, bool, string) error { calls.Add(1); return nil },
		subscribeFn: func(string, byte, domain.MessageHandler) error {
			calls.Add(1)
			return nil
		},
		unsubscribeFn: func(string) error { calls.Add(1); return nil },
	}
	svc := newTestService(t, &mockEmitter{}, factoryWith(client))
	id, err := svc.Connect(domain.ConnectionConfig{Broker: "tcp://localhost:1883"})
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	waitEstablished(t, svc, 1)

	ops := map[string]func() error{
		"Publish a/+":      func() error { return svc.Publish(id, "a/+", "x", 1, false) },
		"Publish a/#":      func() error { return svc.Publish(id, "a/#", "x", 0, false) },
		"Subscribe a/#/b":  func() error { return svc.Subscribe(id, "a/#/b", 0) },
		"Subscribe a/b#":   func() error { return svc.Subscribe(id, "a/b#", 0) },
		"Subscribe a+/b":   func() error { return svc.Subscribe(id, "a+/b", 0) },
		"Unsubscribe a+/b": func() error { return svc.Unsubscribe(id, "a+/b") },
	}
	for name, op := range ops {
		if _, ok := errors.AsType[*cmn.ValidationError](op()); !ok {
			t.Errorf("%s: expected ValidationError", name)
		}
	}
	if got := calls.Load(); got != 0 {
		t.Errorf("client was called %d times for invalid topics", got)
	}
	if subs := svc.GetConnections()[0].Subscriptions; len(subs) != 0 {
		t.Errorf("subscriptions = %v, want none", subs)
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
		unsubscribeFn: func(topic string) error {
			unsubscribedTopics = append(unsubscribedTopics, topic)
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

// GetConnections は接続を作成順、購読を購読した順で返す (map の反復順に依らない)。
// フロントは返った順のまま購読一覧を描くので、順序が不定だとリロードのたびに並びが変わる。
func TestMQTTService_GetConnections_StableOrder(t *testing.T) {
	svc := newTestService(t, &mockEmitter{}, factoryWith(&mockBrokerClient{}))
	names := []string{"e", "a", "d", "b", "c"}
	ids := make([]string, 0, len(names))
	for _, name := range names {
		id, err := svc.Connect(domain.ConnectionConfig{Broker: "tcp://localhost:1883", Name: name})
		if err != nil {
			t.Fatalf("Connect(%s): %v", name, err)
		}
		ids = append(ids, id)
	}
	waitEstablished(t, svc, len(names))

	const target = 2
	subscribe := func(topic string, qos byte) {
		t.Helper()
		if err := svc.Subscribe(ids[target], topic, qos); err != nil {
			t.Fatalf("Subscribe(%s): %v", topic, err)
		}
	}
	// 名前順とも逆順とも異なる順で購読する。
	topics := []string{"h/1", "b/1", "g/1", "a/1", "f/1", "c/1", "e/1", "d/1"}
	for _, topic := range topics {
		subscribe(topic, 0)
	}

	assertOrder := func(want []domain.SubscriptionInfo) {
		t.Helper()
		for range 50 {
			conns := svc.GetConnections()
			gotIDs := make([]string, 0, len(conns))
			for i := range conns {
				gotIDs = append(gotIDs, conns[i].ID)
			}
			if !slices.Equal(gotIDs, ids) {
				t.Fatalf("connection order = %v, want creation order %v", gotIDs, ids)
			}
			if got := conns[target].Subscriptions; !slices.Equal(got, want) {
				t.Fatalf("subscription order = %v, want %v", got, want)
			}
		}
	}
	want := make([]domain.SubscriptionInfo, 0, len(topics))
	for _, topic := range topics {
		want = append(want, domain.SubscriptionInfo{Topic: topic})
	}
	assertOrder(want)

	// 途中の購読を解除して別の購読を足すと、残りの順序はそのままで末尾に足される。
	if err := svc.Unsubscribe(ids[target], "g/1"); err != nil {
		t.Fatalf("Unsubscribe: %v", err)
	}
	subscribe("z/1", 1)
	// 購読中のトピックの QoS を変えても位置は変わらない。
	subscribe("a/1", 2)
	assertOrder([]domain.SubscriptionInfo{
		{Topic: "h/1"},
		{Topic: "b/1"},
		{Topic: "a/1", QoS: 2},
		{Topic: "f/1"},
		{Topic: "c/1"},
		{Topic: "e/1"},
		{Topic: "d/1"},
		{Topic: "z/1", QoS: 1},
	})
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
		unsubscribeFn: func(string) error { return wantErr },
	}
	svc := newTestService(t, &mockEmitter{}, factoryWith(client))
	id, _ := svc.Connect(domain.ConnectionConfig{Broker: "tcp://localhost:1883"})
	waitForEvent(t, done, time.Second, "connect goroutine timeout")

	if err := svc.Subscribe(id, "topic", 0); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	err := svc.Unsubscribe(id, "topic")
	if !errors.Is(err, wantErr) {
		t.Errorf("expected unsubscribe error, got %v", err)
	}
	// 解除に失敗した購読は残す。
	if subs := svc.GetConnections()[0].Subscriptions; !slices.Equal(subs, []domain.SubscriptionInfo{{Topic: "topic"}}) {
		t.Errorf("subscriptions = %v, want topic kept", subs)
	}
}

// ブローカーの応答を確認できなかった Unsubscribe はエラーを返すが、購読は外す。
// client は振り分け先を外しているので、残すと購読中と表示されたままメッセージが届かない。
func TestMQTTService_Unsubscribe_AckTimeout_RemovesSubscription(t *testing.T) {
	client := &mockBrokerClient{
		unsubscribeFn: func(string) error { return fmt.Errorf("unsubscribe: %w", domain.ErrAckTimeout) },
	}
	svc := newTestService(t, &mockEmitter{}, factoryWith(client))
	id, _ := svc.Connect(domain.ConnectionConfig{Broker: "tcp://localhost:1883"})
	waitEstablished(t, svc, 1)
	for _, topic := range []string{"a/#", "b/#"} {
		if err := svc.Subscribe(id, topic, 0); err != nil {
			t.Fatalf("Subscribe(%s): %v", topic, err)
		}
	}

	err := svc.Unsubscribe(id, "a/#")

	if !errors.Is(err, domain.ErrAckTimeout) {
		t.Errorf("err = %v, want ErrAckTimeout", err)
	}
	if subs := svc.GetConnections()[0].Subscriptions; !slices.Equal(subs, []domain.SubscriptionInfo{{Topic: "b/#"}}) {
		t.Errorf("subscriptions = %v, want only b/#", subs)
	}
}

// ブローカーの応答を確認できなかった Subscribe は、購読として扱わない
// (client も振り分け先に残さないので、表示と受信が一致する)。
func TestMQTTService_Subscribe_AckTimeout_NotTracked(t *testing.T) {
	client := &mockBrokerClient{
		subscribeFn: func(string, byte, domain.MessageHandler) error {
			return fmt.Errorf("subscribe: %w", domain.ErrAckTimeout)
		},
	}
	svc := newTestService(t, &mockEmitter{}, factoryWith(client))
	id, _ := svc.Connect(domain.ConnectionConfig{Broker: "tcp://localhost:1883"})
	waitEstablished(t, svc, 1)

	err := svc.Subscribe(id, "a/#", 0)

	if !errors.Is(err, domain.ErrAckTimeout) {
		t.Errorf("err = %v, want ErrAckTimeout", err)
	}
	if subs := svc.GetConnections()[0].Subscriptions; len(subs) != 0 {
		t.Errorf("subscriptions = %v, want none", subs)
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
	case domain.ConnectionEvent:
		return data.ConnectionID
	case domain.ConnectionErrorEvent:
		return data.ConnectionID
	case domain.MQTTMessage:
		return data.ConnectionID
	case domain.ScannedTopic:
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
				_ = svc.StartTopicScan(id)
				_ = svc.StopTopicScan(id)
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

// ------- connection-lost / connection-failed のイベントの中身 -------

// 接続中に接続が切れたら、connectionId と error を付けた connection-lost を 1 回だけ出す。
func TestMQTTService_ConnectionLost_EmitsEventWithError(t *testing.T) {
	rec := &callbackRecorder{}
	emitter := &mockEmitter{}
	svc := newTestService(t, emitter, rec.factory(&mockBrokerClient{}))
	id, err := svc.Connect(domain.ConnectionConfig{Broker: "tcp://localhost:1883"})
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	waitEstablished(t, svc, 1)

	rec.onConnectionLost(0)(errors.New("broker went away"))

	if n := emitter.count(cmn.EventMQTTConnectionLost); n != 1 {
		t.Fatalf("mqtt:connection-lost emitted %d times, want 1", n)
	}
	for _, ev := range emitter.snapshot() {
		if ev.event != cmn.EventMQTTConnectionLost {
			continue
		}
		want := domain.ConnectionErrorEvent{ConnectionID: id, Error: "broker went away"}
		if ev.data != want {
			t.Errorf("event data = %#v, want %#v", ev.data, want)
		}
	}
	// paho が自動再接続するので、接続は追跡したまま残す。
	if conns := svc.GetConnections(); len(conns) != 1 {
		t.Errorf("connections = %d, want 1 (kept for auto-reconnect)", len(conns))
	}
}

// 接続に失敗したら、connectionId と error を付けた connection-failed を出す。
func TestMQTTService_Connect_FailureEventCarriesError(t *testing.T) {
	failedCh := make(chan struct{})
	emitter := &mockEmitterWithChan{ch: failedCh, targetEvent: cmn.EventMQTTConnectionFailed}
	client := &mockBrokerClient{
		connectFn: func(context.Context) error { return errors.New("connection refused") },
	}
	svc := newTestService(t, emitter, factoryWith(client))
	id, err := svc.Connect(domain.ConnectionConfig{Broker: "tcp://localhost:1883"})
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	waitForEvent(t, failedCh, time.Second, "timeout waiting for connection-failed event")

	events := emitter.snapshot()
	if len(events) != 1 {
		t.Fatalf("events = %v, want only connection-failed", events)
	}
	want := domain.ConnectionErrorEvent{ConnectionID: id, Error: "connection refused"}
	if events[0].data != want {
		t.Errorf("event data = %#v, want %#v", events[0].data, want)
	}
}

// ------- 再接続後の購読の張り直し -------

// subscribeCall は Subscribe に渡された引数を記録する。
type subscribeCall struct {
	topic string
	qos   byte
}

// newResubscribeService は Subscribe の呼び出しを記録する接続を 1 本張り、topics を順に購読する。
// subscribeErr を差し替えると、それ以降の Subscribe の結果を変えられる。
func newResubscribeService(t *testing.T, emitter cmn.Emitter, topics []subscribeCall) (svc *MQTTService, id string, rec *callbackRecorder, calls func() []subscribeCall, subscribeErr *atomic.Pointer[error]) {
	t.Helper()
	rec = &callbackRecorder{}
	var mu sync.Mutex
	var recorded []subscribeCall
	subscribeErr = &atomic.Pointer[error]{}
	client := &mockBrokerClient{subscribeFn: func(topic string, qos byte, handler domain.MessageHandler) error {
		if err := subscribeErr.Load(); err != nil {
			return *err
		}
		mu.Lock()
		recorded = append(recorded, subscribeCall{topic: topic, qos: qos})
		mu.Unlock()
		return rec.subscribeFn(topic, qos, handler)
	}}
	svc = newTestService(t, emitter, rec.factory(client))
	id, err := svc.Connect(domain.ConnectionConfig{Broker: "tcp://localhost:1883"})
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	waitEstablished(t, svc, 1)
	for _, sub := range topics {
		if err := svc.Subscribe(id, sub.topic, sub.qos); err != nil {
			t.Fatalf("Subscribe(%s): %v", sub.topic, err)
		}
	}
	calls = func() []subscribeCall {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(recorded)
	}
	return svc, id, rec, calls, subscribeErr
}

// 再接続 (2 回目以降の onConnected) では、同じ QoS で全購読を張り直し、メッセージが届く。
func TestMQTTService_Reconnect_Resubscribes(t *testing.T) {
	emitter := &mockEmitter{}
	svc, _, rec, calls, _ := newResubscribeService(t, emitter, []subscribeCall{{"a/#", 1}, {"b/+", 2}})

	rec.onConnectionLost(0)(errors.New("lost"))
	rec.onConnected(0)()

	got := calls()[2:]
	if want := []subscribeCall{{"a/#", 1}, {"b/+", 2}}; !slices.Equal(got, want) {
		t.Fatalf("resubscribed = %v, want %v", got, want)
	}
	// 張り直しで登録したハンドラもメッセージを発行する。
	rec.handler(len(calls())-1)("a/x", []byte("1"), 1, false)
	if n := emitter.count(cmn.EventMQTTMessage); n != 1 {
		t.Errorf("mqtt:message emitted %d times, want 1", n)
	}
	if subs := svc.GetConnections()[0].Subscriptions; len(subs) != 2 {
		t.Errorf("subscriptions = %v, want both kept", subs)
	}
}

// 再接続では購読した順に張り直す (map の反復順に依らない)。
func TestMQTTService_Reconnect_ResubscribesInOrder(t *testing.T) {
	topics := []subscribeCall{{"h/1", 0}, {"b/1", 1}, {"g/1", 2}, {"a/1", 0}, {"f/1", 1}, {"c/1", 2}, {"e/1", 0}, {"d/1", 1}}
	_, _, rec, calls, _ := newResubscribeService(t, &mockEmitter{}, topics)

	rec.onConnected(0)()

	if got := calls()[len(topics):]; !slices.Equal(got, topics) {
		t.Errorf("resubscribed = %v, want the subscription order %v", got, topics)
	}
}

// 購読が無ければ onConnected は Subscribe を呼ばない (初回接続)。
func TestMQTTService_Connected_WithoutSubscriptions_DoesNotSubscribe(t *testing.T) {
	_, _, rec, calls, _ := newResubscribeService(t, &mockEmitter{}, nil)
	rec.onConnected(0)()
	if got := calls(); len(got) != 0 {
		t.Errorf("Subscribe called with %v, want no calls", got)
	}
}

// 張り直しでブローカーに拒否された購読と、接続が開いたまま失敗した購読は、表示から外して
// client からも解除する。接続が切れて失敗した購読は残して次の再接続に任せる。
func TestMQTTService_Reconnect_ResubscribeFailures(t *testing.T) {
	ackTimeout := fmt.Errorf("subscribe: %w", domain.ErrAckTimeout)
	tests := []struct {
		subscribeErr     error
		unsubscribeErr   error
		name             string
		wantSubs         []string
		wantUnsubscribed []string
		// lost は、張り直しの失敗時に接続が切れている (IsConnected が false) ことを表す。
		lost bool
	}{
		{
			name:             "rejected",
			subscribeErr:     domain.ErrSubscriptionRejected,
			wantSubs:         []string{"b/#"},
			wantUnsubscribed: []string{"a/#"},
		},
		{
			// 解除に失敗しても (直後にまた切断した等)、表示からは外す。
			name:             "rejected and unsubscribe fails",
			subscribeErr:     domain.ErrSubscriptionRejected,
			unsubscribeErr:   errors.New("not connected"),
			wantSubs:         []string{"b/#"},
			wantUnsubscribed: []string{"a/#"},
		},
		{
			// 接続が切れた失敗は、次の再接続で張り直す。
			name:         "connection lost",
			subscribeErr: errors.New("not connected"),
			lost:         true,
			wantSubs:     []string{"a/#", "b/#"},
		},
		{
			// 接続が開いたままの失敗は次の再接続が来ないので、残すと購読中と表示されたまま届かない。
			name:             "fails while connected",
			subscribeErr:     ackTimeout,
			wantSubs:         []string{"b/#"},
			wantUnsubscribed: []string{"a/#"},
		},
		{
			name:             "fails while connected and unsubscribe fails",
			subscribeErr:     ackTimeout,
			unsubscribeErr:   ackTimeout,
			wantSubs:         []string{"b/#"},
			wantUnsubscribed: []string{"a/#"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := &callbackRecorder{}
			var failing atomic.Bool
			var mu sync.Mutex
			var unsubscribed []string
			client := &mockBrokerClient{
				// 張り直しでは a/# だけ失敗させる。
				subscribeFn: func(topic string, _ byte, _ domain.MessageHandler) error {
					if failing.Load() && topic == "a/#" {
						return tc.subscribeErr
					}
					return nil
				},
				unsubscribeFn: func(topic string) error {
					mu.Lock()
					defer mu.Unlock()
					unsubscribed = append(unsubscribed, topic)
					return tc.unsubscribeErr
				},
				isConnectedFn: func() bool { return !failing.Load() || !tc.lost },
			}
			svc := newTestService(t, &mockEmitter{}, rec.factory(client))
			id, err := svc.Connect(domain.ConnectionConfig{Broker: "tcp://localhost:1883"})
			if err != nil {
				t.Fatalf("Connect: %v", err)
			}
			waitEstablished(t, svc, 1)
			for _, topic := range []string{"a/#", "b/#"} {
				if err := svc.Subscribe(id, topic, 0); err != nil {
					t.Fatalf("Subscribe(%s): %v", topic, err)
				}
			}

			failing.Store(true)
			rec.onConnected(0)()

			var subs []string
			for _, s := range svc.GetConnections()[0].Subscriptions {
				subs = append(subs, s.Topic)
			}
			slices.Sort(subs)
			if !slices.Equal(subs, tc.wantSubs) {
				t.Errorf("subscriptions = %v, want %v", subs, tc.wantSubs)
			}
			mu.Lock()
			defer mu.Unlock()
			if !slices.Equal(unsubscribed, tc.wantUnsubscribed) {
				t.Errorf("client.Unsubscribe called with %v, want %v", unsubscribed, tc.wantUnsubscribed)
			}
		})
	}
}

// ------- 接続確立前の購読 -------

// pendingConn は Connect を止めて stateConnecting に留めた接続と、client 操作の記録を持つ。
type pendingConn struct {
	svc *MQTTService
	rec *callbackRecorder
	// release に送った値を client.Connect が返す。送るまでは ctx の打ち切りでしか復帰しない。
	release chan error
	// subscribeErrs はトピックごとに client.Subscribe が返すエラー (mu で保護)。無ければ成功する。
	subscribeErrs map[string]error
	id            string
	subscribes    []subscribeCall
	unsubscribes  []string
	mu            sync.Mutex
}

// newPendingConn は接続を開始し、確立前の状態で返す。onSubscribe は client.Subscribe の記録後に呼ばれる (nil 可)。
func newPendingConn(t *testing.T, emitter cmn.Emitter, onSubscribe func(subscribeCall)) *pendingConn {
	t.Helper()
	p := &pendingConn{rec: &callbackRecorder{}, release: make(chan error)}
	client := &mockBrokerClient{
		connectFn: func(ctx context.Context) error {
			select {
			case err := <-p.release:
				return err
			case <-ctx.Done():
				return ctx.Err()
			}
		},
		subscribeFn: func(topic string, qos byte, handler domain.MessageHandler) error {
			call := subscribeCall{topic: topic, qos: qos}
			p.mu.Lock()
			p.subscribes = append(p.subscribes, call)
			err := p.subscribeErrs[topic]
			p.mu.Unlock()
			if onSubscribe != nil {
				onSubscribe(call)
			}
			if err != nil {
				return err
			}
			return p.rec.subscribeFn(topic, qos, handler)
		},
		unsubscribeFn: func(topic string) error {
			p.mu.Lock()
			defer p.mu.Unlock()
			p.unsubscribes = append(p.unsubscribes, topic)
			return nil
		},
	}
	p.svc = newTestService(t, emitter, p.rec.factory(client))
	id, err := p.svc.Connect(domain.ConnectionConfig{Broker: "tcp://localhost:1883"})
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	p.id = id
	return p
}

func (p *pendingConn) subscribeCalls() []subscribeCall {
	p.mu.Lock()
	defer p.mu.Unlock()
	return slices.Clone(p.subscribes)
}

func (p *pendingConn) unsubscribeCalls() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return slices.Clone(p.unsubscribes)
}

// establish は paho と同じく onConnected を先に起動してから Connect を復帰させ、接続を確立させる。
func (p *pendingConn) establish(t *testing.T) {
	t.Helper()
	p.rec.onConnected(0)()
	p.release <- nil
	waitEstablished(t, p.svc, 1)
}

// subscriptions は GetConnections が返す購読をトピック→QoS で返す。
func (p *pendingConn) subscriptions(t *testing.T) map[string]byte {
	t.Helper()
	conns := p.svc.GetConnections()
	if len(conns) != 1 {
		t.Fatalf("connections = %d, want 1", len(conns))
	}
	subs := map[string]byte{}
	for _, s := range conns[0].Subscriptions {
		subs[s.Topic] = s.QoS
	}
	return subs
}

// 確立前の Subscribe は client を呼ばずに成功し、確立時に同じ QoS で 1 回購読されてメッセージが届く。
func TestMQTTService_Subscribe_BeforeConnected_SubscribesOnConnected(t *testing.T) {
	emitter := &mockEmitter{}
	p := newPendingConn(t, emitter, nil)

	if err := p.svc.Subscribe(p.id, "a/#", 1); err != nil {
		t.Fatalf("Subscribe before connected: %v", err)
	}
	if got := p.subscribeCalls(); len(got) != 0 {
		t.Errorf("client.Subscribe called with %v before connected, want no calls", got)
	}
	if conns := p.svc.GetConnections(); len(conns) != 1 || conns[0].Connected {
		t.Fatalf("connections = %+v, want 1 not connected", conns)
	}
	if subs := p.subscriptions(t); !maps.Equal(subs, map[string]byte{"a/#": 1}) {
		t.Errorf("subscriptions = %v, want the pending one", subs)
	}

	p.establish(t)

	if got := p.subscribeCalls(); !slices.Equal(got, []subscribeCall{{"a/#", 1}}) {
		t.Fatalf("subscribed on connected = %v, want [{a/# 1}]", got)
	}
	p.rec.handler(0)("a/x", []byte("1"), 1, false)
	if n := emitter.count(cmn.EventMQTTMessage); n != 1 {
		t.Errorf("mqtt:message emitted %d times, want 1", n)
	}
}

// 確立前に受け付けた購読が、確立時に接続が開いたまま失敗したら (SUBACK を時間内に確認できない等)、
// 表示から外して client からも解除する。次の再接続が来ないので、残すと購読中と表示されたまま届かない。
func TestMQTTService_Subscribe_BeforeConnected_FailsOnConnected(t *testing.T) {
	p := newPendingConn(t, &mockEmitter{}, nil)
	p.subscribeErrs = map[string]error{"a/#": fmt.Errorf("subscribe: %w", domain.ErrAckTimeout)}
	for _, topic := range []string{"a/#", "b/#"} {
		if err := p.svc.Subscribe(p.id, topic, 1); err != nil {
			t.Fatalf("Subscribe(%s) before connected: %v", topic, err)
		}
	}

	p.establish(t)

	if got := p.subscribeCalls(); !slices.Equal(got, []subscribeCall{{"a/#", 1}, {"b/#", 1}}) {
		t.Fatalf("subscribed on connected = %v, want both", got)
	}
	if subs := p.subscriptions(t); !maps.Equal(subs, map[string]byte{"b/#": 1}) {
		t.Errorf("subscriptions = %v, want only b/#", subs)
	}
	if got := p.unsubscribeCalls(); !slices.Equal(got, []string{"a/#"}) {
		t.Errorf("client.Unsubscribe called with %v, want [a/#]", got)
	}
}

// 確立前に同じトピックを購読し直すと、確立時には最後の QoS で 1 回だけ購読する。
func TestMQTTService_Subscribe_BeforeConnected_UsesLatestQoS(t *testing.T) {
	p := newPendingConn(t, &mockEmitter{}, nil)
	for _, qos := range []byte{0, 1} {
		if err := p.svc.Subscribe(p.id, "a/#", qos); err != nil {
			t.Fatalf("Subscribe(qos=%d): %v", qos, err)
		}
	}

	p.establish(t)

	if got := p.subscribeCalls(); !slices.Equal(got, []subscribeCall{{"a/#", 1}}) {
		t.Errorf("subscribed on connected = %v, want [{a/# 1}]", got)
	}
}

// 確立前に購読して解除したトピックは、client を呼ばずに外れ、確立時にも購読しない。
func TestMQTTService_Unsubscribe_BeforeConnected_DropsPending(t *testing.T) {
	p := newPendingConn(t, &mockEmitter{}, nil)
	if err := p.svc.Subscribe(p.id, "a/#", 0); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	if err := p.svc.Subscribe(p.id, "b/#", 0); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	if err := p.svc.Unsubscribe(p.id, "a/#"); err != nil {
		t.Fatalf("Unsubscribe before connected: %v", err)
	}
	if got := p.unsubscribeCalls(); len(got) != 0 {
		t.Errorf("client.Unsubscribe called with %v before connected, want no calls", got)
	}
	if subs := p.subscriptions(t); !maps.Equal(subs, map[string]byte{"b/#": 0}) {
		t.Errorf("subscriptions = %v, want only b/#", subs)
	}

	p.establish(t)

	if got := p.subscribeCalls(); !slices.Equal(got, []subscribeCall{{"b/#", 0}}) {
		t.Errorf("subscribed on connected = %v, want only b/#", got)
	}
}

// 確立前に購読して接続に失敗したら、client は購読せず、接続ごと一覧から消える。
func TestMQTTService_Subscribe_BeforeConnected_DiscardedOnConnectFailure(t *testing.T) {
	failed := make(chan struct{})
	emitter := &mockEmitterWithChan{ch: failed, targetEvent: cmn.EventMQTTConnectionFailed}
	p := newPendingConn(t, emitter, nil)
	if err := p.svc.Subscribe(p.id, "a/#", 0); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	p.release <- errors.New("connection refused")
	waitForEvent(t, failed, time.Second, "timeout waiting for connection-failed event")

	if got := p.subscribeCalls(); len(got) != 0 {
		t.Errorf("client.Subscribe called with %v, want no calls", got)
	}
	if conns := p.svc.GetConnections(); len(conns) != 0 {
		t.Errorf("connections = %+v, want none", conns)
	}
}

// 確立前に購読して Disconnect したら、後から onConnected が届いても購読しない。
func TestMQTTService_Subscribe_BeforeConnected_DiscardedOnDisconnect(t *testing.T) {
	p := newPendingConn(t, &mockEmitter{}, nil)
	if err := p.svc.Subscribe(p.id, "a/#", 0); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	if err := p.svc.Disconnect(p.id); err != nil {
		t.Fatalf("Disconnect: %v", err)
	}
	waitConnGoroutines(t, p.svc)
	p.rec.onConnected(0)()

	if got := p.subscribeCalls(); len(got) != 0 {
		t.Errorf("client.Subscribe called with %v, want no calls", got)
	}
}

// 確立時の張り直しの最中に届いた Subscribe は張り直しの完了を待ち、その後に自分の QoS で 1 回だけ購読する。
func TestMQTTService_Subscribe_WaitsForResubscribeOnConnected(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	var first sync.Once
	p := newPendingConn(t, &mockEmitter{}, func(subscribeCall) {
		first.Do(func() {
			close(entered)
			<-release
		})
	})
	if err := p.svc.Subscribe(p.id, "a/#", 0); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	connected := make(chan struct{})
	go func() {
		p.rec.onConnected(0)()
		close(connected)
	}()
	waitForEvent(t, entered, time.Second, "resubscribe did not start")

	subscribed := make(chan error, 1)
	go func() { subscribed <- p.svc.Subscribe(p.id, "a/#", 1) }()
	assertBlocked(t, subscribed, "Subscribe returned while resubscribe was in flight")
	if got := p.subscribeCalls(); !slices.Equal(got, []subscribeCall{{"a/#", 0}}) {
		t.Errorf("client.Subscribe calls during resubscribe = %v, want only the resubscribe", got)
	}
	close(release)

	if err := <-subscribed; err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	waitForEvent(t, connected, time.Second, "onConnected did not return")
	if got := p.subscribeCalls(); !slices.Equal(got, []subscribeCall{{"a/#", 0}, {"a/#", 1}}) {
		t.Errorf("client.Subscribe calls = %v, want the resubscribe and then qos 1", got)
	}
	if subs := p.subscriptions(t); !maps.Equal(subs, map[string]byte{"a/#": 1}) {
		t.Errorf("subscriptions = %v, want a/# at qos 1", subs)
	}
	p.release <- nil
}

// 切断済みの接続と、購読解除した購読は張り直さない。
func TestMQTTService_Reconnect_SkipsUnsubscribedAndClosed(t *testing.T) {
	svc, id, rec, calls, _ := newResubscribeService(t, &mockEmitter{}, []subscribeCall{{"a/#", 0}, {"b/#", 0}})
	if err := svc.Unsubscribe(id, "b/#"); err != nil {
		t.Fatalf("Unsubscribe: %v", err)
	}
	rec.onConnected(0)()
	if got := calls()[2:]; !slices.Equal(got, []subscribeCall{{"a/#", 0}}) {
		t.Errorf("resubscribed = %v, want only a/#", got)
	}

	if err := svc.Disconnect(id); err != nil {
		t.Fatalf("Disconnect: %v", err)
	}
	before := len(calls())
	rec.onConnected(0)()
	if n := len(calls()); n != before {
		t.Errorf("Subscribe called %d times after Disconnect", n-before)
	}
}

// ------- Broker Topics のスキャン -------

// recordedClient は clientRecorder が作ったクライアント 1 つ分の記録。
type recordedClient struct {
	lost        func(error)
	config      domain.ConnectionConfig
	subscribes  []subscribeCall
	handlers    []domain.MessageHandler
	disconnects atomic.Int32
	mu          sync.Mutex
}

func (c *recordedClient) subscribeCalls() []subscribeCall {
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.subscribes)
}

// handler は i 番目の Subscribe に渡されたハンドラを返す。
func (c *recordedClient) handler(i int) domain.MessageHandler {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.handlers[i]
}

// clientRecorder は factory の呼び出しごとに別のクライアントを作って記録する。
// 0 番目は元の接続、1 番目以降はスキャン用のクライアント。
type clientRecorder struct {
	// prepare はクライアントを作るたびに呼ばれ、i 番目のクライアントの振る舞いを決める (nil 可)。
	prepare func(i int, rc *recordedClient, client *mockBrokerClient)
	clients []*recordedClient
	mu      sync.Mutex
}

func (r *clientRecorder) factory(cfg domain.ConnectionConfig, _ func(), onConnectionLost func(error)) domain.BrokerClient {
	r.mu.Lock()
	defer r.mu.Unlock()
	rc := &recordedClient{config: cfg, lost: onConnectionLost}
	client := &mockBrokerClient{}
	if r.prepare != nil {
		r.prepare(len(r.clients), rc, client)
	}
	subscribeFn := client.subscribeFn
	client.subscribeFn = func(topic string, qos byte, handler domain.MessageHandler) error {
		rc.mu.Lock()
		rc.subscribes = append(rc.subscribes, subscribeCall{topic: topic, qos: qos})
		rc.handlers = append(rc.handlers, handler)
		rc.mu.Unlock()
		if subscribeFn != nil {
			return subscribeFn(topic, qos, handler)
		}
		return nil
	}
	disconnectFn := client.disconnectFn
	client.disconnectFn = func(quiesce uint) {
		rc.disconnects.Add(1)
		if disconnectFn != nil {
			disconnectFn(quiesce)
		}
	}
	r.clients = append(r.clients, rc)
	return client
}

func (r *clientRecorder) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.clients)
}

func (r *clientRecorder) client(t *testing.T, i int) *recordedClient {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	if i >= len(r.clients) {
		t.Fatalf("client #%d was not created (%d clients)", i, len(r.clients))
	}
	return r.clients[i]
}

// newScanService は接続を 1 本確立し、クライアントを記録する clientRecorder と一緒に返す。
func newScanService(t *testing.T, emitter cmn.Emitter, prepare func(i int, rc *recordedClient, client *mockBrokerClient)) (*MQTTService, string, *clientRecorder) {
	t.Helper()
	rec := &clientRecorder{prepare: prepare}
	svc := newTestService(t, emitter, rec.factory)
	id, err := svc.Connect(domain.ConnectionConfig{Broker: "tcp://localhost:1883", ClientID: "my-client"})
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	waitEstablished(t, svc, 1)
	return svc, id, rec
}

// scanning は唯一の接続の Scanning を返す。
func scanning(t *testing.T, svc *MQTTService) bool {
	t.Helper()
	conns := svc.GetConnections()
	if len(conns) != 1 {
		t.Fatalf("connections = %d, want 1", len(conns))
	}
	return conns[0].Scanning
}

// assertNoScanEvents は mqtt:scan-topic と mqtt:scan-stopped が発行されていないことを確認する。
func assertNoScanEvents(t *testing.T, emitter *mockEmitter) {
	t.Helper()
	for _, event := range []string{cmn.EventMQTTScanTopic, cmn.EventMQTTScanStopped} {
		if n := emitter.count(event); n != 0 {
			t.Errorf("%s emitted %d times, want 0", event, n)
		}
	}
}

// スキャンは別のクライアント ID のクライアントで # を QoS 0 で購読し、元の接続では購読しない。
func TestMQTTService_StartTopicScan_UsesSeparateClient(t *testing.T) {
	svc, id, rec := newScanService(t, &mockEmitter{}, nil)

	if err := svc.StartTopicScan(id); err != nil {
		t.Fatalf("StartTopicScan: %v", err)
	}

	if n := rec.count(); n != 2 {
		t.Fatalf("clients = %d, want 2 (connection + scan)", n)
	}
	original, scan := rec.client(t, 0), rec.client(t, 1)
	if got := scan.config.ClientID; !strings.HasPrefix(got, scanClientIDPrefix) || len(got) != 20 || got == original.config.ClientID {
		t.Errorf("scan client ID = %q, want a 20-character ID starting with %q", got, scanClientIDPrefix)
	}
	if scan.config.Broker != original.config.Broker {
		t.Errorf("scan broker = %q, want %q", scan.config.Broker, original.config.Broker)
	}
	if got := scan.subscribeCalls(); !slices.Equal(got, []subscribeCall{{"#", 0}}) {
		t.Errorf("scan client subscribed %v, want [{# 0}]", got)
	}
	if got := original.subscribeCalls(); len(got) != 0 {
		t.Errorf("the connection's client subscribed %v, want no calls", got)
	}
	conns := svc.GetConnections()
	if !conns[0].Scanning || len(conns[0].Subscriptions) != 0 {
		t.Errorf("status = %+v, want Scanning without subscriptions", conns[0])
	}
}

// スキャン用クライアントが受信したら mqtt:scan-topic を発行し、mqtt:message は発行しない。
func TestMQTTService_TopicScan_EmitsScanTopic(t *testing.T) {
	emitter := &mockEmitter{}
	svc, id, rec := newScanService(t, emitter, nil)
	if err := svc.StartTopicScan(id); err != nil {
		t.Fatalf("StartTopicScan: %v", err)
	}

	rec.client(t, 1).handler(0)("sensors/temp", []byte("25.5"), 0, false)

	if n := emitter.count(cmn.EventMQTTMessage); n != 0 {
		t.Errorf("mqtt:message emitted %d times, want 0", n)
	}
	var got []any
	for _, ev := range emitter.snapshot() {
		if ev.event == cmn.EventMQTTScanTopic {
			got = append(got, ev.data)
		}
	}
	if want := []any{domain.ScannedTopic{ConnectionID: id, Topic: "sensors/temp"}}; !slices.Equal(got, want) {
		t.Errorf("mqtt:scan-topic = %v, want %v", got, want)
	}
}

// 購読の完了前に届いたトピック (SUBACK の直後に届く retained メッセージなど) は、開始が確定してから発行する。
func TestMQTTService_TopicScan_EmitsTopicsReceivedWhileStarting(t *testing.T) {
	emitter := &mockEmitter{}
	duringStart := -1
	svc, id, _ := newScanService(t, emitter, func(i int, _ *recordedClient, client *mockBrokerClient) {
		if i == 0 {
			return
		}
		client.subscribeFn = func(_ string, _ byte, handler domain.MessageHandler) error {
			handler("retained/topic", []byte("kept"), 0, true)
			duringStart = emitter.count(cmn.EventMQTTScanTopic)
			return nil
		}
	})

	if err := svc.StartTopicScan(id); err != nil {
		t.Fatalf("StartTopicScan: %v", err)
	}

	if duringStart != 0 {
		t.Errorf("mqtt:scan-topic emitted %d times before the scan started, want 0", duringStart)
	}
	if n := emitter.count(cmn.EventMQTTScanTopic); n != 1 {
		t.Errorf("mqtt:scan-topic emitted %d times, want 1", n)
	}
}

// 接続または購読に失敗したらエラーを返し、スキャン用クライアントを切断して残さない。
func TestMQTTService_StartTopicScan_Failure(t *testing.T) {
	failure := errors.New("refused")
	tests := []struct {
		prepare func(client *mockBrokerClient)
		name    string
	}{
		{name: "connect", prepare: func(client *mockBrokerClient) {
			client.connectFn = func(context.Context) error { return failure }
		}},
		{name: "subscribe", prepare: func(client *mockBrokerClient) {
			client.subscribeFn = func(string, byte, domain.MessageHandler) error { return failure }
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			emitter := &mockEmitter{}
			svc, id, rec := newScanService(t, emitter, func(i int, _ *recordedClient, client *mockBrokerClient) {
				if i == 1 {
					tc.prepare(client)
				}
			})

			if err := svc.StartTopicScan(id); !errors.Is(err, failure) {
				t.Fatalf("StartTopicScan error = %v, want %v", err, failure)
			}

			if n := rec.client(t, 1).disconnects.Load(); n != 1 {
				t.Errorf("scan client disconnected %d times, want 1", n)
			}
			if scanning(t, svc) {
				t.Error("Scanning = true after a failed start")
			}
			assertNoScanEvents(t, emitter)
			// 失敗したスキャンは残らないので、次の開始は新しいクライアントで始まる。
			if err := svc.StartTopicScan(id); err != nil {
				t.Fatalf("StartTopicScan after a failure: %v", err)
			}
			if n := rec.count(); n != 3 {
				t.Errorf("clients = %d, want 3", n)
			}
		})
	}
}

// 稼働中の二重の開始はクライアントを増やさず、止めたら切断し、次の開始は新しいクライアントで始まる。
func TestMQTTService_TopicScan_StartTwiceStopAndRestart(t *testing.T) {
	svc, id, rec := newScanService(t, &mockEmitter{}, nil)
	for range 2 {
		if err := svc.StartTopicScan(id); err != nil {
			t.Fatalf("StartTopicScan: %v", err)
		}
	}
	if n := rec.count(); n != 2 {
		t.Fatalf("clients = %d, want 2 (a second start must not add a client)", n)
	}

	// 2 回目の停止はスキャンが無いので何もしない。
	for range 2 {
		if err := svc.StopTopicScan(id); err != nil {
			t.Fatalf("StopTopicScan: %v", err)
		}
	}
	if n := rec.client(t, 1).disconnects.Load(); n != 1 {
		t.Errorf("scan client disconnected %d times, want 1", n)
	}
	if n := rec.client(t, 0).disconnects.Load(); n != 0 {
		t.Errorf("the connection's client disconnected %d times, want 0", n)
	}
	if scanning(t, svc) {
		t.Error("Scanning = true after StopTopicScan")
	}

	if err := svc.StartTopicScan(id); err != nil {
		t.Fatalf("StartTopicScan after stop: %v", err)
	}
	if n := rec.count(); n != 3 {
		t.Errorf("clients = %d, want 3 (a new scan client)", n)
	}
	if !scanning(t, svc) {
		t.Error("Scanning = false after restarting")
	}
}

func TestMQTTService_TopicScan_UnknownConnection(t *testing.T) {
	svc := newTestService(t, &mockEmitter{}, factoryWith(&mockBrokerClient{}))
	for name, op := range map[string]func(string) error{"StartTopicScan": svc.StartTopicScan, "StopTopicScan": svc.StopTopicScan} {
		if _, ok := errors.AsType[*cmn.NotFoundError](op("nonexistent")); !ok {
			t.Errorf("%s: expected NotFoundError", name)
		}
	}
}

// 止めた後にスキャン用クライアントの接続断やメッセージが届いても、イベントを発行しない。
func TestMQTTService_TopicScan_NoEventsAfterStop(t *testing.T) {
	emitter := &mockEmitter{}
	svc, id, rec := newScanService(t, emitter, nil)
	if err := svc.StartTopicScan(id); err != nil {
		t.Fatalf("StartTopicScan: %v", err)
	}
	if err := svc.StopTopicScan(id); err != nil {
		t.Fatalf("StopTopicScan: %v", err)
	}

	scan := rec.client(t, 1)
	scan.handler(0)("sensors/temp", []byte("1"), 0, false)
	scan.lost(errors.New("lost"))

	assertNoScanEvents(t, emitter)
	if n := scan.disconnects.Load(); n != 1 {
		t.Errorf("scan client disconnected %d times, want 1", n)
	}
}

// 元の接続が終端状態になる経路 (Disconnect・Shutdown・接続失敗) は、どれもスキャン用クライアントを切断する。
func TestMQTTService_TopicScan_StoppedWhenConnectionEnds(t *testing.T) {
	tests := []struct {
		end  func(t *testing.T, svc *MQTTService, id string, release chan<- error)
		name string
	}{
		{name: "Disconnect", end: func(t *testing.T, svc *MQTTService, id string, _ chan<- error) {
			t.Helper()
			if err := svc.Disconnect(id); err != nil {
				t.Fatalf("Disconnect: %v", err)
			}
		}},
		{name: "Shutdown", end: func(t *testing.T, svc *MQTTService, _ string, _ chan<- error) {
			t.Helper()
			if !svc.Shutdown(time.Second) {
				t.Fatal("expected Shutdown to drain within timeout")
			}
		}},
		{name: "connection failure", end: func(t *testing.T, svc *MQTTService, _ string, release chan<- error) {
			t.Helper()
			release <- errors.New("connection refused")
			waitConnGoroutines(t, svc)
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			emitter := &mockEmitter{}
			release := make(chan error)
			rec := &clientRecorder{prepare: func(i int, _ *recordedClient, client *mockBrokerClient) {
				if i != 0 {
					return
				}
				// 元の接続は確立前のまま留める。
				client.connectFn = func(ctx context.Context) error {
					select {
					case err := <-release:
						return err
					case <-ctx.Done():
						return ctx.Err()
					}
				}
			}}
			svc := newTestService(t, emitter, rec.factory)
			id, err := svc.Connect(domain.ConnectionConfig{Broker: "tcp://localhost:1883"})
			if err != nil {
				t.Fatalf("Connect: %v", err)
			}
			// スキャンは元の接続の確立前でも始められる。
			if err := svc.StartTopicScan(id); err != nil {
				t.Fatalf("StartTopicScan before connected: %v", err)
			}

			tc.end(t, svc, id, release)

			scan := rec.client(t, 1)
			if n := scan.disconnects.Load(); n != 1 {
				t.Errorf("scan client disconnected %d times, want 1", n)
			}
			scan.handler(0)("sensors/temp", []byte("1"), 0, false)
			scan.lost(errors.New("lost"))
			assertNoScanEvents(t, emitter)
		})
	}
}

// startingScan は、スキャン用クライアントの Connect を止めて開始中に留めたスキャン。
type startingScan struct {
	svc     *MQTTService
	rec     *clientRecorder
	emitter *mockEmitter
	// connecting はスキャン用クライアントの Connect が始まると閉じる。
	connecting chan struct{}
	// release に送った値をスキャン用クライアントの Connect が返す。
	release chan error
	// result は StartTopicScan の結果を受け取る。
	result chan error
	id     string
}

// newStartingScan は StartTopicScan を別の goroutine で呼び、スキャン用クライアントの Connect の途中で返す。
// ignoreCtx が true なら Connect は打ち切りを無視し、release でしか復帰しない (打ち切りと行き違いで成功する接続)。
func newStartingScan(t *testing.T, ignoreCtx bool, prepareScan func(rc *recordedClient, client *mockBrokerClient)) *startingScan {
	t.Helper()
	p := &startingScan{
		emitter:    &mockEmitter{},
		connecting: make(chan struct{}),
		release:    make(chan error),
		result:     make(chan error, 1),
	}
	p.svc, p.id, p.rec = newScanService(t, p.emitter, func(i int, rc *recordedClient, client *mockBrokerClient) {
		if i != 1 {
			return
		}
		client.connectFn = func(ctx context.Context) error {
			close(p.connecting)
			done := ctx.Done()
			if ignoreCtx {
				done = nil
			}
			select {
			case err := <-p.release:
				return err
			case <-done:
				return ctx.Err()
			}
		}
		if prepareScan != nil {
			prepareScan(rc, client)
		}
	})
	go func() { p.result <- p.svc.StartTopicScan(p.id) }()
	waitForEvent(t, p.connecting, time.Second, "scan client did not start connecting")
	return p
}

// wait は StartTopicScan の結果を返す。
func (p *startingScan) wait(t *testing.T) error {
	t.Helper()
	select {
	case err := <-p.result:
		return err
	case <-time.After(2 * time.Second):
		t.Fatal("StartTopicScan did not return")
		return nil
	}
}

// 開始中は Scanning を返さない。
func TestMQTTService_TopicScan_NotScanningWhileStarting(t *testing.T) {
	p := newStartingScan(t, false, nil)
	if scanning(t, p.svc) {
		t.Error("Scanning = true while the scan is still starting")
	}
	p.release <- nil
	if err := p.wait(t); err != nil {
		t.Fatalf("StartTopicScan: %v", err)
	}
	if !scanning(t, p.svc) {
		t.Error("Scanning = false after the scan started")
	}
}

// 開始の途中で止められたら (StopTopicScan・Disconnect・Shutdown)、StartTopicScan はエラーを返し、
// 接続が成功していても捨て、以後イベントを発行しない。
func TestMQTTService_TopicScan_StoppedWhileStarting(t *testing.T) {
	tests := []struct {
		stop func(t *testing.T, p *startingScan)
		name string
		// lateSuccess は、止めた後にスキャン用クライアントの接続が成功する場合。
		lateSuccess bool
	}{
		{name: "StopTopicScan", stop: func(t *testing.T, p *startingScan) {
			t.Helper()
			if err := p.svc.StopTopicScan(p.id); err != nil {
				t.Fatalf("StopTopicScan: %v", err)
			}
		}},
		{name: "StopTopicScan then late success", lateSuccess: true, stop: func(t *testing.T, p *startingScan) {
			t.Helper()
			if err := p.svc.StopTopicScan(p.id); err != nil {
				t.Fatalf("StopTopicScan: %v", err)
			}
		}},
		{name: "Disconnect then late success", lateSuccess: true, stop: func(t *testing.T, p *startingScan) {
			t.Helper()
			if err := p.svc.Disconnect(p.id); err != nil {
				t.Fatalf("Disconnect: %v", err)
			}
		}},
		{name: "Shutdown", stop: func(t *testing.T, p *startingScan) {
			t.Helper()
			// Shutdown はスキャンの開始が復帰するのを待つ。
			if !p.svc.Shutdown(time.Second) {
				t.Fatal("expected Shutdown to drain within timeout")
			}
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := newStartingScan(t, tc.lateSuccess, nil)

			tc.stop(t, p)
			if tc.lateSuccess {
				p.release <- nil
			}

			if err := p.wait(t); !errors.Is(err, errScanStopped) {
				t.Fatalf("StartTopicScan error = %v, want %v", err, errScanStopped)
			}
			scan := p.rec.client(t, 1)
			if n := scan.disconnects.Load(); n != 1 {
				t.Errorf("scan client disconnected %d times, want 1", n)
			}
			if conns := p.svc.GetConnections(); len(conns) == 1 && conns[0].Scanning {
				t.Error("Scanning = true after the start was stopped")
			}
			if tc.lateSuccess {
				scan.handler(0)("sensors/temp", []byte("1"), 0, false)
			}
			scan.lost(errors.New("lost"))
			assertNoScanEvents(t, p.emitter)
		})
	}
}

// 開始の途中の 2 回目の StartTopicScan はクライアントを増やさず、1 回目と同じ結果を返す。
func TestMQTTService_TopicScan_SecondStartJoinsTheFirst(t *testing.T) {
	failure := errors.New("refused")
	for _, connectErr := range []error{nil, failure} {
		t.Run(fmt.Sprint(connectErr), func(t *testing.T) {
			p := newStartingScan(t, false, nil)
			second := make(chan error, 1)
			go func() { second <- p.svc.StartTopicScan(p.id) }()
			assertBlocked(t, second, "the second StartTopicScan returned before the first finished")

			p.release <- connectErr

			first := p.wait(t)
			if !errors.Is(first, connectErr) {
				t.Errorf("first StartTopicScan error = %v, want %v", first, connectErr)
			}
			select {
			case err := <-second:
				if !errors.Is(err, first) {
					t.Errorf("second StartTopicScan error = %v, want the first result %v", err, first)
				}
			case <-time.After(time.Second):
				t.Fatal("the second StartTopicScan did not return")
			}
			if n := p.rec.count(); n != 2 {
				t.Errorf("clients = %d, want 2", n)
			}
			if got := scanning(t, p.svc); got != (connectErr == nil) {
				t.Errorf("Scanning = %v, want %v", got, connectErr == nil)
			}
		})
	}
}

// スキャン用の接続が切れたら mqtt:scan-stopped を 1 回発行し、クライアントを切断して自動再接続を止める。
func TestMQTTService_TopicScan_ConnectionLost_StopsScan(t *testing.T) {
	emitter := &mockEmitter{}
	svc, id, rec := newScanService(t, emitter, nil)
	if err := svc.StartTopicScan(id); err != nil {
		t.Fatalf("StartTopicScan: %v", err)
	}
	scan := rec.client(t, 1)

	scan.lost(errors.New("broker went away"))
	// 止まった後の接続断とメッセージは無視する。
	scan.lost(errors.New("again"))
	scan.handler(0)("sensors/temp", []byte("1"), 0, false)

	var stopped []any
	for _, ev := range emitter.snapshot() {
		if ev.event == cmn.EventMQTTScanStopped {
			stopped = append(stopped, ev.data)
		}
	}
	if len(stopped) != 1 {
		t.Fatalf("mqtt:scan-stopped emitted %d times, want 1", len(stopped))
	}
	if want := (domain.ConnectionErrorEvent{ConnectionID: id, Error: "broker went away"}); stopped[0] != want {
		t.Errorf("event data = %#v, want %#v", stopped[0], want)
	}
	if n := emitter.count(cmn.EventMQTTScanTopic); n != 0 {
		t.Errorf("mqtt:scan-topic emitted %d times after the scan stopped, want 0", n)
	}
	if n := scan.disconnects.Load(); n != 1 {
		t.Errorf("scan client disconnected %d times, want 1", n)
	}
	if scanning(t, svc) {
		t.Error("Scanning = true after the scan connection was lost")
	}
	// 元の接続はそのまま。
	if n := emitter.count(cmn.EventMQTTConnectionLost); n != 0 {
		t.Errorf("mqtt:connection-lost emitted %d times, want 0", n)
	}
}

// 開始の途中でスキャン用の接続が切れたら、イベントは発行せず StartTopicScan がエラーを返す。
func TestMQTTService_TopicScan_ConnectionLostWhileStarting(t *testing.T) {
	lost := errors.New("broker went away")
	p := newStartingScan(t, false, func(rc *recordedClient, client *mockBrokerClient) {
		// 購読の完了前に接続が切れる。
		client.subscribeFn = func(string, byte, domain.MessageHandler) error {
			rc.lost(lost)
			return nil
		}
	})

	p.release <- nil

	if err := p.wait(t); !errors.Is(err, lost) {
		t.Fatalf("StartTopicScan error = %v, want %v", err, lost)
	}
	assertNoScanEvents(t, p.emitter)
	if n := p.rec.client(t, 1).disconnects.Load(); n != 1 {
		t.Errorf("scan client disconnected %d times, want 1", n)
	}
	if scanning(t, p.svc) {
		t.Error("Scanning = true after the start failed")
	}
}

// 終了処理の後は、スキャンを始めない。
func TestMQTTService_StartTopicScan_AfterShutdown_Rejected(t *testing.T) {
	svc, id, rec := newScanService(t, &mockEmitter{}, nil)
	if !svc.Shutdown(time.Second) {
		t.Fatal("expected Shutdown to drain within timeout")
	}

	if err := svc.StartTopicScan(id); !errors.Is(err, errShuttingDown) {
		t.Errorf("err = %v, want errShuttingDown", err)
	}
	if n := rec.count(); n != 1 {
		t.Errorf("clients = %d, want 1 (no scan client)", n)
	}
}
