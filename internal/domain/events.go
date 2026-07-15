package domain

// Wails イベント名の中央レジストリ。フロントエンドと共有する唯一の真実の源。
// バックエンドが Emitter / runtime.EventsEmit で発火し、フロントが購読するイベント名を
// ここに集約する。これらの値は go generate 経由で frontend/src/shared/wails-events.ts
// へ書き出され、フロント側の二重宣言を排除する（tools/gen-events を参照）。
//
//go:generate go run github.com/f0reth/Wirexa/tools/gen-events
const (
	EventMQTTConnected        = "mqtt:connected"
	EventMQTTDisconnected     = "mqtt:disconnected"
	EventMQTTConnectionLost   = "mqtt:connection-lost"
	EventMQTTConnectionFailed = "mqtt:connection-failed"
	EventMQTTMessage          = "mqtt:message"
	EventUDPMessage           = "udp:message"
	EventAppBeforeClose       = "app:before-close"
)
