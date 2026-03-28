// Package udpapp は UDP ユースケース層を提供する。
package udpapp

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net"
	"strconv"
	"strings"

	domain "github.com/f0reth/Wirexa/internal/domain/udp"
)

var _ domain.SendUseCase = (*UdpSendService)(nil)

// UdpSendService は UDP パケット送信ユースケースの実装。
type UdpSendService struct{}

// NewUdpSendService は UdpSendService を生成する。
func NewUdpSendService() *UdpSendService {
	return &UdpSendService{}
}

// Send は UDP パケットを送信して結果を返す。
func (s *UdpSendService) Send(req domain.UdpSendRequest) (domain.UdpSendResult, error) {
	if req.Host == "" {
		return domain.UdpSendResult{}, &domain.ValidationError{Field: "host", Message: "required"}
	}
	if req.Port < 1 || req.Port > 65535 {
		return domain.UdpSendResult{}, &domain.ValidationError{Field: "port", Message: "must be 1-65535"}
	}

	data, err := decodePayload(req.Payload, req.Encoding)
	if err != nil {
		return domain.UdpSendResult{}, err
	}

	addr := net.JoinHostPort(req.Host, strconv.Itoa(req.Port))
	conn, err := net.Dial("udp4", addr)
	if err != nil {
		return domain.UdpSendResult{}, fmt.Errorf("failed to connect: %w", err)
	}
	defer conn.Close()

	n, err := conn.Write(data)
	if err != nil {
		return domain.UdpSendResult{}, fmt.Errorf("failed to send: %w", err)
	}

	return domain.UdpSendResult{BytesSent: n}, nil
}

func decodePayload(payload string, encoding domain.PayloadEncoding) ([]byte, error) {
	switch encoding {
	case domain.EncodingText:
		return []byte(payload), nil
	case domain.EncodingHex:
		cleaned := strings.ReplaceAll(payload, " ", "")
		data, err := hex.DecodeString(cleaned)
		if err != nil {
			return nil, &domain.ValidationError{Field: "payload", Message: "invalid hex: " + err.Error()}
		}
		return data, nil
	case domain.EncodingBase64:
		data, err := base64.StdEncoding.DecodeString(payload)
		if err != nil {
			return nil, &domain.ValidationError{Field: "payload", Message: "invalid base64: " + err.Error()}
		}
		return data, nil
	default:
		return nil, &domain.ValidationError{Field: "encoding", Message: "unknown: " + string(encoding)}
	}
}
