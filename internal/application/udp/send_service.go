// Package udpapp は UDP ユースケース層を提供する。
package udpapp

import (
	cmn "github.com/f0reth/Wirexa/internal/domain"
	domain "github.com/f0reth/Wirexa/internal/domain/udp"
)

var _ domain.SendUseCase = (*UdpSendService)(nil)

// UdpSendService は UDP パケット送信ユースケースの実装。
type UdpSendService struct {
	socket domain.UdpSocket
	logger cmn.Logger
}

// NewUdpSendService は UdpSendService を生成する。
func NewUdpSendService(socket domain.UdpSocket, logger cmn.Logger) *UdpSendService {
	return &UdpSendService{socket: socket, logger: logger}
}

// Send は UDP パケットを送信して結果を返す。
func (s *UdpSendService) Send(req domain.UdpSendRequest) (domain.UdpSendResult, error) {
	if err := req.Validate(); err != nil {
		return domain.UdpSendResult{}, err
	}

	var data []byte
	var err error

	if req.Encoding == domain.EncodingFixed {
		data, err = domain.DecodeFixedLengthPayload(&req.FixedLengthPayload, req.Endianness)
	} else {
		data, err = domain.DecodePayload(req.Payload, req.Encoding, req.MessageLength)
	}

	if err != nil {
		return domain.UdpSendResult{}, err
	}

	n, err := s.socket.Send(req.Host, req.Port, data)
	if err != nil {
		s.logger.Error("UDP send failed", "source", "udp", "host", req.Host, "port", req.Port, "error", err)
		return domain.UdpSendResult{}, err
	}

	s.logger.Info("UDP packet sent", "source", "udp", "host", req.Host, "port", req.Port, "bytes", n)
	return domain.UdpSendResult{BytesSent: n}, nil
}
