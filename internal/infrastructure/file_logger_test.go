package infrastructure

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ログディレクトリを作り、Info / Error は wirexa.log に書き、Debug は書かない (レベルは Info)。
func TestFileLogger_WritesInfoAndErrorButNotDebug(t *testing.T) {
	logDir := filepath.Join(t.TempDir(), "logs", "nested")

	logger, err := NewFileLogger(logDir)
	if err != nil {
		t.Fatalf("NewFileLogger: %v", err)
	}
	// Windows では開いたままのファイルを TempDir の後始末で消せないので閉じる。
	t.Cleanup(func() { _ = logger.(*fileLogger).out.Close() })

	logger.Info("info-message", "key", "info-value")
	logger.Error("error-message", "key", "error-value")
	logger.Debug("debug-message", "key", "debug-value")

	data, err := os.ReadFile(filepath.Join(logDir, "wirexa.log"))
	if err != nil {
		t.Fatalf("read wirexa.log: %v", err)
	}
	got := string(data)
	for _, want := range []string{
		"level=INFO", "msg=info-message", "key=info-value",
		"level=ERROR", "msg=error-message", "key=error-value",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("log should contain %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "debug-message") {
		t.Errorf("debug log should not be written:\n%s", got)
	}
}
