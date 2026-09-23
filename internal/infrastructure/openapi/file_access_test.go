package openapiinfra

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNativeFileAccess_WriteThenRead(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "spec.yaml")
	fa := NewNativeFileAccess()

	if err := fa.WriteFile(target, []byte("openapi: 3.1.0")); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	got, err := fa.ReadFile(target)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "openapi: 3.1.0" {
		t.Fatalf("ReadFile content = %q", got)
	}

	// アトミック書き込み: .tmp-* が残っていないこと。
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
