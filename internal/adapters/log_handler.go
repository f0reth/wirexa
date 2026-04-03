package adapters

import (
	domain "github.com/f0reth/Wirexa/internal/domain"
)

// LogEntry はフロントエンドから送信されるログエントリを表す。
type LogEntry struct {
	Level   string         `json:"level"`
	Source  string         `json:"source"`
	Message string         `json:"message"`
	Attrs   map[string]any `json:"attrs,omitempty"`
}

// LogHandler はフロントエンドからのログを受け取る Wails RPC アダプター。
type LogHandler struct {
	logger domain.Logger
}

// SetupLogHandler は既存の LogHandler インスタンスにロガーを注入する。
func SetupLogHandler(h *LogHandler, logger domain.Logger) {
	h.logger = logger
}

// Log はフロントエンドから送られるログエントリを受け取りファイルに書き込む。
func (h *LogHandler) Log(entry LogEntry) {
	args := make([]any, 0, len(entry.Attrs)*2+2)
	args = append(args, "source", entry.Source)
	for k, v := range entry.Attrs {
		args = append(args, k, v)
	}
	switch entry.Level {
	case "ERROR":
		h.logger.Error(entry.Message, args...)
	case "DEBUG":
		h.logger.Debug(entry.Message, args...)
	default:
		h.logger.Info(entry.Message, args...)
	}
}
