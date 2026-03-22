package mqttdomain

import "fmt"

// NotFoundError はリソースが見つからない場合のエラー。
type NotFoundError struct {
	Resource string // "connection" | "profile"
	ID       string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("%s not found: %s", e.Resource, e.ID)
}

// ValidationError は入力値検証エラー。
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("invalid %s: %s", e.Field, e.Message)
	}
	return e.Message
}
