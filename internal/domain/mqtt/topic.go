package mqttdomain

import (
	"errors"
	"strings"

	cmn "github.com/f0reth/Wirexa/internal/domain"
)

// maxTopicBytes は MQTT のトピック (UTF-8 エンコード文字列) の最大バイト数。
const maxTopicBytes = 65535

// ErrSubscriptionRejected はブローカーが購読を拒否した (SUBACK が失敗コードを返した) ことを表す。
var ErrSubscriptionRejected = errors.New("subscription rejected by broker")

// ValidateTopicName は publish 先のトピック名を検証する (MQTT 3.1.1 4.7)。
// トピック名にはワイルドカード (+ と #) を含められない。ブローカーは違反をプロトコル違反として
// 接続ごと切断し、QoS 1/2 ではクライアントが再接続後に同じパケットを再送して切断を繰り返すため、
// 送信前に拒否する。
func ValidateTopicName(field, topic string) error {
	if err := validateTopicString(field, topic); err != nil {
		return err
	}
	if strings.ContainsAny(topic, "+#") {
		return &cmn.ValidationError{Field: field, Message: "must not contain wildcards (+ or #)"}
	}
	return nil
}

// ValidateTopicFilter は購読・購読解除のトピックフィルターを検証する (MQTT 3.1.1 4.7.1)。
// + は階層全体を占め、# は階層全体を占めて最後に置く。ブローカーによっては位置違反の
// フィルターを受け付けて何にも一致しない購読を作るため、ブローカーに依らず送信前に拒否する。
func ValidateTopicFilter(field, filter string) error {
	if err := validateTopicString(field, filter); err != nil {
		return err
	}
	levels := strings.Split(filter, "/")
	for i, level := range levels {
		if strings.Contains(level, "#") && (level != "#" || i != len(levels)-1) {
			return &cmn.ValidationError{Field: field, Message: "# must occupy the last level entirely"}
		}
		if strings.Contains(level, "+") && level != "+" {
			return &cmn.ValidationError{Field: field, Message: "+ must occupy an entire level"}
		}
	}
	return nil
}

// validateTopicString はトピック名とフィルターに共通の制約 (空でない・NUL を含まない・長さ) を検証する。
func validateTopicString(field, topic string) error {
	if topic == "" {
		return &cmn.ValidationError{Field: field, Message: cmn.MsgRequired}
	}
	if len(topic) > maxTopicBytes {
		return &cmn.ValidationError{Field: field, Message: "must be at most 65535 bytes"}
	}
	if strings.ContainsRune(topic, 0) {
		return &cmn.ValidationError{Field: field, Message: "must not contain the null character"}
	}
	return nil
}
