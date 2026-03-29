package udpapp

import (
	"errors"
	"sync"
	"testing"
	"time"

	cmn "github.com/f0reth/Wirexa/internal/domain"
	domain "github.com/f0reth/Wirexa/internal/domain/udp"
)

// mockUDPConn は domain.UdpConn のモック。
// packets に登録されたデータを順番に返し、なくなると done チャンネルが閉じられるまでブロックする。
type mockUDPConn struct {
	mu      sync.Mutex
	idx     int
	packets []struct {
		data []byte
		addr string
	}
	done   chan struct{}
	closed bool
}

func newMockConn(packets ...struct {
	data []byte
	addr string
},
) *mockUDPConn {
	return &mockUDPConn{
		packets: packets,
		done:    make(chan struct{}),
	}
}

func (m *mockUDPConn) ReadFrom(b []byte) (int, string, error) {
	m.mu.Lock()
	i := m.idx
	m.idx++
	m.mu.Unlock()

	if i < len(m.packets) {
		n := copy(b, m.packets[i].data)
		return n, m.packets[i].addr, nil
	}
	// ブロックして Close() を待つ
	<-m.done
	return 0, "", errors.New("connection closed")
}

func (m *mockUDPConn) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.closed {
		m.closed = true
		close(m.done)
	}
	return nil
}

// listenerEmitter は "udp:message" イベントをチャンネルで通知するモック。
type listenerEmitter struct {
	msgCh chan domain.UdpReceivedMessage
}

func newListenerEmitter() *listenerEmitter {
	return &listenerEmitter{msgCh: make(chan domain.UdpReceivedMessage, 10)}
}

func (e *listenerEmitter) Emit(event string, data any) {
	if event == "udp:message" {
		if msg, ok := data.(domain.UdpReceivedMessage); ok {
			e.msgCh <- msg
		}
	}
}

func newListenerSvc(socket domain.UdpSocket, emitter cmn.Emitter) *UdpListenerService {
	if socket == nil {
		socket = &mockUDPSocket{}
	}
	if emitter == nil {
		emitter = newListenerEmitter()
	}
	return NewUdpListenerService(socket, emitter)
}

// ------- StartListen -------

func TestUdpListenerService_StartListen_InvalidPort(t *testing.T) {
	svc := newListenerSvc(nil, nil)
	for _, port := range []int{0, -1, 65536, 99999} {
		_, err := svc.StartListen(port, domain.EncodingText)
		if err == nil {
			t.Errorf("expected error for port=%d, got nil", port)
		}
		var ve *cmn.ValidationError
		if !errors.As(err, &ve) {
			t.Errorf("port=%d: expected ValidationError, got %T", port, err)
		}
	}
}

func TestUdpListenerService_StartListen_ValidPorts(t *testing.T) {
	for _, port := range []int{1, 1024, 65535} {
		conn := newMockConn()
		socket := &mockUDPSocket{
			listenFn: func(_ int) (domain.UdpConn, error) {
				return conn, nil
			},
		}
		svc := newListenerSvc(socket, nil)
		session, err := svc.StartListen(port, domain.EncodingText)
		if err != nil {
			t.Errorf("port=%d: unexpected error %v", port, err)
			continue
		}
		if session.Port != port {
			t.Errorf("port=%d: session.Port = %d", port, session.Port)
		}
		conn.Close()
	}
}

func TestUdpListenerService_StartListen_DuplicatePort(t *testing.T) {
	conn := newMockConn()
	socket := &mockUDPSocket{
		listenFn: func(_ int) (domain.UdpConn, error) {
			return conn, nil
		},
	}
	svc := newListenerSvc(socket, nil)
	if _, err := svc.StartListen(9000, domain.EncodingText); err != nil {
		t.Fatalf("first StartListen: %v", err)
	}

	_, err := svc.StartListen(9000, domain.EncodingText)
	if err == nil {
		t.Fatal("expected error for duplicate port, got nil")
	}
	var ve *cmn.ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("expected ValidationError, got %T", err)
	}
	conn.Close()
}

func TestUdpListenerService_StartListen_SocketError(t *testing.T) {
	socket := &mockUDPSocket{
		listenFn: func(_ int) (domain.UdpConn, error) {
			return nil, errors.New("address already in use")
		},
	}
	svc := newListenerSvc(socket, nil)
	_, err := svc.StartListen(9000, domain.EncodingText)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestUdpListenerService_StartListen_SessionHasCorrectFields(t *testing.T) {
	conn := newMockConn()
	socket := &mockUDPSocket{
		listenFn: func(_ int) (domain.UdpConn, error) { return conn, nil },
	}
	svc := newListenerSvc(socket, nil)
	session, err := svc.StartListen(5555, domain.EncodingHex)
	if err != nil {
		t.Fatalf("StartListen: %v", err)
	}
	defer conn.Close()

	if session.ID == "" {
		t.Error("expected non-empty session ID")
	}
	if session.Port != 5555 {
		t.Errorf("Port = %d, want 5555", session.Port)
	}
	if session.Encoding != domain.EncodingHex {
		t.Errorf("Encoding = %q, want hex", session.Encoding)
	}
}

// ------- StopListen -------

func TestUdpListenerService_StopListen_NotFound(t *testing.T) {
	svc := newListenerSvc(nil, nil)
	err := svc.StopListen("nonexistent")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var nfe *cmn.NotFoundError
	if !errors.As(err, &nfe) {
		t.Errorf("expected NotFoundError, got %T", err)
	}
}

func TestUdpListenerService_StopListen_Success(t *testing.T) {
	conn := newMockConn()
	socket := &mockUDPSocket{
		listenFn: func(_ int) (domain.UdpConn, error) { return conn, nil },
	}
	svc := newListenerSvc(socket, nil)
	session, _ := svc.StartListen(9000, domain.EncodingText)

	if err := svc.StopListen(session.ID); err != nil {
		t.Fatalf("StopListen: %v", err)
	}
	if len(svc.GetListeners()) != 0 {
		t.Error("expected 0 listeners after stop")
	}
	if !conn.closed {
		t.Error("expected conn.Close() to be called")
	}
}

// ------- GetListeners -------

func TestUdpListenerService_GetListeners_Empty(t *testing.T) {
	svc := newListenerSvc(nil, nil)
	if listeners := svc.GetListeners(); len(listeners) != 0 {
		t.Errorf("expected 0 listeners, got %d", len(listeners))
	}
}

func TestUdpListenerService_GetListeners_WithSessions(t *testing.T) {
	conn1 := newMockConn()
	conn2 := newMockConn()
	callCount := 0
	conns := []*mockUDPConn{conn1, conn2}
	socket := &mockUDPSocket{
		listenFn: func(_ int) (domain.UdpConn, error) {
			c := conns[callCount]
			callCount++
			return c, nil
		},
	}
	svc := newListenerSvc(socket, nil)
	svc.StartListen(9001, domain.EncodingText)
	svc.StartListen(9002, domain.EncodingBase64)
	defer conn1.Close()
	defer conn2.Close()

	listeners := svc.GetListeners()
	if len(listeners) != 2 {
		t.Errorf("expected 2 listeners, got %d", len(listeners))
	}
}

// ------- StopAll -------

func TestUdpListenerService_StopAll_ClosesAllConns(t *testing.T) {
	conn1 := newMockConn()
	conn2 := newMockConn()
	callCount := 0
	conns := []*mockUDPConn{conn1, conn2}
	socket := &mockUDPSocket{
		listenFn: func(_ int) (domain.UdpConn, error) {
			c := conns[callCount]
			callCount++
			return c, nil
		},
	}
	svc := newListenerSvc(socket, nil)
	svc.StartListen(9001, domain.EncodingText)
	svc.StartListen(9002, domain.EncodingBase64)

	svc.StopAll()

	if len(svc.GetListeners()) != 0 {
		t.Error("expected 0 listeners after StopAll")
	}
	if !conn1.closed {
		t.Error("conn1 should be closed")
	}
	if !conn2.closed {
		t.Error("conn2 should be closed")
	}
}

func TestUdpListenerService_StopAll_NoSessions(_ *testing.T) {
	svc := newListenerSvc(nil, nil)
	// panic しないことを確認
	svc.StopAll()
}

// ------- receiveLoop: message emission -------

func TestUdpListenerService_ReceiveLoop_EmitsMessage(t *testing.T) {
	pkt := struct {
		data []byte
		addr string
	}{data: []byte("hello"), addr: "127.0.0.1:12345"}
	conn := newMockConn(pkt)

	socket := &mockUDPSocket{
		listenFn: func(_ int) (domain.UdpConn, error) { return conn, nil },
	}
	emitter := newListenerEmitter()
	svc := NewUdpListenerService(socket, emitter)

	session, err := svc.StartListen(9000, domain.EncodingText)
	if err != nil {
		t.Fatalf("StartListen: %v", err)
	}
	defer svc.StopAll()

	select {
	case msg := <-emitter.msgCh:
		if msg.SessionID != session.ID {
			t.Errorf("SessionID = %q, want %q", msg.SessionID, session.ID)
		}
		if msg.Port != 9000 {
			t.Errorf("Port = %d, want 9000", msg.Port)
		}
		if msg.RemoteAddr != "127.0.0.1:12345" {
			t.Errorf("RemoteAddr = %q, want %q", msg.RemoteAddr, "127.0.0.1:12345")
		}
		if msg.Payload != "hello" {
			t.Errorf("Payload = %q, want %q", msg.Payload, "hello")
		}
		if msg.Encoding != domain.EncodingText {
			t.Errorf("Encoding = %q, want text", msg.Encoding)
		}
		if msg.Timestamp == 0 {
			t.Error("Timestamp should not be zero")
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for udp:message event")
	}
}

func TestUdpListenerService_ReceiveLoop_HexEncoding(t *testing.T) {
	pkt := struct {
		data []byte
		addr string
	}{data: []byte{0xDE, 0xAD}, addr: "10.0.0.1:5000"}
	conn := newMockConn(pkt)

	socket := &mockUDPSocket{
		listenFn: func(_ int) (domain.UdpConn, error) { return conn, nil },
	}
	emitter := newListenerEmitter()
	svc := NewUdpListenerService(socket, emitter)

	svc.StartListen(9000, domain.EncodingHex)
	defer svc.StopAll()

	select {
	case msg := <-emitter.msgCh:
		if msg.Payload != "dead" {
			t.Errorf("Payload = %q, want %q", msg.Payload, "dead")
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for message")
	}
}

func TestUdpListenerService_ReceiveLoop_StopsOnClose(t *testing.T) {
	conn := newMockConn() // パケットなし、Close() を待つ
	socket := &mockUDPSocket{
		listenFn: func(_ int) (domain.UdpConn, error) { return conn, nil },
	}
	svc := newListenerSvc(socket, nil)
	session, _ := svc.StartListen(9000, domain.EncodingText)
	svc.StopListen(session.ID)

	// goroutine が終了していることを間接的に確認（StopAll がハングしないこと）
	done := make(chan struct{})
	go func() {
		svc.StopAll()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("StopAll should complete quickly after StopListen")
	}
}
