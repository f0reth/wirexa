package adapters

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/f0reth/Wirexa/internal/testutil"
)

func newTestHandler(t *testing.T) (*OpenAPIHandler, string) {
	t.Helper()
	dir := t.TempDir()
	h := &OpenAPIHandler{}
	SetupOpenAPIHandler(context.Background(), h, filepath.Join(dir, "openapi-recents.json"), testutil.NoopLogger{})
	return h, dir
}

func TestReadFileDeniesUngrantedPath(t *testing.T) {
	h, dir := newTestHandler(t)
	target := filepath.Join(dir, "secret.yaml")
	if err := os.WriteFile(target, []byte("openapi: 3.0.0"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := h.ReadFile(target); !errors.Is(err, errAccessDenied) {
		t.Fatalf("ReadFile on ungranted path: want errAccessDenied, got %v", err)
	}
}

func TestWriteFileDeniesUngrantedPath(t *testing.T) {
	h, dir := newTestHandler(t)
	target := filepath.Join(dir, "out.yaml")

	if err := h.WriteFile(target, "data"); !errors.Is(err, errAccessDenied) {
		t.Fatalf("WriteFile on ungranted path: want errAccessDenied, got %v", err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("WriteFile must not create a file on denial")
	}
}

func TestGrantedPathReadWrite(t *testing.T) {
	h, dir := newTestHandler(t)
	target := filepath.Join(dir, "spec.yaml")

	// grant はダイアログ経由で登録されるパスを模す。
	h.grant(target)

	if err := h.WriteFile(target, "openapi: 3.1.0"); err != nil {
		t.Fatalf("WriteFile on granted path: %v", err)
	}
	got, err := h.ReadFile(target)
	if err != nil {
		t.Fatalf("ReadFile on granted path: %v", err)
	}
	if got != "openapi: 3.1.0" {
		t.Fatalf("ReadFile content = %q", got)
	}

	// アトミック書き込み: .tmp-* が残っていないこと。
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if len(e.Name()) >= 5 && e.Name()[:5] == ".tmp-" {
			t.Fatalf("leftover temp file: %s", e.Name())
		}
	}
}

func TestGrantIsPathCleaned(t *testing.T) {
	h, dir := newTestHandler(t)
	target := filepath.Join(dir, "spec.yaml")
	h.grant(target)

	// 非正規化パス (途中に ./ を含む) でも同一とみなされること。
	messy := filepath.Join(dir, ".", "spec.yaml")
	if !h.isGranted(messy) {
		t.Fatalf("isGranted should treat cleaned paths as equal")
	}
}

func TestRecentsSeedGrantsOnStartup(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "seeded.yaml")
	if err := os.WriteFile(target, []byte("openapi: 3.0.0"), 0o600); err != nil {
		t.Fatal(err)
	}

	// recents JSON を事前に書き、再起動時 seed を模す。
	recentsPath := filepath.Join(dir, "openapi-recents.json")
	seed := []OpenAPIRecent{{Path: target, Name: "seeded.yaml", Order: 0, LastOpenedAt: "2026-07-12T00:00:00Z"}}
	data, _ := json.Marshal(seed)
	if err := os.WriteFile(recentsPath, data, 0o600); err != nil {
		t.Fatal(err)
	}

	h := &OpenAPIHandler{}
	SetupOpenAPIHandler(context.Background(), h, recentsPath, testutil.NoopLogger{})

	if _, err := h.ReadFile(target); err != nil {
		t.Fatalf("seeded recents path should be granted: %v", err)
	}
	if got := h.GetRecents(); len(got) != 1 || got[0].Path != target {
		t.Fatalf("GetRecents = %+v", got)
	}
}

func TestRemoveRecentRevokesGrant(t *testing.T) {
	h, dir := newTestHandler(t)
	target := filepath.Join(dir, "spec.yaml")
	h.grant(target)
	if err := h.recents.add(filepath.Clean(target), "spec.yaml"); err != nil {
		t.Fatal(err)
	}

	if err := h.RemoveRecent(target); err != nil {
		t.Fatalf("RemoveRecent: %v", err)
	}
	if _, err := h.ReadFile(target); !errors.Is(err, errAccessDenied) {
		t.Fatalf("after RemoveRecent, ReadFile should be denied, got %v", err)
	}
	if got := h.GetRecents(); len(got) != 0 {
		t.Fatalf("GetRecents after remove = %+v", got)
	}
}

func TestMoveRecentReorders(t *testing.T) {
	h, _ := newTestHandler(t)
	for _, p := range []string{"/a.yaml", "/b.yaml", "/c.yaml"} {
		if err := h.recents.add(p, filepath.Base(p)); err != nil {
			t.Fatal(err)
		}
	}
	// c を先頭へ。
	if err := h.MoveRecent("/c.yaml", 0); err != nil {
		t.Fatalf("MoveRecent: %v", err)
	}
	got := h.GetRecents()
	want := []string{"/c.yaml", "/a.yaml", "/b.yaml"}
	for i, w := range want {
		if got[i].Path != w || got[i].Order != i {
			t.Fatalf("order[%d] = %+v, want path %s order %d", i, got[i], w, i)
		}
	}
}
