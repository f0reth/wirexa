package mqttdomain

import (
	"strings"

	cmn "github.com/f0reth/Wirexa/internal/domain"
)

// ValidateBrokerScheme は、useTLS のときに Broker URL のスキームが TLS で接続できるものかを検証する。
// TLS のスキーム (ssl・tls・mqtts・wss) と、TLS のスキームに変えられるもの (tcp・mqtt・ws・スキーム無し) を通す。
// スキームの大文字小文字は区別しない。それ以外のスキームは平文で繋がるか接続に失敗するだけなので拒否する。
// useTLS でなければ何も検証しない。
func ValidateBrokerScheme(broker string, useTLS bool) error {
	if !useTLS {
		return nil
	}
	scheme, _, ok := strings.Cut(broker, "://")
	if !ok {
		return nil
	}
	switch strings.ToLower(scheme) {
	case "tcp", "mqtt", "ws", "ssl", "tls", "mqtts", "wss":
		return nil
	}
	return &cmn.ValidationError{Field: "broker URL", Message: "scheme cannot be used with TLS"}
}
