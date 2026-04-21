package udpdomain

import (
	"errors"
	"testing"

	cmn "github.com/f0reth/Wirexa/internal/domain"
)

func TestFieldTypeByteSize_AllTypes(t *testing.T) {
	tests := []struct {
		fieldType FieldType
		want      int
	}{
		{FieldTypeUint8, 1},
		{FieldTypeInt8, 1},
		{FieldTypeUint16, 2},
		{FieldTypeInt16, 2},
		{FieldTypeUint32, 4},
		{FieldTypeInt32, 4},
		{FieldTypeFloat32, 4},
		{FieldTypeUint64, 8},
		{FieldTypeInt64, 8},
		{FieldTypeFloat64, 8},
		{FieldTypeString, -1},
		{FieldTypeBytes, -1},
		{"unknown", -1},
	}
	for _, tt := range tests {
		t.Run(string(tt.fieldType), func(t *testing.T) {
			if got := FieldTypeByteSize(tt.fieldType); got != tt.want {
				t.Errorf("FieldTypeByteSize(%q) = %d, want %d", tt.fieldType, got, tt.want)
			}
		})
	}
}

func TestUdpSendRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		req     UdpSendRequest
		wantErr bool
		field   string
	}{
		{
			name:    "valid request",
			req:     UdpSendRequest{Host: "localhost", Port: 9000},
			wantErr: false,
		},
		{
			name:    "port 1 (min valid)",
			req:     UdpSendRequest{Host: "127.0.0.1", Port: 1},
			wantErr: false,
		},
		{
			name:    "port 65535 (max valid)",
			req:     UdpSendRequest{Host: "127.0.0.1", Port: 65535},
			wantErr: false,
		},
		{
			name:    "empty host",
			req:     UdpSendRequest{Host: "", Port: 9000},
			wantErr: true,
			field:   "host",
		},
		{
			name:    "port 0",
			req:     UdpSendRequest{Host: "localhost", Port: 0},
			wantErr: true,
			field:   "port",
		},
		{
			name:    "port negative",
			req:     UdpSendRequest{Host: "localhost", Port: -1},
			wantErr: true,
			field:   "port",
		},
		{
			name:    "port 65536",
			req:     UdpSendRequest{Host: "localhost", Port: 65536},
			wantErr: true,
			field:   "port",
		},
		{
			name:    "port very large",
			req:     UdpSendRequest{Host: "localhost", Port: 99999},
			wantErr: true,
			field:   "port",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.field != "" {
				var ve *cmn.ValidationError
				if !errors.As(err, &ve) {
					t.Fatalf("expected *ValidationError, got %T", err)
				}
				if ve.Field != tt.field {
					t.Errorf("ValidationError.Field = %q, want %q", ve.Field, tt.field)
				}
			}
		})
	}
}
