package udpapp

import (
	"errors"
	"testing"

	cmn "github.com/f0reth/Wirexa/internal/domain"
	domain "github.com/f0reth/Wirexa/internal/domain/udp"
)

// mockUDPSocket は domain.UdpSocket のインメモリモック。
type mockUDPSocket struct {
	sendFn   func(host string, port int, data []byte) (int, error)
	listenFn func(port int) (domain.UdpConn, error)
}

func (m *mockUDPSocket) Send(host string, port int, data []byte) (int, error) {
	if m.sendFn != nil {
		return m.sendFn(host, port, data)
	}
	return len(data), nil
}

func (m *mockUDPSocket) Listen(port int) (domain.UdpConn, error) {
	if m.listenFn != nil {
		return m.listenFn(port)
	}
	return nil, errors.New("Listen not implemented in mock")
}

func newSendSvc(socket domain.UdpSocket) *UdpSendService {
	if socket == nil {
		socket = &mockUDPSocket{}
	}
	return NewUdpSendService(socket, cmn.NoopLogger{})
}

func TestUdpSendService_Send_EmptyHost(t *testing.T) {
	svc := newSendSvc(nil)
	_, err := svc.Send(domain.UdpSendRequest{Host: "", Port: 9000, Encoding: domain.EncodingText, Payload: "hi"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var ve *cmn.ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("expected ValidationError, got %T", err)
	}
}

func TestUdpSendService_Send_InvalidPort(t *testing.T) {
	svc := newSendSvc(nil)
	for _, port := range []int{0, -1, 65536} {
		_, err := svc.Send(domain.UdpSendRequest{Host: "localhost", Port: port, Encoding: domain.EncodingText})
		if err == nil {
			t.Errorf("expected error for port=%d, got nil", port)
		}
	}
}

func TestUdpSendService_Send_InvalidJSON(t *testing.T) {
	svc := newSendSvc(nil)
	_, err := svc.Send(domain.UdpSendRequest{
		Host:     "localhost",
		Port:     9000,
		Encoding: domain.EncodingJSON,
		Payload:  "not json",
	})
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestUdpSendService_Send_SocketError(t *testing.T) {
	wantErr := errors.New("network unreachable")
	socket := &mockUDPSocket{
		sendFn: func(_ string, _ int, _ []byte) (int, error) {
			return 0, wantErr
		},
	}
	svc := newSendSvc(socket)
	_, err := svc.Send(domain.UdpSendRequest{
		Host:     "localhost",
		Port:     9000,
		Encoding: domain.EncodingText,
		Payload:  "hello",
	})
	if !errors.Is(err, wantErr) {
		t.Errorf("expected network error, got %v", err)
	}
}

func TestUdpSendService_Send_TextEncoding(t *testing.T) {
	var sentData []byte
	socket := &mockUDPSocket{
		sendFn: func(_ string, _ int, data []byte) (int, error) {
			sentData = data
			return len(data), nil
		},
	}
	svc := newSendSvc(socket)
	result, err := svc.Send(domain.UdpSendRequest{
		Host:     "127.0.0.1",
		Port:     9000,
		Encoding: domain.EncodingText,
		Payload:  "hello world",
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if string(sentData) != "hello world" {
		t.Errorf("sentData = %q, want %q", sentData, "hello world")
	}
	if result.BytesSent != 11 {
		t.Errorf("BytesSent = %d, want 11", result.BytesSent)
	}
}

func TestUdpSendService_Send_FixedLengthEncoding(t *testing.T) {
	var sentData []byte
	socket := &mockUDPSocket{
		sendFn: func(_ string, _ int, data []byte) (int, error) {
			sentData = data
			return len(data), nil
		},
	}
	svc := newSendSvc(socket)
	_, err := svc.Send(domain.UdpSendRequest{
		Host:     "localhost",
		Port:     9000,
		Encoding: domain.EncodingFixed,
		FixedLengthPayload: domain.FixedLengthPayload{
			Fields: []domain.FixedLengthField{
				{Name: "header", Length: 3, Value: "HDR"},
				{Name: "data", Length: 4, Value: "AB"},
			},
		},
	})
	if err != nil {
		t.Fatalf("Send fixed: %v", err)
	}
	// "HDR" + "AB\x00\x00"
	want := []byte{'H', 'D', 'R', 'A', 'B', 0x00, 0x00}
	if len(sentData) != len(want) {
		t.Fatalf("sentData = %v, want %v", sentData, want)
	}
	for i := range want {
		if sentData[i] != want[i] {
			t.Errorf("sentData[%d] = %#x, want %#x", i, sentData[i], want[i])
		}
	}
}

func TestUdpSendService_Send_FixedLengthEncoding_InvalidPayload(t *testing.T) {
	svc := newSendSvc(nil)
	_, err := svc.Send(domain.UdpSendRequest{
		Host:               "localhost",
		Port:               9000,
		Encoding:           domain.EncodingFixed,
		FixedLengthPayload: domain.FixedLengthPayload{Fields: []domain.FixedLengthField{}},
	})
	if err == nil {
		t.Fatal("expected error for empty fixed length fields, got nil")
	}
}

func TestUdpSendService_Send_BytesSentMatchesSocketReturn(t *testing.T) {
	socket := &mockUDPSocket{
		sendFn: func(_ string, _ int, _ []byte) (int, error) {
			return 3, nil // 実際に送信されたバイト数
		},
	}
	svc := newSendSvc(socket)
	result, err := svc.Send(domain.UdpSendRequest{
		Host:     "localhost",
		Port:     9000,
		Encoding: domain.EncodingText,
		Payload:  "hello",
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if result.BytesSent != 3 {
		t.Errorf("BytesSent = %d, want 3", result.BytesSent)
	}
}
