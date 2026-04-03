package domain

// Logger はアプリケーション全体で使用するログインターフェース。
type Logger interface {
	Info(msg string, args ...any)
	Error(msg string, args ...any)
	Debug(msg string, args ...any)
}

// NoopLogger は何もしない Logger 実装（テスト・デフォルト用）。
type NoopLogger struct{}

// Info は何もしない。
func (NoopLogger) Info(_ string, _ ...any) {}

// Error は何もしない。
func (NoopLogger) Error(_ string, _ ...any) {}

// Debug は何もしない。
func (NoopLogger) Debug(_ string, _ ...any) {}
