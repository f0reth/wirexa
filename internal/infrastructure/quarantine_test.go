package infrastructure

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestQuarantineFile_RenamesToCorrupt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.json")
	if err := os.WriteFile(path, []byte("{broken"), 0o600); err != nil {
		t.Fatal(err)
	}

	dest, err := QuarantineFile(path)
	if err != nil {
		t.Fatalf("QuarantineFile: %v", err)
	}
	if dest != path+".corrupt" {
		t.Errorf("dest = %q, want %q", dest, path+".corrupt")
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Error("original file should have been moved away")
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("ReadFile(dest): %v", err)
	}
	if string(got) != "{broken" {
		t.Errorf("quarantined content = %q", got)
	}
}

// 退避先が既にある場合は別名へ退避し、既存の退避ファイルを上書きしない。
func TestQuarantineFile_FallsBackWhenCorruptExists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.json")
	if err := os.WriteFile(path+".corrupt", []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}

	dest, err := QuarantineFile(path)
	if err != nil {
		t.Fatalf("QuarantineFile: %v", err)
	}
	if !strings.HasPrefix(dest, path+".corrupt.") {
		t.Errorf("dest = %q, want prefix %q", dest, path+".corrupt.")
	}
	if got, _ := os.ReadFile(dest); string(got) != "new" {
		t.Errorf("fallback content = %q, want new", got)
	}
	if got, _ := os.ReadFile(path + ".corrupt"); string(got) != "old" {
		t.Errorf("existing quarantine file changed: %q", got)
	}
}
