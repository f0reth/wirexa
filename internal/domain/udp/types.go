package udpdomain

// PayloadEncoding はペイロードのエンコーディング形式を表す。
type PayloadEncoding string

// エンコーディング種別定数。
const (
	EncodingText   PayloadEncoding = "text"
	EncodingHex    PayloadEncoding = "hex"
	EncodingBase64 PayloadEncoding = "base64"
)

// UdpTarget は保存可能な送信先プリセットを表す。
type UdpTarget struct {
	ID       string          `json:"id"`
	Name     string          `json:"name"`
	Host     string          `json:"host"`
	Port     int             `json:"port"`
	Encoding PayloadEncoding `json:"encoding"`
}

// UdpSendRequest は UDP 送信リクエストを表す。
type UdpSendRequest struct {
	Host     string          `json:"host"`
	Port     int             `json:"port"`
	Payload  string          `json:"payload"`
	Encoding PayloadEncoding `json:"encoding"`
}

// UdpSendResult は UDP 送信結果を表す。
type UdpSendResult struct {
	BytesSent int `json:"bytesSent"`
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
