package udpdomain

import (
	"encoding/base64"
	"encoding/hex"
	"strings"
)

// EncodePayload はバイト列を指定エンコーディングで文字列化する。
func EncodePayload(data []byte, encoding PayloadEncoding) string {
	switch encoding {
	case EncodingHex:
		return hex.EncodeToString(data)
	case EncodingBase64:
		return base64.StdEncoding.EncodeToString(data)
	default:
		return string(data)
	}
}

// DecodePayload は文字列を指定エンコーディングでバイト列に変換する。
func DecodePayload(payload string, encoding PayloadEncoding) ([]byte, error) {
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
	default:
		return nil, &ValidationError{Field: "encoding", Message: "unknown: " + string(encoding)}
	}
}
