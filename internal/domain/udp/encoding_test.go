package udpdomain

import (
	"errors"
	"testing"

	cmn "github.com/f0reth/Wirexa/internal/domain"
)

func TestDecodeFixedLengthPayload_SingleField(t *testing.T) {
	tests := []struct {
		name    string
		payload *FixedLengthPayload
		want    []byte
		wantErr bool
	}{
		{
			name: "single field with exact length",
			payload: &FixedLengthPayload{
				Fields: []FixedLengthField{
					{Name: "field1", Length: 5, Value: "hello"},
				},
			},
			want:    []byte{'h', 'e', 'l', 'l', 'o'},
			wantErr: false,
		},
		{
			name: "single field with padding",
			payload: &FixedLengthPayload{
				Fields: []FixedLengthField{
					{Name: "field1", Length: 4, Value: "hi"},
				},
			},
			want:    []byte{'h', 'i', 0x00, 0x00},
			wantErr: false,
		},
		{
			name: "single field exceeds length",
			payload: &FixedLengthPayload{
				Fields: []FixedLengthField{
					{Name: "field1", Length: 2, Value: "hello"},
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "empty field value",
			payload: &FixedLengthPayload{
				Fields: []FixedLengthField{
					{Name: "field1", Length: 2, Value: ""},
				},
			},
			want:    []byte{0x00, 0x00},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DecodeFixedLengthPayload(tt.payload, EndiannessBig)
			if (err != nil) != tt.wantErr {
				t.Errorf("DecodeFixedLengthPayload() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !bytesEqual(got, tt.want) {
				t.Errorf("DecodeFixedLengthPayload() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDecodeFixedLengthPayload_MultipleFields(t *testing.T) {
	tests := []struct {
		name    string
		payload *FixedLengthPayload
		want    []byte
		wantErr bool
	}{
		{
			name: "multiple fields exact lengths",
			payload: &FixedLengthPayload{
				Fields: []FixedLengthField{
					{Name: "header", Length: 3, Value: "HDR"},
					{Name: "payload", Length: 4, Value: "DATA"},
				},
			},
			want:    []byte{'H', 'D', 'R', 'D', 'A', 'T', 'A'},
			wantErr: false,
		},
		{
			name: "multiple fields with padding",
			payload: &FixedLengthPayload{
				Fields: []FixedLengthField{
					{Name: "field1", Length: 4, Value: "AB"},
					{Name: "field2", Length: 2, Value: "C"},
				},
			},
			want:    []byte{'A', 'B', 0x00, 0x00, 'C', 0x00},
			wantErr: false,
		},
		{
			name: "three fields",
			payload: &FixedLengthPayload{
				Fields: []FixedLengthField{
					{Name: "f1", Length: 1, Value: "X"},
					{Name: "f2", Length: 1, Value: "Y"},
					{Name: "f3", Length: 1, Value: "Z"},
				},
			},
			want:    []byte{'X', 'Y', 'Z'},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DecodeFixedLengthPayload(tt.payload, EndiannessBig)
			if (err != nil) != tt.wantErr {
				t.Errorf("DecodeFixedLengthPayload() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !bytesEqual(got, tt.want) {
				t.Errorf("DecodeFixedLengthPayload() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDecodeFixedLengthPayload_ErrorCases(t *testing.T) {
	tests := []struct {
		payload *FixedLengthPayload
		name    string
		wantErr bool
	}{
		{
			name:    "nil payload",
			payload: nil,
			wantErr: true,
		},
		{
			name:    "empty fields",
			payload: &FixedLengthPayload{Fields: []FixedLengthField{}},
			wantErr: true,
		},
		{
			name: "invalid length (<=0)",
			payload: &FixedLengthPayload{
				Fields: []FixedLengthField{
					{Name: "field1", Length: 0, Value: "hello"},
				},
			},
			wantErr: true,
		},
		{
			name: "non-ASCII multibyte character",
			payload: &FixedLengthPayload{
				Fields: []FixedLengthField{
					{Name: "field1", Length: 4, Value: "あ"},
				},
			},
			wantErr: true,
		},
		{
			name: "value exceeds field length",
			payload: &FixedLengthPayload{
				Fields: []FixedLengthField{
					{Name: "field1", Length: 2, Value: "hello"},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DecodeFixedLengthPayload(tt.payload, EndiannessBig)
			if (err != nil) != tt.wantErr {
				t.Errorf("DecodeFixedLengthPayload() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDecodeFixedLengthPayload_NumericTypes(t *testing.T) {
	tests := []struct {
		name       string
		field      FixedLengthField
		endianness Endianness
		want       []byte
		wantErr    bool
	}{
		{
			name:       "uint8",
			field:      FixedLengthField{Name: "f", FieldType: FieldTypeUint8, Value: "255"},
			endianness: EndiannessBig,
			want:       []byte{0xFF},
		},
		{
			name:       "uint16 big-endian",
			field:      FixedLengthField{Name: "f", FieldType: FieldTypeUint16, Value: "256"},
			endianness: EndiannessBig,
			want:       []byte{0x01, 0x00},
		},
		{
			name:       "uint16 little-endian",
			field:      FixedLengthField{Name: "f", FieldType: FieldTypeUint16, Value: "256"},
			endianness: EndiannessLittle,
			want:       []byte{0x00, 0x01},
		},
		{
			name:       "uint32 big-endian",
			field:      FixedLengthField{Name: "f", FieldType: FieldTypeUint32, Value: "1"},
			endianness: EndiannessBig,
			want:       []byte{0x00, 0x00, 0x00, 0x01},
		},
		{
			name:       "int8 negative",
			field:      FixedLengthField{Name: "f", FieldType: FieldTypeInt8, Value: "-1"},
			endianness: EndiannessBig,
			want:       []byte{0xFF},
		},
		{
			name:       "int16 big-endian negative",
			field:      FixedLengthField{Name: "f", FieldType: FieldTypeInt16, Value: "-1"},
			endianness: EndiannessBig,
			want:       []byte{0xFF, 0xFF},
		},
		{
			name:       "bytes type",
			field:      FixedLengthField{Name: "f", FieldType: FieldTypeBytes, Length: 3, Value: "0a1b2c"},
			endianness: EndiannessBig,
			want:       []byte{0x0A, 0x1B, 0x2C},
		},
		{
			name:       "uint8 overflow",
			field:      FixedLengthField{Name: "f", FieldType: FieldTypeUint8, Value: "256"},
			endianness: EndiannessBig,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := &FixedLengthPayload{Fields: []FixedLengthField{tt.field}}
			got, err := DecodeFixedLengthPayload(payload, tt.endianness)
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !bytesEqual(got, tt.want) {
				t.Errorf("got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDecodeFixedLengthPayload_NumericTypes_Int32_Int64_Float(t *testing.T) {
	tests := []struct {
		name       string
		endianness Endianness
		field      FixedLengthField
		wantLen    int
		wantErr    bool
	}{
		{
			name:       "int32 big-endian",
			field:      FixedLengthField{Name: "f", FieldType: FieldTypeInt32, Value: "-1"},
			endianness: EndiannessBig,
			wantLen:    4,
		},
		{
			name:       "int64 big-endian",
			field:      FixedLengthField{Name: "f", FieldType: FieldTypeInt64, Value: "1"},
			endianness: EndiannessBig,
			wantLen:    8,
		},
		{
			name:       "float32 big-endian",
			field:      FixedLengthField{Name: "f", FieldType: FieldTypeFloat32, Value: "1.5"},
			endianness: EndiannessBig,
			wantLen:    4,
		},
		{
			name:       "float64 big-endian",
			field:      FixedLengthField{Name: "f", FieldType: FieldTypeFloat64, Value: "3.14"},
			endianness: EndiannessBig,
			wantLen:    8,
		},
		{
			name:       "uint64 big-endian",
			field:      FixedLengthField{Name: "f", FieldType: FieldTypeUint64, Value: "1000"},
			endianness: EndiannessBig,
			wantLen:    8,
		},
		{
			name:       "int32 invalid",
			field:      FixedLengthField{Name: "f", FieldType: FieldTypeInt32, Value: "not-a-number"},
			endianness: EndiannessBig,
			wantErr:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := &FixedLengthPayload{Fields: []FixedLengthField{tt.field}}
			got, err := DecodeFixedLengthPayload(payload, tt.endianness)
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && len(got) != tt.wantLen {
				t.Errorf("len(got) = %d, want %d", len(got), tt.wantLen)
			}
		})
	}
}

func TestDecodeFixedLengthPayload_BytesType_InvalidHex(t *testing.T) {
	payload := &FixedLengthPayload{
		Fields: []FixedLengthField{
			{Name: "f", FieldType: FieldTypeBytes, Length: 4, Value: "ZZZZ"},
		},
	}
	_, err := DecodeFixedLengthPayload(payload, EndiannessBig)
	if err == nil {
		t.Error("expected error for invalid hex, got nil")
	}
}

func TestDecodeFixedLengthPayload_BytesType_ExceedsLength(t *testing.T) {
	payload := &FixedLengthPayload{
		Fields: []FixedLengthField{
			{Name: "f", FieldType: FieldTypeBytes, Length: 2, Value: "aabbccdd"},
		},
	}
	_, err := DecodeFixedLengthPayload(payload, EndiannessBig)
	if err == nil {
		t.Error("expected error for bytes exceeding length, got nil")
	}
}

func TestDecodeFixedLengthPayload_UnknownFieldType(t *testing.T) {
	payload := &FixedLengthPayload{
		Fields: []FixedLengthField{
			{Name: "f", FieldType: "unknown_type", Value: "123"},
		},
	}
	_, err := DecodeFixedLengthPayload(payload, EndiannessBig)
	if err == nil {
		t.Error("expected error for unknown field type, got nil")
	}
}

func TestDecodePayload_Fixed_SpaceSeparated(t *testing.T) {
	// スペース区切りの hex 文字列が正しく処理されることを確認する。
	got, err := DecodePayload("AA BB CC", EncodingFixed, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []byte{0xAA, 0xBB, 0xCC}
	if !bytesEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// TestEncodeFixedField_ErrorMessages はエラー文言が UI にそのまま出るため、
// 型名と書式が崩れていないことを確認する。
func TestEncodeFixedField_ErrorMessages(t *testing.T) {
	tests := []struct {
		name  string
		want  string
		field FixedLengthField
	}{
		{
			name:  "numeric parse failure names the field type",
			field: FixedLengthField{Name: "f", FieldType: FieldTypeUint16, Value: "abc"},
			want:  `field 'f': invalid uint16: strconv.ParseUint: parsing "abc": invalid syntax`,
		},
		{
			name:  "numeric overflow names the field type",
			field: FixedLengthField{Name: "count", FieldType: FieldTypeUint8, Value: "256"},
			want:  `field 'count': invalid uint8: strconv.ParseUint: parsing "256": value out of range`,
		},
		{
			name:  "float parse failure names the field type",
			field: FixedLengthField{Name: "ratio", FieldType: FieldTypeFloat32, Value: "x"},
			want:  `field 'ratio': invalid float32: strconv.ParseFloat: parsing "x": invalid syntax`,
		},
		{
			name:  "string length must be positive",
			field: FixedLengthField{Name: "hdr", FieldType: FieldTypeString, Length: 0, Value: "a"},
			want:  `field 'hdr': length must be > 0`,
		},
		{
			name:  "string rejects non-ASCII",
			field: FixedLengthField{Name: "hdr", FieldType: FieldTypeString, Length: 4, Value: "あ"},
			want:  `field 'hdr': character 'あ' is not single-byte (ASCII only)`,
		},
		{
			name:  "string exceeding length reports both sizes",
			field: FixedLengthField{Name: "hdr", FieldType: FieldTypeString, Length: 2, Value: "hello"},
			want:  `field 'hdr': data (5 bytes) exceeds length 2`,
		},
		{
			name:  "bytes rejects invalid hex",
			field: FixedLengthField{Name: "raw", FieldType: FieldTypeBytes, Length: 4, Value: "ZZ"},
			want:  `field 'raw': invalid hex: encoding/hex: invalid byte: U+005A 'Z'`,
		},
		{
			name:  "bytes exceeding length reports both sizes",
			field: FixedLengthField{Name: "raw", FieldType: FieldTypeBytes, Length: 2, Value: "aabbcc"},
			want:  `field 'raw': data (3 bytes) exceeds length 2`,
		},
		{
			name:  "unknown field type is echoed back",
			field: FixedLengthField{Name: "f", FieldType: "nope", Value: "1"},
			want:  `field 'f': unknown field type: nope`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := &FixedLengthPayload{Fields: []FixedLengthField{tt.field}}
			_, err := DecodeFixedLengthPayload(payload, EndiannessBig)
			if err == nil {
				t.Fatal("expected an error, got nil")
			}
			var vErr *cmn.ValidationError
			if !errors.As(err, &vErr) {
				t.Fatalf("expected *cmn.ValidationError, got %T", err)
			}
			if vErr.Field != "fixedLengthPayload" {
				t.Errorf("Field = %q, want %q", vErr.Field, "fixedLengthPayload")
			}
			if vErr.Message != tt.want {
				t.Errorf("Message = %q, want %q", vErr.Message, tt.want)
			}
		})
	}
}

// TestEncodeFixedField_LengthMatchesFieldTypeByteSize は、エンコード結果の長さと
// FieldTypeByteSize が単一のテーブルから導かれていることを全数値型で確認する。
func TestEncodeFixedField_LengthMatchesFieldTypeByteSize(t *testing.T) {
	numericTypes := []FieldType{
		FieldTypeUint8, FieldTypeUint16, FieldTypeUint32, FieldTypeUint64,
		FieldTypeInt8, FieldTypeInt16, FieldTypeInt32, FieldTypeInt64,
		FieldTypeFloat32, FieldTypeFloat64,
	}
	for _, ft := range numericTypes {
		t.Run(string(ft), func(t *testing.T) {
			size := FieldTypeByteSize(ft)
			if size <= 0 {
				t.Fatalf("FieldTypeByteSize(%q) = %d, want a positive size", ft, size)
			}
			// Length はわざと型サイズと矛盾させ、数値型では無視されることも確かめる。
			payload := &FixedLengthPayload{
				Fields: []FixedLengthField{{Name: "f", FieldType: ft, Value: "1", Length: 99}},
			}
			got, err := DecodeFixedLengthPayload(payload, EndiannessBig)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != size {
				t.Errorf("encoded length = %d, want %d (FieldTypeByteSize)", len(got), size)
			}
		})
	}
}

func TestPadTo(t *testing.T) {
	tests := []struct {
		name   string
		data   []byte
		want   []byte
		length int
	}{
		{name: "pads with zeros", data: []byte{0xAA}, length: 3, want: []byte{0xAA, 0x00, 0x00}},
		{name: "exact length is unchanged", data: []byte{0xAA, 0xBB}, length: 2, want: []byte{0xAA, 0xBB}},
		{name: "longer than length is returned as-is", data: []byte{1, 2, 3}, length: 2, want: []byte{1, 2, 3}},
		{name: "empty input", data: []byte{}, length: 2, want: []byte{0x00, 0x00}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := padTo(tt.data, tt.length); !bytesEqual(got, tt.want) {
				t.Errorf("padTo() = %v, want %v", got, tt.want)
			}
		})
	}
}

// bytesEqual は2つのバイト列が等しいことを確認する
func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestEncodePayload(t *testing.T) {
	tests := []struct {
		name     string
		encoding PayloadEncoding
		want     string
		data     []byte
	}{
		{
			name:     "text",
			data:     []byte("hello"),
			encoding: EncodingText,
			want:     "hello",
		},
		{
			name:     "json valid",
			data:     []byte(`{"key":"val"}`),
			encoding: EncodingJSON,
			want:     "{\n  \"key\": \"val\"\n}",
		},
		{
			name:     "json invalid bytes fall back to string",
			data:     []byte("not json"),
			encoding: EncodingJSON,
			want:     "not json",
		},
		{
			name:     "fixed treated as hex",
			data:     []byte{0xAB, 0xCD},
			encoding: EncodingFixed,
			want:     "abcd",
		},
		{
			name:     "unknown encoding falls back to text",
			data:     []byte("data"),
			encoding: "unknown",
			want:     "data",
		},
		{
			name:     "empty bytes",
			data:     []byte{},
			encoding: EncodingText,
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EncodePayload(tt.data, tt.encoding)
			if got != tt.want {
				t.Errorf("EncodePayload() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDecodePayload(t *testing.T) {
	tests := []struct {
		name          string
		payload       string
		encoding      PayloadEncoding
		want          []byte
		messageLength int
		wantErr       bool
	}{
		{
			name:     "text",
			payload:  "hello",
			encoding: EncodingText,
			want:     []byte("hello"),
		},
		{
			name:     "text empty",
			payload:  "",
			encoding: EncodingText,
			want:     []byte(""),
		},
		{
			name:     "json valid object",
			payload:  `{"a":1}`,
			encoding: EncodingJSON,
			want:     []byte(`{"a":1}`),
		},
		{
			name:     "json valid array",
			payload:  `[1,2,3]`,
			encoding: EncodingJSON,
			want:     []byte(`[1,2,3]`),
		},
		{
			name:     "json invalid",
			payload:  "not json",
			encoding: EncodingJSON,
			wantErr:  true,
		},
		{
			name:          "fixed valid exact length",
			payload:       "aabb",
			encoding:      EncodingFixed,
			messageLength: 2,
			want:          []byte{0xAA, 0xBB},
		},
		{
			name:          "fixed with padding",
			payload:       "aa",
			encoding:      EncodingFixed,
			messageLength: 4,
			want:          []byte{0xAA, 0x00, 0x00, 0x00},
		},
		{
			name:          "fixed payload exceeds messageLength",
			payload:       "aabbccdd",
			encoding:      EncodingFixed,
			messageLength: 2,
			wantErr:       true,
		},
		{
			name:          "fixed messageLength zero",
			payload:       "aa",
			encoding:      EncodingFixed,
			messageLength: 0,
			wantErr:       true,
		},
		{
			name:          "fixed messageLength negative",
			payload:       "aa",
			encoding:      EncodingFixed,
			messageLength: -1,
			wantErr:       true,
		},
		{
			name:     "unknown encoding",
			payload:  "data",
			encoding: "unknown",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DecodePayload(tt.payload, tt.encoding, tt.messageLength)
			if (err != nil) != tt.wantErr {
				t.Fatalf("DecodePayload() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && !bytesEqual(got, tt.want) {
				t.Errorf("DecodePayload() = %v, want %v", got, tt.want)
			}
		})
	}
}
