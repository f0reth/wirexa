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

const (
	fieldPayload            = "payload"
	fieldFixedLengthPayload = "fixedLengthPayload"
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
			return nil, &cmn.ValidationError{Field: fieldPayload, Message: "invalid JSON"}
		}
		return []byte(payload), nil
	case EncodingFixed:
		if messageLength <= 0 {
			return nil, &cmn.ValidationError{Field: "messageLength", Message: "must be > 0"}
		}
		cleaned := strings.ReplaceAll(payload, " ", "")
		data, err := hex.DecodeString(cleaned)
		if err != nil {
			return nil, &cmn.ValidationError{Field: fieldPayload, Message: "invalid hex: " + err.Error()}
		}
		if len(data) > messageLength {
			return nil, &cmn.ValidationError{Field: fieldPayload, Message: "payload exceeds messageLength"}
		}
		return padTo(data, messageLength), nil
	default:
		return nil, &cmn.ValidationError{Field: "encoding", Message: "unknown: " + string(encoding)}
	}
}

// DecodeFixedLengthPayload は複数フィールドから単一バイト列を生成する。
func DecodeFixedLengthPayload(payload *FixedLengthPayload, endianness Endianness) ([]byte, error) {
	if payload == nil || len(payload.Fields) == 0 {
		return nil, &cmn.ValidationError{Field: fieldFixedLengthPayload, Message: "no fields"}
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

// numericKind は数値型フィールドの符号種別を表す。parse の呼び分けに使う。
type numericKind int

const (
	kindUint numericKind = iota
	kindInt
	kindFloat
)

// numericSpec は数値型フィールドのバイトサイズと符号種別を保持する。
type numericSpec struct {
	size int
	kind numericKind
}

// numericFieldSpecs は数値型フィールドのバイトサイズと符号種別を定義する唯一のテーブル。
// encodeNumericField と FieldTypeByteSize の双方がこれを読む。
var numericFieldSpecs = map[FieldType]numericSpec{
	FieldTypeUint8:   {size: 1, kind: kindUint},
	FieldTypeUint16:  {size: 2, kind: kindUint},
	FieldTypeUint32:  {size: 4, kind: kindUint},
	FieldTypeUint64:  {size: 8, kind: kindUint},
	FieldTypeInt8:    {size: 1, kind: kindInt},
	FieldTypeInt16:   {size: 2, kind: kindInt},
	FieldTypeInt32:   {size: 4, kind: kindInt},
	FieldTypeInt64:   {size: 8, kind: kindInt},
	FieldTypeFloat32: {size: 4, kind: kindFloat},
	FieldTypeFloat64: {size: 8, kind: kindFloat},
}

func encodeFixedField(field FixedLengthField, byteOrder binary.ByteOrder) ([]byte, error) {
	switch field.FieldType {
	case FieldTypeString, "":
		return encodeVariableField(field, decodeASCII)
	case FieldTypeBytes:
		return encodeVariableField(field, decodeHex)
	}
	if spec, ok := numericFieldSpecs[field.FieldType]; ok {
		return encodeNumericField(field, spec, byteOrder)
	}
	return nil, fixedFieldErr(field.Name, "unknown field type: "+string(field.FieldType))
}

// encodeNumericField は数値型フィールドを spec.size バイトのバイト列に変換する。
// field.Length は数値型では参照しない (サイズは型が決める)。
func encodeNumericField(field FixedLengthField, spec numericSpec, byteOrder binary.ByteOrder) ([]byte, error) {
	bits := spec.size * 8
	var raw uint64
	switch spec.kind {
	case kindUint:
		v, err := strconv.ParseUint(field.Value, 10, bits)
		if err != nil {
			return nil, invalidValueErr(field, err)
		}
		raw = v
	case kindInt:
		v, err := strconv.ParseInt(field.Value, 10, bits)
		if err != nil {
			return nil, invalidValueErr(field, err)
		}
		// 負値は 2 の補数として下位ビットに残り、putUint の切り詰めで型幅に収まる
		raw = uint64(v) //nolint:gosec // ビットパターンの再解釈が目的であり値の大小は意味を持たない
	case kindFloat:
		v, err := strconv.ParseFloat(field.Value, bits)
		if err != nil {
			return nil, invalidValueErr(field, err)
		}
		if spec.size == 4 {
			raw = uint64(math.Float32bits(float32(v)))
		} else {
			raw = math.Float64bits(v)
		}
	}
	return putUint(raw, spec.size, byteOrder), nil
}

// invalidValueErr は parse 失敗を "field 'x': invalid uint16: ..." 形式のエラーにする。
// 型名は FieldType の文字列値をそのまま使う。
func invalidValueErr(field FixedLengthField, err error) *cmn.ValidationError {
	return fixedFieldErr(field.Name, "invalid "+string(field.FieldType)+": "+err.Error())
}

// putUint は raw の下位 size バイトを byteOrder に従って書き出す。
// size は numericFieldSpecs の値 (1/2/4/8) のみを想定する。
func putUint(raw uint64, size int, byteOrder binary.ByteOrder) []byte {
	buf := make([]byte, size)
	switch size {
	case 1:
		buf[0] = byte(raw) //nolint:gosec // bitSize=8 の parse 結果であり下位 8 ビットのみが有効
	case 2:
		byteOrder.PutUint16(buf, uint16(raw)) //nolint:gosec // bitSize=16 の parse 結果でビット幅が保証済み
	case 4:
		byteOrder.PutUint32(buf, uint32(raw)) //nolint:gosec // bitSize=32 の parse 結果でビット幅が保証済み
	case 8:
		byteOrder.PutUint64(buf, raw)
	}
	return buf
}

// encodeVariableField は可変長フィールド (string / bytes) を field.Length バイトに整える。
// 型ごとの差分は decode のみ。
func encodeVariableField(field FixedLengthField, decode func(FixedLengthField) ([]byte, error)) ([]byte, error) {
	if field.Length <= 0 {
		return nil, fixedFieldErr(field.Name, "length must be > 0")
	}
	data, err := decode(field)
	if err != nil {
		return nil, err
	}
	if len(data) > field.Length {
		return nil, fixedFieldErr(field.Name, fmt.Sprintf("data (%d bytes) exceeds length %d", len(data), field.Length))
	}
	return padTo(data, field.Length), nil
}

func decodeASCII(field FixedLengthField) ([]byte, error) {
	data := make([]byte, 0, len(field.Value))
	for _, r := range field.Value {
		if r > 0x7F {
			return nil, fixedFieldErr(field.Name, fmt.Sprintf("character '%c' is not single-byte (ASCII only)", r))
		}
		data = append(data, byte(r)) //nolint:gosec // r <= 0x7F が保証済み
	}
	return data, nil
}

func decodeHex(field FixedLengthField) ([]byte, error) {
	cleaned := strings.ReplaceAll(field.Value, " ", "")
	data, err := hex.DecodeString(cleaned)
	if err != nil {
		return nil, fixedFieldErr(field.Name, "invalid hex: "+err.Error())
	}
	return data, nil
}

// padTo は data を length バイトまで 0x00 で右パディングする。
// len(data) >= length なら data をそのまま返す。
func padTo(data []byte, length int) []byte {
	if len(data) >= length {
		return data
	}
	padded := make([]byte, length)
	copy(padded, data)
	return padded
}

func fixedFieldErr(name, message string) *cmn.ValidationError {
	return &cmn.ValidationError{
		Field:   fieldFixedLengthPayload,
		Message: fmt.Sprintf("field '%s': %s", name, message),
	}
}
