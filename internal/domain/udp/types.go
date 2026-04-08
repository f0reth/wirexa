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

// UdpTarget は保存可能な送信先プリセットを表す。
type UdpTarget struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Host string `json:"host"`
	Port int    `json:"port"`
}

// FixedLengthField は固定長フィールドを表す。
type FixedLengthField struct {
	Name      string    `json:"name"`
	FieldType FieldType `json:"fieldType"`
	Length    int       `json:"length"` // FieldTypeString, FieldTypeBytes のみ使用
	Value     string    `json:"value"`
}

// FixedLengthPayload は複数フィールドで構成されるペイロード。
type FixedLengthPayload struct {
	Fields []FixedLengthField `json:"fields"`
}

// UdpSendRequest は UDP 送信リクエストを表す。
type UdpSendRequest struct {
	Host               string             `json:"host"`
	Port               int                `json:"port"`
	Encoding           PayloadEncoding    `json:"encoding"`
	Payload            string             `json:"payload"`
	MessageLength      int                `json:"messageLength"`
	FixedLengthPayload FixedLengthPayload `json:"fixedLengthPayload"`
	Endianness         Endianness         `json:"endianness"`
}

// UdpSendResult は UDP 送信結果を表す。
type UdpSendResult struct {
	BytesSent int `json:"bytesSent"`
}

// Validate は UdpSendRequest のドメイン不変条件を検証する。
func (r *UdpSendRequest) Validate() error {
	if r.Host == "" {
		return &cmn.ValidationError{Field: "host", Message: "required"}
	}
	if r.Port < 1 || r.Port > 65535 {
		return &cmn.ValidationError{Field: "port", Message: "must be 1-65535"}
	}
	return nil
}

// UdpListenSession はアクティブなリスニングセッションを表す。
type UdpListenSession struct {
	ID       string          `json:"id"`
	Port     int             `json:"port"`
	Encoding PayloadEncoding `json:"encoding"`
}

// UdpReceivedMessage は受信した UDP パケットを表す。
type UdpReceivedMessage struct {
	SessionID  string          `json:"sessionId"`
	Port       int             `json:"port"`
	RemoteAddr string          `json:"remoteAddr"`
	Payload    string          `json:"payload"`
	Encoding   PayloadEncoding `json:"encoding"`
	Timestamp  int64           `json:"timestamp"`
}
