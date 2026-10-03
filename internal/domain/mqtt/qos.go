package mqttdomain

import cmn "github.com/f0reth/Wirexa/internal/domain"

// maxQoS は MQTT の QoS の最大値。
const maxQoS = 2

// ValidateQoS は publish・購読の QoS が 0・1・2 のいずれかかを検証する。
func ValidateQoS(qos byte) error {
	if qos > maxQoS {
		return &cmn.ValidationError{Field: "qos", Message: "must be 0, 1, or 2"}
	}
	return nil
}
