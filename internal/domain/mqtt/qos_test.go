package mqttdomain

import (
	"errors"
	"testing"

	cmn "github.com/f0reth/Wirexa/internal/domain"
)

func TestValidateQoS(t *testing.T) {
	for _, qos := range []byte{0, 1, 2} {
		if err := ValidateQoS(qos); err != nil {
			t.Errorf("ValidateQoS(%d) = %v, want nil", qos, err)
		}
	}
	for _, qos := range []byte{3, 255} {
		ve, ok := errors.AsType[*cmn.ValidationError](ValidateQoS(qos))
		if !ok {
			t.Fatalf("ValidateQoS(%d): want ValidationError", qos)
		}
		if ve.Field != "qos" || ve.Message != "must be 0, 1, or 2" {
			t.Errorf("ValidateQoS(%d) = {%q, %q}, want {qos, must be 0, 1, or 2}", qos, ve.Field, ve.Message)
		}
	}
}
