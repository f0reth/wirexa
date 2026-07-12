package infrastructure

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWindowState_RoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "window-state.json")
	want := WindowState{Width: 1024, Height: 768, X: 100, Y: 50, Maximised: true}

	if err := SaveWindowState(path, want); err != nil {
		t.Fatalf("SaveWindowState: %v", err)
	}
	got, ok := LoadWindowState(path)
	if !ok {
		t.Fatalf("LoadWindowState: ok=false, want true")
	}
	if got != want {
		t.Fatalf("round-trip mismatch: got %+v want %+v", got, want)
	}
}

func TestLoadWindowState_Missing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist.json")
	if _, ok := LoadWindowState(path); ok {
		t.Fatalf("expected ok=false for missing file")
	}
}

func TestLoadWindowState_Corrupt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "window-state.json")
	if err := os.WriteFile(path, []byte("{ not json"), 0o600); err != nil {
		t.Fatalf("write corrupt file: %v", err)
	}
	if _, ok := LoadWindowState(path); ok {
		t.Fatalf("expected ok=false for corrupt file")
	}
}
