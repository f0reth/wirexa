package domain

// Logger はアプリケーション全体で使用するログインターフェース。
type Logger interface {
	Info(msg string, args ...any)
	Error(msg string, args ...any)
	Debug(msg string, args ...any)
}
