package udpdomain

// PayloadEncoding はペイロードのエンコーディング形式を表す。
type PayloadEncoding string

// エンコーディング種別定数。
const (
	EncodingText   PayloadEncoding = "text"
	EncodingHex    PayloadEncoding = "hex"
	EncodingBase64 PayloadEncoding = "base64"
	EncodingJSON   PayloadEncoding = "json"
	EncodingFixed  PayloadEncoding = "fixed"
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
	Name   string `json:"name"`
	Length int    `json:"length"`
	Value  string `json:"value"`
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
}

// UdpSendResult は UDP 送信結果を表す。
type UdpSendResult struct {
	BytesSent int `json:"bytesSent"`
}

// Validate は UdpSendRequest のドメイン不変条件を検証する。
func (r *UdpSendRequest) Validate() error {
	if r.Host == "" {
		return &ValidationError{Field: "host", Message: "required"}
	}
	if r.Port < 1 || r.Port > 65535 {
		return &ValidationError{Field: "port", Message: "must be 1-65535"}
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
