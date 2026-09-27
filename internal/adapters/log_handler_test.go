package adapters

import (
	"slices"
	"testing"
)

// logCall は recordLogger が記録した 1 回分の呼び出し。
type logCall struct {
	level string
	msg   string
	args  []any
}

// recordLogger は呼ばれたレベル・メッセージ・引数を記録する domain.Logger。
type recordLogger struct{ calls []logCall }

func (l *recordLogger) Info(msg string, args ...any) {
	l.calls = append(l.calls, logCall{"INFO", msg, args})
}

func (l *recordLogger) Error(msg string, args ...any) {
	l.calls = append(l.calls, logCall{"ERROR", msg, args})
}

func (l *recordLogger) Debug(msg string, args ...any) {
	l.calls = append(l.calls, logCall{"DEBUG", msg, args})
}

func TestLogHandler_Log_RoutesByLevelAndFlattensAttrs(t *testing.T) {
	tests := []struct {
		level string
		want  string
	}{
		{"ERROR", "ERROR"},
		{"DEBUG", "DEBUG"},
		{"INFO", "INFO"},
		{"", "INFO"},
		{"WARN", "INFO"},
	}
	for _, tc := range tests {
		t.Run(tc.level, func(t *testing.T) {
			logger := &recordLogger{}
			h := &LogHandler{}
			SetupLogHandler(h, logger)

			h.Log(LogEntry{
				Level:   tc.level,
				Source:  "collections",
				Message: "something happened",
				Attrs:   map[string]any{"id": "c1"},
			})

			if len(logger.calls) != 1 {
				t.Fatalf("logger calls = %d, want 1", len(logger.calls))
			}
			got := logger.calls[0]
			if got.level != tc.want || got.msg != "something happened" {
				t.Fatalf("call = %+v, want level %s", got, tc.want)
			}
			want := []any{"source", "collections", "id", "c1"}
			if !slices.Equal(got.args, want) {
				t.Fatalf("args = %v, want %v", got.args, want)
			}
		})
	}
}

// Attrs が複数あっても、source の後にキーと値の組で並ぶ (map の順序は問わない)。
func TestLogHandler_Log_FlattensEveryAttr(t *testing.T) {
	logger := &recordLogger{}
	h := &LogHandler{}
	SetupLogHandler(h, logger)

	h.Log(LogEntry{Level: "INFO", Source: "mqtt", Message: "m", Attrs: map[string]any{"a": 1.0, "b": "x"}})

	args := logger.calls[0].args
	if len(args) != 6 || args[0] != "source" || args[1] != "mqtt" {
		t.Fatalf("args = %v, want source first followed by 2 attrs", args)
	}
	got := map[any]any{}
	for i := 2; i < len(args); i += 2 {
		got[args[i]] = args[i+1]
	}
	if got["a"] != 1.0 || got["b"] != "x" {
		t.Fatalf("attrs = %v, want a=1 b=x", got)
	}
}
