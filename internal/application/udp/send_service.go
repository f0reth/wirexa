// Package udpapp は UDP ユースケース層を提供する。
package udpapp

import (
	domain "github.com/f0reth/Wirexa/internal/domain/udp"
)

var _ domain.SendUseCase = (*UdpSendService)(nil)

// UdpSendService は UDP パケット送信ユースケースの実装。
type UdpSendService struct {
	socket domain.UdpSocket
}

// NewUdpSendService は UdpSendService を生成する。
func NewUdpSendService(socket domain.UdpSocket) *UdpSendService {
	return &UdpSendService{socket: socket}
}

// Send は UDP パケットを送信して結果を返す。
func (s *UdpSendService) Send(req domain.UdpSendRequest) (domain.UdpSendResult, error) {
	if err := req.Validate(); err != nil {
		return domain.UdpSendResult{}, err
	}

	var data []byte
	var err error

	if req.Encoding == domain.EncodingFixed {
		data, err = domain.DecodeFixedLengthPayload(&req.FixedLengthPayload)
	} else {
		data, err = domain.DecodePayload(req.Payload, req.Encoding, req.MessageLength)
	}

	if err != nil {
		return domain.UdpSendResult{}, err
	}

	n, err := s.socket.Send(req.Host, req.Port, data)
	if err != nil {
		return domain.UdpSendResult{}, err
	}

	return domain.UdpSendResult{BytesSent: n}, nil
}
