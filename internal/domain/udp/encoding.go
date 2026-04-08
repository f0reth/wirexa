// Package udpdomain は UDP クライアントのドメイン層を提供する。
package udpdomain

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	cmn "github.com/f0reth/Wirexa/internal/domain"
)

// EncodePayload はバイト列を指定エンコーディングで文字列化する。
func EncodePayload(data []byte, encoding PayloadEncoding) string {
	switch encoding {
	case EncodingFixed:
		return hex.EncodeToString(data)
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
	case EncodingJSON:
		if !json.Valid([]byte(payload)) {
			return nil, &cmn.ValidationError{Field: "payload", Message: "invalid JSON"}
		}
		return []byte(payload), nil
	case EncodingFixed:
		if messageLength <= 0 {
			return nil, &cmn.ValidationError{Field: "messageLength", Message: "must be > 0"}
		}
		cleaned := strings.ReplaceAll(payload, " ", "")
		data, err := hex.DecodeString(cleaned)
		if err != nil {
			return nil, &cmn.ValidationError{Field: "payload", Message: "invalid hex: " + err.Error()}
		}
		if len(data) > messageLength {
			return nil, &cmn.ValidationError{Field: "payload", Message: "payload exceeds messageLength"}
		}
		if len(data) < messageLength {
			padded := make([]byte, messageLength)
			copy(padded, data)
			data = padded
		}
		return data, nil
	default:
		return nil, &cmn.ValidationError{Field: "encoding", Message: "unknown: " + string(encoding)}
	}
}

// DecodeFixedLengthPayload は複数フィールドから単一バイト列を生成する。
func DecodeFixedLengthPayload(payload *FixedLengthPayload, endianness Endianness) ([]byte, error) {
	if payload == nil || len(payload.Fields) == 0 {
		return nil, &cmn.ValidationError{Field: "fixedLengthPayload", Message: "no fields"}
	}

	var byteOrder binary.ByteOrder
	if endianness == EndiannessBig {
		byteOrder = binary.BigEndian
	} else {
		byteOrder = binary.LittleEndian
	}

	var result []byte
	for _, field := range payload.Fields {
		data, err := encodeFixedField(field, byteOrder)
		if err != nil {
			return nil, err
		}
		result = append(result, data...)
	}
	return result, nil
}

func encodeFixedField(field FixedLengthField, byteOrder binary.ByteOrder) ([]byte, error) {
	switch field.FieldType {
	case FieldTypeString, "":
		return encodeStringField(field)
	case FieldTypeBytes:
		return encodeBytesField(field)
	case FieldTypeUint8:
		v, err := strconv.ParseUint(field.Value, 10, 8)
		if err != nil {
			return nil, fixedFieldErr(field.Name, "invalid uint8: "+err.Error())
		}
		return []byte{uint8(v)}, nil
	case FieldTypeUint16:
		v, err := strconv.ParseUint(field.Value, 10, 16)
		if err != nil {
			return nil, fixedFieldErr(field.Name, "invalid uint16: "+err.Error())
		}
		buf := make([]byte, 2)
		byteOrder.PutUint16(buf, uint16(v))
		return buf, nil
	case FieldTypeUint32:
		v, err := strconv.ParseUint(field.Value, 10, 32)
		if err != nil {
			return nil, fixedFieldErr(field.Name, "invalid uint32: "+err.Error())
		}
		buf := make([]byte, 4)
		byteOrder.PutUint32(buf, uint32(v))
		return buf, nil
	case FieldTypeUint64:
		v, err := strconv.ParseUint(field.Value, 10, 64)
		if err != nil {
			return nil, fixedFieldErr(field.Name, "invalid uint64: "+err.Error())
		}
		buf := make([]byte, 8)
		byteOrder.PutUint64(buf, v)
		return buf, nil
	case FieldTypeInt8:
		v, err := strconv.ParseInt(field.Value, 10, 8)
		if err != nil {
			return nil, fixedFieldErr(field.Name, "invalid int8: "+err.Error())
		}
		return []byte{byte(v)}, nil //nolint:gosec // ParseInt(10, 8) で -128〜127 が保証済み
	case FieldTypeInt16:
		v, err := strconv.ParseInt(field.Value, 10, 16)
		if err != nil {
			return nil, fixedFieldErr(field.Name, "invalid int16: "+err.Error())
		}
		buf := make([]byte, 2)
		byteOrder.PutUint16(buf, uint16(v)) //nolint:gosec // ParseInt(10, 16) でビット幅が保証済み
		return buf, nil
	case FieldTypeInt32:
		v, err := strconv.ParseInt(field.Value, 10, 32)
		if err != nil {
			return nil, fixedFieldErr(field.Name, "invalid int32: "+err.Error())
		}
		buf := make([]byte, 4)
		byteOrder.PutUint32(buf, uint32(v)) //nolint:gosec // ParseInt(10, 32) でビット幅が保証済み
		return buf, nil
	case FieldTypeInt64:
		v, err := strconv.ParseInt(field.Value, 10, 64)
		if err != nil {
			return nil, fixedFieldErr(field.Name, "invalid int64: "+err.Error())
		}
		buf := make([]byte, 8)
		byteOrder.PutUint64(buf, uint64(v)) //nolint:gosec // ParseInt(10, 64) でビット幅が保証済み
		return buf, nil
	case FieldTypeFloat32:
		v, err := strconv.ParseFloat(field.Value, 32)
		if err != nil {
			return nil, fixedFieldErr(field.Name, "invalid float32: "+err.Error())
		}
		buf := make([]byte, 4)
		byteOrder.PutUint32(buf, math.Float32bits(float32(v)))
		return buf, nil
	case FieldTypeFloat64:
		v, err := strconv.ParseFloat(field.Value, 64)
		if err != nil {
			return nil, fixedFieldErr(field.Name, "invalid float64: "+err.Error())
		}
		buf := make([]byte, 8)
		byteOrder.PutUint64(buf, math.Float64bits(v))
		return buf, nil
	default:
		return nil, fixedFieldErr(field.Name, "unknown field type: "+string(field.FieldType))
	}
}

func encodeStringField(field FixedLengthField) ([]byte, error) {
	if field.Length <= 0 {
		return nil, &cmn.ValidationError{
			Field:   "fixedLengthPayload",
			Message: fmt.Sprintf("field '%s': length must be > 0", field.Name),
		}
	}
	data := make([]byte, 0, len(field.Value))
	for _, r := range field.Value {
		if r > 0x7F {
			return nil, &cmn.ValidationError{
				Field:   "fixedLengthPayload",
				Message: fmt.Sprintf("field '%s': character '%c' is not single-byte (ASCII only)", field.Name, r),
			}
		}
		data = append(data, byte(r)) //nolint:gosec // r <= 0x7F が保証済み
	}
	if len(data) > field.Length {
		return nil, &cmn.ValidationError{
			Field:   "fixedLengthPayload",
			Message: fmt.Sprintf("field '%s': data (%d bytes) exceeds length %d", field.Name, len(data), field.Length),
		}
	}
	if len(data) < field.Length {
		padded := make([]byte, field.Length)
		copy(padded, data)
		data = padded
	}
	return data, nil
}

func encodeBytesField(field FixedLengthField) ([]byte, error) {
	if field.Length <= 0 {
		return nil, &cmn.ValidationError{
			Field:   "fixedLengthPayload",
			Message: fmt.Sprintf("field '%s': length must be > 0", field.Name),
		}
	}
	cleaned := strings.ReplaceAll(field.Value, " ", "")
	data, err := hex.DecodeString(cleaned)
	if err != nil {
		return nil, &cmn.ValidationError{
			Field:   "fixedLengthPayload",
			Message: fmt.Sprintf("field '%s': invalid hex: %s", field.Name, err.Error()),
		}
	}
	if len(data) > field.Length {
		return nil, &cmn.ValidationError{
			Field:   "fixedLengthPayload",
			Message: fmt.Sprintf("field '%s': data (%d bytes) exceeds length %d", field.Name, len(data), field.Length),
		}
	}
	if len(data) < field.Length {
		padded := make([]byte, field.Length)
		copy(padded, data)
		data = padded
	}
	return data, nil
}

func fixedFieldErr(name, message string) *cmn.ValidationError {
	return &cmn.ValidationError{
		Field:   "fixedLengthPayload",
		Message: fmt.Sprintf("field '%s': %s", name, message),
	}
}
