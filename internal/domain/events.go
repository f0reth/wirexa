package domain

// Wails イベント名の定数。イベント名はここだけで定義する。
// go generate が frontend/src/shared/wails-events.ts へ書き出す（tools/gen-events を参照）。
//
//go:generate go run github.com/f0reth/Wirexa/tools/gen-events
const (
	EventMQTTConnected           = "mqtt:connected"
	EventMQTTDisconnected        = "mqtt:disconnected"
	EventMQTTConnectionLost      = "mqtt:connection-lost"
	EventMQTTConnectionFailed    = "mqtt:connection-failed"
	EventMQTTMessage             = "mqtt:message"
	EventMQTTScanTopic           = "mqtt:scan-topic"
	EventMQTTScanStopped         = "mqtt:scan-stopped"
	EventMQTTSubscriptionDropped = "mqtt:subscription-dropped"
	EventUDPMessage              = "udp:message"
	EventAppBeforeClose          = "app:before-close"
)
