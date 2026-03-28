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
					{Name: "field1", Length: 4, Value: "DEADBEEF"},
				},
			},
			want:    []byte{0xDE, 0xAD, 0xBE, 0xEF},
			wantErr: false,
		},
		{
			name: "single field with padding",
			payload: &FixedLengthPayload{
				Fields: []FixedLengthField{
					{Name: "field1", Length: 4, Value: "DEAD"},
				},
			},
			want:    []byte{0xDE, 0xAD, 0x00, 0x00},
			wantErr: false,
		},
		{
			name: "single field exceeds length",
			payload: &FixedLengthPayload{
				Fields: []FixedLengthField{
					{Name: "field1", Length: 2, Value: "DEADBEEF"},
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
					{Name: "header", Length: 2, Value: "DEAD"},
					{Name: "payload", Length: 2, Value: "BEEF"},
				},
			},
			want:    []byte{0xDE, 0xAD, 0xBE, 0xEF},
			wantErr: false,
		},
		{
			name: "multiple fields with padding",
			payload: &FixedLengthPayload{
				Fields: []FixedLengthField{
					{Name: "field1", Length: 4, Value: "AD"},
					{Name: "field2", Length: 2, Value: "BE"},
				},
			},
			want:    []byte{0xAD, 0x00, 0x00, 0x00, 0xBE, 0x00},
			wantErr: false,
		},
		{
			name: "three fields",
			payload: &FixedLengthPayload{
				Fields: []FixedLengthField{
					{Name: "f1", Length: 1, Value: "12"},
					{Name: "f2", Length: 1, Value: "34"},
					{Name: "f3", Length: 1, Value: "56"},
				},
			},
			want:    []byte{0x12, 0x34, 0x56},
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
					{Name: "field1", Length: 0, Value: "DEAD"},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid hex value",
			payload: &FixedLengthPayload{
				Fields: []FixedLengthField{
					{Name: "field1", Length: 2, Value: "ZZZZ"},
				},
			},
			wantErr: true,
		},
		{
			name: "odd-length hex without padding",
			payload: &FixedLengthPayload{
				Fields: []FixedLengthField{
					{Name: "field1", Length: 2, Value: "ABC"},
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

func TestDecodeFixedLengthPayload_Whitespace(t *testing.T) {
	tests := []struct {
		name    string
		payload *FixedLengthPayload
		want    []byte
		wantErr bool
	}{
		{
			name: "hex with spaces",
			payload: &FixedLengthPayload{
				Fields: []FixedLengthField{
					{Name: "field1", Length: 4, Value: "DE AD BE EF"},
				},
			},
			want:    []byte{0xDE, 0xAD, 0xBE, 0xEF},
			wantErr: false,
		},
		{
			name: "hex with multiple spaces",
			payload: &FixedLengthPayload{
				Fields: []FixedLengthField{
					{Name: "field1", Length: 2, Value: "D E A D"},
				},
			},
			want:    []byte{0xDE, 0xAD},
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
