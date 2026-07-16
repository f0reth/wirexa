package infrastructure

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// assertNoTempLeft は書き込み先ディレクトリに一時ファイルが残っていないことを検証する。
func assertNoTempLeft(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".tmp-") {
			t.Fatalf("leftover temp file: %s", e.Name())
		}
	}
}

func TestAtomicWriteFile_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")

	if err := AtomicWriteFile(path, []byte("hello"), 0o600); err != nil {
		t.Fatalf("AtomicWriteFile: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "hello" {
		t.Fatalf("content = %q, want %q", got, "hello")
	}
	assertNoTempLeft(t, dir)
}

func TestAtomicWriteFile_ReplacesExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")
	if err := os.WriteFile(path, []byte("old-and-longer"), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if err := AtomicWriteFile(path, []byte("new"), 0o600); err != nil {
		t.Fatalf("AtomicWriteFile: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	// 旧内容の残骸が混ざらず、完全に置き換わること。
	if string(got) != "new" {
		t.Fatalf("content = %q, want %q", got, "new")
	}
	assertNoTempLeft(t, dir)
}

func TestAtomicWriteFile_MissingDirReturnsError(t *testing.T) {
	dir := t.TempDir()
	// AtomicWriteFile はディレクトリを作らない。存在しない親への書き込みは失敗する。
	path := filepath.Join(dir, "nope", "out.txt")

	if err := AtomicWriteFile(path, []byte("data"), 0o600); err == nil {
		t.Fatalf("expected error for missing parent directory")
	}
	assertNoTempLeft(t, dir)
}
