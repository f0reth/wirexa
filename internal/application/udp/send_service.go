// Package udpapp は UDP ユースケース層を提供する。
package udpapp

import (
	"fmt"

	cmn "github.com/f0reth/Wirexa/internal/domain"
	domain "github.com/f0reth/Wirexa/internal/domain/udp"
)

var _ domain.SendUseCase = (*UDPSendService)(nil)

// UDPSendService は UDP パケット送信ユースケースの実装。
type UDPSendService struct {
	socket domain.UDPSocket
	logger cmn.Logger
}

// NewUDPSendService は UDPSendService を生成する。
func NewUDPSendService(socket domain.UDPSocket, logger cmn.Logger) *UDPSendService {
	return &UDPSendService{socket: socket, logger: logger}
}

// Send は UDP パケットを送信して結果を返す。
func (s *UDPSendService) Send(req domain.UDPSendRequest) (domain.UDPSendResult, error) {
	if err := req.Validate(); err != nil {
		return domain.UDPSendResult{}, err
	}

	var data []byte
	var err error

	if req.Encoding == domain.EncodingFixed {
		data, err = domain.DecodeFixedLengthPayload(&req.FixedLengthPayload, req.Endianness)
	} else {
		data, err = domain.DecodePayload(req.Payload, req.Encoding, req.MessageLength)
	}

	if err != nil {
		return domain.UDPSendResult{}, err
	}

	n, err := s.socket.Send(req.Host, req.Port, data)
	if err != nil {
		s.logger.Error("UDP send failed", "source", "udp", "host", req.Host, "port", req.Port, "error", err)
		return domain.UDPSendResult{}, fmt.Errorf("failed to send UDP packet: %w", err)
	}

	s.logger.Info("UDP packet sent", "source", "udp", "host", req.Host, "port", req.Port, "bytes", n)
	return domain.UDPSendResult{BytesSent: n}, nil
}
