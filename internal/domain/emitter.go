package domain

// Emitter はフロントエンドへのイベント通知の抽象。
// Wails に依存しないため、テスト時はモックに差し替え可能。
type Emitter interface {
	Emit(event string, data any)
}
