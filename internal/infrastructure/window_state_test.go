package infrastructure

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// countLogger は Error の呼び出し回数だけを数えるテスト用ロガー。
type countLogger struct{ errors int }

func (l *countLogger) Info(_ string, _ ...any)  {}
func (l *countLogger) Debug(_ string, _ ...any) {}
func (l *countLogger) Error(_ string, _ ...any) { l.errors++ }
func TestWindowState_RoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "window-state.json")
	want := WindowState{Width: 1024, Height: 768, X: 100, Y: 50, Maximised: true}

	if err := SaveWindowState(path, want); err != nil {
		t.Fatalf("SaveWindowState: %v", err)
	}
	got, ok := LoadWindowState(path, nil)
	if !ok {
		t.Fatalf("LoadWindowState: ok=false, want true")
	}
	if got != want {
		t.Fatalf("round-trip mismatch: got %+v want %+v", got, want)
	}
}

func TestLoadWindowState_Missing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist.json")
	if _, ok := LoadWindowState(path, nil); ok {
		t.Fatalf("expected ok=false for missing file")
	}
}

func TestLoadWindowState_Corrupt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "window-state.json")
	if err := os.WriteFile(path, []byte("{ not json"), 0o600); err != nil {
		t.Fatalf("write corrupt file: %v", err)
	}
	if _, ok := LoadWindowState(path, nil); ok {
		t.Fatalf("expected ok=false for corrupt file")
	}
}

func TestLoadWindowState_CorruptIsQuarantined(t *testing.T) {
	path := filepath.Join(t.TempDir(), "window-state.json")
	corrupt := []byte("{ not json")
	if err := os.WriteFile(path, corrupt, 0o600); err != nil {
		t.Fatalf("write corrupt file: %v", err)
	}
	logger := &countLogger{}

	if _, ok := LoadWindowState(path, logger); ok {
		t.Fatal("expected ok=false for corrupt file")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("corrupt file should be moved away: %v", err)
	}
	got, err := os.ReadFile(path + ".corrupt")
	if err != nil {
		t.Fatalf("read quarantined file: %v", err)
	}
	if !bytes.Equal(got, corrupt) {
		t.Errorf("quarantined content = %q, want %q", got, corrupt)
	}
	if logger.errors == 0 {
		t.Error("quarantine should be logged")
	}
}

// 読み込み自体の失敗 (ここではパスがディレクトリ) では退避しない。
func TestLoadWindowState_ReadErrorIsNotQuarantined(t *testing.T) {
	path := filepath.Join(t.TempDir(), "window-state.json")
	if err := os.Mkdir(path, 0o750); err != nil {
		t.Fatal(err)
	}
	logger := &countLogger{}

	if _, ok := LoadWindowState(path, logger); ok {
		t.Fatal("expected ok=false for unreadable file")
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("unreadable path should be left in place: %v", err)
	}
	if _, err := os.Stat(path + ".corrupt"); !os.IsNotExist(err) {
		t.Errorf("unreadable file must not be quarantined: %v", err)
	}
	if logger.errors == 0 {
		t.Error("read failure should be logged")
	}
}
