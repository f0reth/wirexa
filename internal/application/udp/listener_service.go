package udpapp

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/google/uuid"

	domain "github.com/f0reth/Wirexa/internal/domain/udp"
)

var _ domain.ListenUseCase = (*UdpListenerService)(nil)

// listenSession はアクティブなリスニングセッションの内部状態を保持する。
type listenSession struct {
	session domain.UdpListenSession
	conn    net.PacketConn
}

// UdpListenerService は UDP 受信ユースケースの実装。
type UdpListenerService struct {
	emitter  domain.UdpEmitter
	sessions map[string]*listenSession
	mu       sync.Mutex
}

// NewUdpListenerService は UdpListenerService を生成する。
func NewUdpListenerService(emitter domain.UdpEmitter) *UdpListenerService {
	return &UdpListenerService{
		emitter:  emitter,
		sessions: make(map[string]*listenSession),
	}
}

// StartListen は指定ポートでリスニングを開始し、セッションを返す。
func (s *UdpListenerService) StartListen(port int, encoding domain.PayloadEncoding) (domain.UdpListenSession, error) {
	if port < 1 || port > 65535 {
		return domain.UdpListenSession{}, &domain.ValidationError{Field: "port", Message: "must be 1-65535"}
	}

	s.mu.Lock()
	for _, ls := range s.sessions {
		if ls.session.Port == port {
			s.mu.Unlock()
			return domain.UdpListenSession{}, &domain.ValidationError{Field: "port", Message: fmt.Sprintf("port %d is already listening", port)}
		}
	}
	s.mu.Unlock()

	conn, err := net.ListenPacket("udp4", fmt.Sprintf(":%d", port))
	if err != nil {
		return domain.UdpListenSession{}, fmt.Errorf("failed to listen on port %d: %w", port, err)
	}

	session := domain.UdpListenSession{
		ID:       uuid.NewString(),
		Port:     port,
		Encoding: encoding,
	}

	ls := &listenSession{session: session, conn: conn}

	s.mu.Lock()
	s.sessions[session.ID] = ls
	s.mu.Unlock()

	go s.receiveLoop(ls)

	return session, nil
}

// StopListen は指定セッションのリスニングを停止する。
func (s *UdpListenerService) StopListen(sessionID string) error {
	s.mu.Lock()
	ls, ok := s.sessions[sessionID]
	if !ok {
		s.mu.Unlock()
		return &domain.NotFoundError{Resource: "session", ID: sessionID}
	}
	delete(s.sessions, sessionID)
	s.mu.Unlock()

	return ls.conn.Close()
}

// GetListeners はアクティブなセッション一覧を返す。
func (s *UdpListenerService) GetListeners() []domain.UdpListenSession {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]domain.UdpListenSession, 0, len(s.sessions))
	for _, ls := range s.sessions {
		result = append(result, ls.session)
	}
	return result
}

// StopAll は全セッションを停止する。
func (s *UdpListenerService) StopAll() {
	s.mu.Lock()
	sessions := make([]*listenSession, 0, len(s.sessions))
	for _, ls := range s.sessions {
		sessions = append(sessions, ls)
	}
	s.sessions = make(map[string]*listenSession)
	s.mu.Unlock()

	for _, ls := range sessions {
		_ = ls.conn.Close()
	}
}

// receiveLoop は指定セッションのパケット受信ループ。
func (s *UdpListenerService) receiveLoop(ls *listenSession) {
	buf := make([]byte, 65535)
	for {
		n, addr, err := ls.conn.ReadFrom(buf)
		if err != nil {
			// conn.Close() による終了
			return
		}

		payload := encodePayload(buf[:n], ls.session.Encoding)

		msg := domain.UdpReceivedMessage{
			SessionID:  ls.session.ID,
			Port:       ls.session.Port,
			RemoteAddr: addr.String(),
			Payload:    payload,
			Encoding:   ls.session.Encoding,
			Timestamp:  time.Now().UnixMilli(),
		}

		s.emitter.Emit("udp:message", msg)
	}
}

// encodePayload は受信バイト列を指定エンコーディングで文字列化する。
func encodePayload(data []byte, encoding domain.PayloadEncoding) string {
	switch encoding {
	case domain.EncodingHex:
		return hex.EncodeToString(data)
	case domain.EncodingBase64:
		return base64.StdEncoding.EncodeToString(data)
	default:
		return string(data)
	}
}
