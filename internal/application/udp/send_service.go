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
	if req.Host == "" {
		return domain.UdpSendResult{}, &domain.ValidationError{Field: "host", Message: "required"}
	}
	if req.Port < 1 || req.Port > 65535 {
		return domain.UdpSendResult{}, &domain.ValidationError{Field: "port", Message: "must be 1-65535"}
	}

	var data []byte
	var err error

	if req.Encoding == domain.EncodingFixed {
		data, err = domain.DecodeFixedLengthPayload(&req.FixedLengthPayload)
	} else {
		return domain.UdpSendResult{}, &domain.ValidationError{
			Field:   "encoding",
			Message: "only 'fixed' encoding is currently supported",
		}
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
