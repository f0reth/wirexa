package mqttdomain

import (
	"errors"
	"testing"

	cmn "github.com/f0reth/Wirexa/internal/domain"
)

func TestValidateBrokerScheme(t *testing.T) {
	tests := []struct {
		name    string
		broker  string
		useTLS  bool
		wantErr bool
	}{
		{name: "TLS なしは検証しない", broker: "http://broker:1883"},
		{name: "TLS なしのスキーム無し", broker: "broker:1883"},
		{name: "tcp", broker: "tcp://broker:1883", useTLS: true},
		{name: "mqtt", broker: "mqtt://broker:1883", useTLS: true},
		{name: "ws", broker: "ws://broker:8080/mqtt", useTLS: true},
		{name: "ssl", broker: "ssl://broker:8883", useTLS: true},
		{name: "tls", broker: "tls://broker:8883", useTLS: true},
		{name: "mqtts", broker: "mqtts://broker:8883", useTLS: true},
		{name: "wss", broker: "wss://broker:443/mqtt", useTLS: true},
		{name: "大文字の TCP", broker: "TCP://broker:1883", useTLS: true},
		{name: "大文字混じりの Mqtt", broker: "Mqtt://broker:1883", useTLS: true},
		{name: "大文字の WSS", broker: "WSS://broker:443", useTLS: true},
		{name: "スキーム無し", broker: "broker:8883", useTLS: true},
		{name: "http", broker: "http://broker:1883", useTLS: true, wantErr: true},
		{name: "unix", broker: "unix:///tmp/mqtt.sock", useTLS: true, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateBrokerScheme(tc.broker, tc.useTLS)
			if !tc.wantErr {
				if err != nil {
					t.Fatalf("ValidateBrokerScheme(%q, %v) = %v, want nil", tc.broker, tc.useTLS, err)
				}
				return
			}
			if _, ok := errors.AsType[*cmn.ValidationError](err); !ok {
				t.Fatalf("ValidateBrokerScheme(%q, %v) = %v, want ValidationError", tc.broker, tc.useTLS, err)
			}
		})
	}
}
