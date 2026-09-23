//go:build integration

package integration

import (
	"bytes"
	"encoding/json/v2"
	"os"
	"path/filepath"
	"testing"

	openapiapp "github.com/f0reth/Wirexa/internal/application/openapi"
	openapidomain "github.com/f0reth/Wirexa/internal/domain/openapi"
	openapiinfra "github.com/f0reth/Wirexa/internal/infrastructure/openapi"
	"github.com/f0reth/Wirexa/internal/testutil"
)

// newOpenAPIFileService は recentsPath の実リポジトリと実ファイル I/O で FileService を組み立てる。
func newOpenAPIFileService(recentsPath string) *openapiapp.FileService {
	return openapiapp.NewFileService(
		openapiinfra.NewRecentRepository(recentsPath),
		openapiinfra.NewNativeFileAccess(),
		testutil.NoopLogger{},
	)
}

// TestOpenAPI_CorruptRecents は recents ファイルが壊れていても起動を継続し、
// 元の内容を退避したまま新しい一覧が保存されることを確認する。
func TestOpenAPI_CorruptRecents(t *testing.T) {
	dir := t.TempDir()
	recentsPath := filepath.Join(dir, "openapi-recents.json")
	original := []byte("{invalid json}")
	if err := os.WriteFile(recentsPath, original, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	svc := newOpenAPIFileService(recentsPath)

	quarantined := recentsPath + ".corrupt"
	if got, err := os.ReadFile(quarantined); err != nil || !bytes.Equal(got, original) {
		t.Fatalf("openapi-recents.json.corrupt should keep the original bytes: %q, %v", got, err)
	}
	if got := svc.GetRecents(); len(got) != 0 {
		t.Fatalf("GetRecents = %+v, want empty", got)
	}

	target := svc.OpenSelected(filepath.Join(dir, "spec.yaml"))

	data, err := os.ReadFile(recentsPath)
	if err != nil {
		t.Fatalf("openapi-recents.json should be recreated: %v", err)
	}
	var saved []openapidomain.OpenAPIRecent
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(saved) != 1 || saved[0].Path != target {
		t.Fatalf("saved recents = %+v, want only %s", saved, target)
	}
	if got, _ := os.ReadFile(quarantined); !bytes.Equal(got, original) {
		t.Fatalf("quarantined file changed: %q", got)
	}
}

// TestOpenAPI_RecentsSeedAcrossRestart はダイアログで開いたパスが
// 再起動後も recents 経由で読めることを確認する。
func TestOpenAPI_RecentsSeedAcrossRestart(t *testing.T) {
	dir := t.TempDir()
	recentsPath := filepath.Join(dir, "openapi-recents.json")
	target := filepath.Join(dir, "spec.yaml")
	if err := os.WriteFile(target, []byte("openapi: 3.1.0"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	first := newOpenAPIFileService(recentsPath)
	opened := first.OpenSelected(target)

	restarted := newOpenAPIFileService(recentsPath)
	got, err := restarted.ReadFile(opened)
	if err != nil {
		t.Fatalf("ReadFile after restart: %v", err)
	}
	if got != "openapi: 3.1.0" {
		t.Fatalf("ReadFile content = %q", got)
	}
}
