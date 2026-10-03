package mqttdomain

import (
	"strings"

	cmn "github.com/f0reth/Wirexa/internal/domain"
)

// fieldBroker はブローカー URL の検証エラーのフィールド名。
const fieldBroker = "broker URL"

// ValidateBroker は接続先のブローカー URL を検証する。空を拒否し、useTLS のときは
// validateBrokerScheme でスキームも検証する。
func ValidateBroker(broker string, useTLS bool) error {
	if broker == "" {
		return &cmn.ValidationError{Field: fieldBroker, Message: cmn.MsgRequired}
	}
	return validateBrokerScheme(broker, useTLS)
}

// validateBrokerScheme は、useTLS のときに Broker URL のスキームが TLS で接続できるものかを検証する。
// TLS のスキーム (ssl・tls・mqtts・wss) と、TLS のスキームに変えられるもの (tcp・mqtt・ws・スキーム無し) を通す。
// スキームの大文字小文字は区別しない。それ以外のスキームは平文で繋がるか接続に失敗するだけなので拒否する。
// useTLS でなければ何も検証しない。
func validateBrokerScheme(broker string, useTLS bool) error {
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
	return &cmn.ValidationError{Field: fieldBroker, Message: "scheme cannot be used with TLS"}
}
