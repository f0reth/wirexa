package udpdomain

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"strings"
)

// EncodePayload はバイト列を指定エンコーディングで文字列化する。
func EncodePayload(data []byte, encoding PayloadEncoding) string {
	switch encoding {
	case EncodingHex, EncodingFixed:
		return hex.EncodeToString(data)
	case EncodingBase64:
		return base64.StdEncoding.EncodeToString(data)
	case EncodingJSON:
		var buf bytes.Buffer
		if err := json.Indent(&buf, data, "", "  "); err != nil {
			return string(data)
		}
		return buf.String()
	default:
		return string(data)
	}
}

// DecodePayload は文字列を指定エンコーディングでバイト列に変換する。
// messageLength は EncodingFixed 時のみ使用する。それ以外では無視される。
func DecodePayload(payload string, encoding PayloadEncoding, messageLength int) ([]byte, error) {
	switch encoding {
	case EncodingText:
		return []byte(payload), nil
	case EncodingHex:
		cleaned := strings.ReplaceAll(payload, " ", "")
		data, err := hex.DecodeString(cleaned)
		if err != nil {
			return nil, &ValidationError{Field: "payload", Message: "invalid hex: " + err.Error()}
		}
		return data, nil
	case EncodingBase64:
		data, err := base64.StdEncoding.DecodeString(payload)
		if err != nil {
			return nil, &ValidationError{Field: "payload", Message: "invalid base64: " + err.Error()}
		}
		return data, nil
	case EncodingJSON:
		if !json.Valid([]byte(payload)) {
			return nil, &ValidationError{Field: "payload", Message: "invalid JSON"}
		}
		return []byte(payload), nil
	case EncodingFixed:
		if messageLength <= 0 {
			return nil, &ValidationError{Field: "messageLength", Message: "must be > 0"}
		}
		cleaned := strings.ReplaceAll(payload, " ", "")
		data, err := hex.DecodeString(cleaned)
		if err != nil {
			return nil, &ValidationError{Field: "payload", Message: "invalid hex: " + err.Error()}
		}
		if len(data) > messageLength {
			return nil, &ValidationError{Field: "payload", Message: "payload exceeds messageLength"}
		}
		if len(data) < messageLength {
			padded := make([]byte, messageLength)
			copy(padded, data)
			data = padded
		}
		return data, nil
	default:
		return nil, &ValidationError{Field: "encoding", Message: "unknown: " + string(encoding)}
	}
}
