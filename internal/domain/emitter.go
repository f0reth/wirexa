package domain

// Emitter はフロントエンドへのイベント通知の抽象。
type Emitter interface {
	Emit(event string, data any)
}
