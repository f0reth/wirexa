package udpdomain

import cmn "github.com/f0reth/Wirexa/internal/domain"

// PayloadEncoding はペイロードのエンコーディング形式を表す。
type PayloadEncoding string

// エンコーディング種別定数。
const (
	EncodingText  PayloadEncoding = "text"
	EncodingJSON  PayloadEncoding = "json"
	EncodingFixed PayloadEncoding = "fixed"
)

// FieldType は fixed encoding フィールドのデータ型を表す。
type FieldType string

// FieldType 定数。
const (
	FieldTypeString  FieldType = "string"
	FieldTypeBytes   FieldType = "bytes"
	FieldTypeUint8   FieldType = "uint8"
	FieldTypeUint16  FieldType = "uint16"
	FieldTypeUint32  FieldType = "uint32"
	FieldTypeUint64  FieldType = "uint64"
	FieldTypeInt8    FieldType = "int8"
	FieldTypeInt16   FieldType = "int16"
	FieldTypeInt32   FieldType = "int32"
	FieldTypeInt64   FieldType = "int64"
	FieldTypeFloat32 FieldType = "float32"
	FieldTypeFloat64 FieldType = "float64"
)

// FieldTypeByteSize は数値型の固定バイトサイズを返す。可変長型（string, bytes）は -1 を返す。
func FieldTypeByteSize(t FieldType) int {
	switch t {
	case FieldTypeUint8, FieldTypeInt8:
		return 1
	case FieldTypeUint16, FieldTypeInt16:
		return 2
	case FieldTypeUint32, FieldTypeInt32, FieldTypeFloat32:
		return 4
	case FieldTypeUint64, FieldTypeInt64, FieldTypeFloat64:
		return 8
	default:
		return -1
	}
}

// Endianness はバイトオーダーを表す。
type Endianness string

// Endianness 定数。
const (
	EndiannessBig    Endianness = "big"
	EndiannessLittle Endianness = "little"
)

// UDPTarget は保存可能な送信先プリセットを表す。
type UDPTarget struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Host string `json:"host"`
	Port int    `json:"port"`
}

// Validate は UDPTarget のドメイン不変条件を検証する。
// UDPSendRequest.Validate と同じ host/port 規則を適用する。
func (t *UDPTarget) Validate() error {
	if t.Host == "" {
		return &cmn.ValidationError{Field: "host", Message: cmn.MsgRequired}
	}
	if t.Port < 1 || t.Port > 65535 {
		return &cmn.ValidationError{Field: "port", Message: "must be 1-65535"}
	}
	return nil
}

// FixedLengthField は固定長フィールドを表す。
type FixedLengthField struct {
	Name      string    `json:"name"`
	FieldType FieldType `json:"fieldType"`
	Value     string    `json:"value"`
	Length    int       `json:"length"`
}

// FixedLengthPayload は複数フィールドで構成されるペイロード。
type FixedLengthPayload struct {
	Fields []FixedLengthField `json:"fields"`
}

// UDPSendRequest は UDP 送信リクエストを表す。
type UDPSendRequest struct {
	Host               string             `json:"host"`
	Encoding           PayloadEncoding    `json:"encoding"`
	Payload            string             `json:"payload"`
	Endianness         Endianness         `json:"endianness"`
	FixedLengthPayload FixedLengthPayload `json:"fixedLengthPayload"`
	Port               int                `json:"port"`
	MessageLength      int                `json:"messageLength"`
}

// UDPSendResult は UDP 送信結果を表す。
type UDPSendResult struct {
	BytesSent int `json:"bytesSent"`
}

// Validate は UDPSendRequest のドメイン不変条件を検証する。
func (r *UDPSendRequest) Validate() error {
	if r.Host == "" {
		return &cmn.ValidationError{Field: "host", Message: cmn.MsgRequired}
	}
	if r.Port < 1 || r.Port > 65535 {
		return &cmn.ValidationError{Field: "port", Message: "must be 1-65535"}
	}
	return nil
}

// UDPListenSession はアクティブなリスニングセッションを表す。
type UDPListenSession struct {
	ID       string          `json:"id"`
	Encoding PayloadEncoding `json:"encoding"`
	Port     int             `json:"port"`
}

// UDPReceivedMessage は受信した UDP パケットを表す。
type UDPReceivedMessage struct {
	SessionID  string          `json:"sessionId"`
	RemoteAddr string          `json:"remoteAddr"`
	Payload    string          `json:"payload"`
	Encoding   PayloadEncoding `json:"encoding"`
	Port       int             `json:"port"`
	Timestamp  int64           `json:"timestamp"`
}
