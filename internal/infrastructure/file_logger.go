package infrastructure

import (
	"log/slog"
	"os"
	"path/filepath"

	"gopkg.in/natefinch/lumberjack.v2"

	domain "github.com/f0reth/Wirexa/internal/domain"
)

type fileLogger struct {
	slog *slog.Logger
}

// NewFileLogger はファイルローテーション付きのロガーを生成する。
// logDir にログディレクトリを指定する（存在しない場合は自動作成）。
func NewFileLogger(logDir string) (domain.Logger, error) {
	if err := os.MkdirAll(logDir, 0o750); err != nil {
		return nil, err
	}
	w := &lumberjack.Logger{
		Filename:   filepath.Join(logDir, "wirexa.log"),
		MaxSize:    10, // MB
		MaxBackups: 5,
		Compress:   true,
	}
	handler := slog.NewTextHandler(w, &slog.HandlerOptions{Level: slog.LevelInfo})
	return &fileLogger{slog: slog.New(handler)}, nil
}

func (l *fileLogger) Info(msg string, args ...any) {
	l.slog.Info(msg, args...)
}

func (l *fileLogger) Error(msg string, args ...any) {
	l.slog.Error(msg, args...)
}

func (l *fileLogger) Debug(msg string, args ...any) {
	l.slog.Debug(msg, args...)
}
