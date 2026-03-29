package udpdomain

import (
	"testing"
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
			got, err := DecodeFixedLengthPayload(tt.payload)
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
			got, err := DecodeFixedLengthPayload(tt.payload)
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
		name    string
		payload *FixedLengthPayload
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
			_, err := DecodeFixedLengthPayload(tt.payload)
			if (err != nil) != tt.wantErr {
				t.Errorf("DecodeFixedLengthPayload() error = %v, wantErr %v", err, tt.wantErr)
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
		data     []byte
		encoding PayloadEncoding
		want     string
	}{
		{
			name:     "text",
			data:     []byte("hello"),
			encoding: EncodingText,
			want:     "hello",
		},
		{
			name:     "hex lowercase",
			data:     []byte{0xDE, 0xAD, 0xBE, 0xEF},
			encoding: EncodingHex,
			want:     "deadbeef",
		},
		{
			name:     "base64",
			data:     []byte("hello"),
			encoding: EncodingBase64,
			want:     "aGVsbG8=",
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
		messageLength int
		want          []byte
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
			name:     "hex valid",
			payload:  "deadbeef",
			encoding: EncodingHex,
			want:     []byte{0xDE, 0xAD, 0xBE, 0xEF},
		},
		{
			name:     "hex with spaces",
			payload:  "DE AD BE EF",
			encoding: EncodingHex,
			want:     []byte{0xDE, 0xAD, 0xBE, 0xEF},
		},
		{
			name:     "hex uppercase",
			payload:  "DEADBEEF",
			encoding: EncodingHex,
			want:     []byte{0xDE, 0xAD, 0xBE, 0xEF},
		},
		{
			name:     "hex invalid",
			payload:  "zzzz",
			encoding: EncodingHex,
			wantErr:  true,
		},
		{
			name:     "hex odd length",
			payload:  "abc",
			encoding: EncodingHex,
			wantErr:  true,
		},
		{
			name:     "base64 valid",
			payload:  "aGVsbG8=",
			encoding: EncodingBase64,
			want:     []byte("hello"),
		},
		{
			name:     "base64 invalid",
			payload:  "!!!",
			encoding: EncodingBase64,
			wantErr:  true,
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
