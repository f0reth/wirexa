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
