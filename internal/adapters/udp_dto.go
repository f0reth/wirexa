package adapters

import udpdomain "github.com/f0reth/Wirexa/internal/domain/udp"

// UdpTarget は UDP 送信先プリセットの RPC 転送型。
type UdpTarget struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Host string `json:"host"`
	Port int    `json:"port"`
}

// FixedLengthField は固定長フィールドの RPC 転送型。
type FixedLengthField struct {
	Name      string `json:"name"`
	FieldType string `json:"fieldType"`
	Length    int    `json:"length"`
	Value     string `json:"value"`
}

// FixedLengthPayload は固定長ペイロードの RPC 転送型。
type FixedLengthPayload struct {
	Fields []FixedLengthField `json:"fields"`
}

// UdpSendRequest は UDP 送信リクエストの RPC 転送型。
type UdpSendRequest struct {
	Host               string             `json:"host"`
	Port               int                `json:"port"`
	Encoding           string             `json:"encoding"`
	Payload            string             `json:"payload"`
	MessageLength      int                `json:"messageLength"`
	FixedLengthPayload FixedLengthPayload `json:"fixedLengthPayload"`
	Endianness         string             `json:"endianness"`
}

// UdpSendResult は UDP 送信結果の RPC 転送型。
type UdpSendResult struct {
	BytesSent int `json:"bytesSent"`
}

// UdpListenSession は UDP リスニングセッションの RPC 転送型。
type UdpListenSession struct {
	ID       string `json:"id"`
	Port     int    `json:"port"`
	Encoding string `json:"encoding"`
}

func fromUdpSendRequestDTO(req UdpSendRequest) udpdomain.UdpSendRequest {
	fields := make([]udpdomain.FixedLengthField, len(req.FixedLengthPayload.Fields))
	for i, f := range req.FixedLengthPayload.Fields {
		fields[i] = udpdomain.FixedLengthField{
			Name:      f.Name,
			FieldType: udpdomain.FieldType(f.FieldType),
			Length:    f.Length,
			Value:     f.Value,
		}
	}
	return udpdomain.UdpSendRequest{
		Host:               req.Host,
		Port:               req.Port,
		Encoding:           udpdomain.PayloadEncoding(req.Encoding),
		Payload:            req.Payload,
		MessageLength:      req.MessageLength,
		FixedLengthPayload: udpdomain.FixedLengthPayload{Fields: fields},
		Endianness:         udpdomain.Endianness(req.Endianness),
	}
}

func toUdpSendResultDTO(res udpdomain.UdpSendResult) UdpSendResult {
	return UdpSendResult{BytesSent: res.BytesSent}
}

func fromUdpTargetDTO(t UdpTarget) udpdomain.UdpTarget {
	return udpdomain.UdpTarget{ID: t.ID, Name: t.Name, Host: t.Host, Port: t.Port}
}

func toUdpTargetDTO(t udpdomain.UdpTarget) UdpTarget {
	return UdpTarget{ID: t.ID, Name: t.Name, Host: t.Host, Port: t.Port}
}

func toUdpListenSessionDTO(s udpdomain.UdpListenSession) UdpListenSession {
	return UdpListenSession{ID: s.ID, Port: s.Port, Encoding: string(s.Encoding)}
}
