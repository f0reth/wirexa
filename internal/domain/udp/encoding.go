package udpdomain

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
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

// DecodeFixedLengthPayload は複数フィールドから単一バイト列を生成する。
func DecodeFixedLengthPayload(payload *FixedLengthPayload) ([]byte, error) {
	if payload == nil || len(payload.Fields) == 0 {
		return nil, &ValidationError{Field: "fixedLengthPayload", Message: "no fields"}
	}

	var result []byte

	for _, field := range payload.Fields {
		if field.Length <= 0 {
			return nil, &ValidationError{
				Field:   "fixedLengthPayload",
				Message: fmt.Sprintf("field '%s': length must be > 0", field.Name),
			}
		}

		// UTF-8文字列をバイト列に変換（1文字=1byte、ASCII範囲のみ許可）
		data := make([]byte, 0, len(field.Value))
		for _, r := range field.Value {
			if r > 0x7F {
				return nil, &ValidationError{
					Field:   "fixedLengthPayload",
					Message: fmt.Sprintf("field '%s': character '%c' is not single-byte (ASCII only)", field.Name, r),
				}
			}
			data = append(data, byte(r)) //nolint:gosec // r <= 0x7F が保証済み
		}

		// 長さ検証とパディング
		if len(data) > field.Length {
			return nil, &ValidationError{
				Field:   "fixedLengthPayload",
				Message: fmt.Sprintf("field '%s': data (%d bytes) exceeds length %d", field.Name, len(data), field.Length),
			}
		}

		if len(data) < field.Length {
			padded := make([]byte, field.Length)
			copy(padded, data)
			data = padded
		}

		result = append(result, data...)
	}

	return result, nil
}
