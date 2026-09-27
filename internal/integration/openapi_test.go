//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/f0reth/Wirexa/internal/adapters"
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

// newOpenAPIHandler は SetupOpenAPIHandler で OpenAPIHandler を組み立てる。
// dialog に nil を渡すと Wails のダイアログが使われるので、必ずテスト用のダイアログを渡す。
func newOpenAPIHandler(t *testing.T, recentsPath string, dialog *fileDialog) *adapters.OpenAPIHandler {
	t.Helper()
	h := &adapters.OpenAPIHandler{}
	adapters.SetupOpenAPIHandler(context.Background(), h, adapters.OpenAPIHandlerDeps{
		Files:  newOpenAPIFileService(recentsPath),
		Dialog: dialog,
	})
	return h
}

// writeSpec は dir/name に content を書いてパスを返す。
func writeSpec(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return p
}

// openSpec はダイアログで p を選んだものとして OpenFilePicker を呼ぶ。
func openSpec(t *testing.T, h *adapters.OpenAPIHandler, dialog *fileDialog, p string) string {
	t.Helper()
	dialog.path = p
	opened, err := h.OpenFilePicker()
	if err != nil {
		t.Fatalf("OpenFilePicker: %v", err)
	}
	return opened
}

// recentPaths は GetRecents のパスを並び順に返す。
func recentPaths(h *adapters.OpenAPIHandler) []string {
	var paths []string
	for _, r := range h.GetRecents() {
		paths = append(paths, r.Path)
	}
	return paths
}

// assertFileContent は p の中身が want であることを確かめる。
func assertFileContent(t *testing.T, p, want string) {
	t.Helper()
	got, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", p, err)
	}
	if string(got) != want {
		t.Errorf("%s = %q, want %q", filepath.Base(p), got, want)
	}
}

// TestOpenAPI_HandlerOpenSaveReadWrite は Handler を通して、ダイアログで開いた・保存したファイルだけが
// 許可リストと recents に入り、実ファイルを読み書きできることと、キャンセルでは何も変わらないことを確認する。
func TestOpenAPI_HandlerOpenSaveReadWrite(t *testing.T) {
	dir := t.TempDir()
	recentsPath := filepath.Join(dir, "openapi-recents.json")
	spec := writeSpec(t, dir, "spec.yaml", "openapi: 3.0.0")
	dialog := &fileDialog{}
	h := newOpenAPIHandler(t, recentsPath, dialog)

	opened := openSpec(t, h, dialog, spec)
	if opened != spec {
		t.Errorf("OpenFilePicker = %q, want %q", opened, spec)
	}
	got, err := h.ReadFile(opened)
	if err != nil || got != "openapi: 3.0.0" {
		t.Fatalf("ReadFile = (%q, %v), want the file content", got, err)
	}
	if err = h.WriteFile(opened, "openapi: 3.1.0"); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	assertFileContent(t, spec, "openapi: 3.1.0")

	dialog.savePath = filepath.Join(dir, "saved.yaml")
	saved, err := h.SaveFileAs("spec.yaml", "openapi: 3.1.1")
	if err != nil {
		t.Fatalf("SaveFileAs: %v", err)
	}
	if saved != dialog.savePath {
		t.Errorf("SaveFileAs = %q, want %q", saved, dialog.savePath)
	}
	assertFileContent(t, saved, "openapi: 3.1.1")
	if got, err = h.ReadFile(saved); err != nil || got != "openapi: 3.1.1" {
		t.Errorf("ReadFile(saved) = (%q, %v)", got, err)
	}
	want := []string{spec, saved}
	if got := recentPaths(h); !slices.Equal(got, want) {
		t.Errorf("recents = %v, want %v", got, want)
	}

	// キャンセルは空のパスとエラー無しを返し、recents も変わらない。
	dialog.path, dialog.savePath = "", ""
	if p, perr := h.OpenFilePicker(); p != "" || perr != nil {
		t.Errorf("OpenFilePicker (canceled) = (%q, %v), want an empty path and no error", p, perr)
	}
	if p, perr := h.SaveFileAs("spec.yaml", "x"); p != "" || perr != nil {
		t.Errorf("SaveFileAs (canceled) = (%q, %v), want an empty path and no error", p, perr)
	}
	if got := recentPaths(h); !slices.Equal(got, want) {
		t.Errorf("recents after cancel = %v, want %v", got, want)
	}
}

// TestOpenAPI_RemoveAndMoveRecent_SurviveRestart は recents の並び替えと削除が保存され、
// 削除したパスは再起動後の seed でも許可されないことを確認する。
func TestOpenAPI_RemoveAndMoveRecent_SurviveRestart(t *testing.T) {
	dir := t.TempDir()
	recentsPath := filepath.Join(dir, "openapi-recents.json")
	a := writeSpec(t, dir, "a.yaml", "a")
	b := writeSpec(t, dir, "b.yaml", "b")
	c := writeSpec(t, dir, "c.yaml", "c")
	dialog := &fileDialog{}
	h1 := newOpenAPIHandler(t, recentsPath, dialog)
	for _, p := range []string{a, b, c} {
		openSpec(t, h1, dialog, p)
	}

	if err := h1.MoveRecent(c, 0); err != nil {
		t.Fatalf("MoveRecent: %v", err)
	}
	if err := h1.RemoveRecent(a); err != nil {
		t.Fatalf("RemoveRecent: %v", err)
	}
	if _, err := h1.ReadFile(a); !errors.Is(err, openapidomain.ErrFileAccessDenied) {
		t.Errorf("ReadFile(removed) in the same session: want ErrFileAccessDenied, got %v", err)
	}

	h2 := newOpenAPIHandler(t, recentsPath, &fileDialog{})
	if got, want := recentPaths(h2), []string{c, b}; !slices.Equal(got, want) {
		t.Errorf("recents after restart = %v, want %v", got, want)
	}
	if _, err := h2.ReadFile(a); !errors.Is(err, openapidomain.ErrFileAccessDenied) {
		t.Errorf("ReadFile(removed) after restart: want ErrFileAccessDenied, got %v", err)
	}
	if got, err := h2.ReadFile(b); err != nil || got != "b" {
		t.Errorf("ReadFile(kept) after restart = (%q, %v)", got, err)
	}
}

// TestOpenAPI_UnreadableRecents_StartsEmptyWithoutSaving は、openapi-recents.json が破損以外の理由で
// 読めないとき (ここではディレクトリ) 空の recents で始め、このセッションでは保存も退避もしないことと、
// 壊れたファイルの退避先が既にあるときは上書きせず別名へ退避することを確認する。
func TestOpenAPI_UnreadableRecents_StartsEmptyWithoutSaving(t *testing.T) {
	dir := t.TempDir()
	recentsPath := filepath.Join(dir, "openapi-recents.json")
	if err := os.Mkdir(recentsPath, 0o750); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	spec := writeSpec(t, dir, "spec.yaml", "openapi: 3.0.0")
	dialog := &fileDialog{}
	h := newOpenAPIHandler(t, recentsPath, dialog)
	if got := h.GetRecents(); len(got) != 0 {
		t.Fatalf("recents = %v, want empty", got)
	}
	opened := openSpec(t, h, dialog, spec)
	if got, err := h.ReadFile(opened); err != nil || got != "openapi: 3.0.0" {
		t.Errorf("ReadFile in this session = (%q, %v)", got, err)
	}
	if info, err := os.Stat(recentsPath); err != nil || !info.IsDir() {
		t.Errorf("openapi-recents.json should be left as is: %v", err)
	}
	if _, err := os.Stat(recentsPath + ".corrupt"); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("an unreadable (not corrupt) file must not be quarantined: %v", err)
	}
	// 保存していないので、次のセッションには引き継がれない。
	next := newOpenAPIHandler(t, recentsPath, &fileDialog{})
	if _, err := next.ReadFile(opened); !errors.Is(err, openapidomain.ErrFileAccessDenied) {
		t.Errorf("ReadFile in the next session: want ErrFileAccessDenied, got %v", err)
	}

	// 退避先の衝突: 既存の .corrupt は上書きせず、別名へ退避する。
	other := filepath.Join(t.TempDir(), "openapi-recents.json")
	const broken = "{ not json"
	if err := os.WriteFile(other+".corrupt", []byte("old"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.WriteFile(other, []byte(broken), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	newOpenAPIHandler(t, other, &fileDialog{})
	assertFileContent(t, other+".corrupt", "old")
	moved, err := filepath.Glob(other + ".corrupt.*")
	if err != nil || len(moved) != 1 {
		t.Fatalf("quarantined files = %v (%v), want 1", moved, err)
	}
	assertFileContent(t, moved[0], broken)
}

// TestOpenAPI_UngrantedPathsRejected は、ダイアログを通していない実在のパスと、.. でたどった
// 別のファイルへの ReadFile / WriteFile が ErrFileAccessDenied になり、ファイルが変わらないことを確認する。
func TestOpenAPI_UngrantedPathsRejected(t *testing.T) {
	dir := t.TempDir()
	spec := writeSpec(t, dir, "spec.yaml", "openapi: 3.0.0")
	secret := writeSpec(t, dir, "secret.yaml", "secret")
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o750); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	dialog := &fileDialog{}
	h := newOpenAPIHandler(t, filepath.Join(dir, "openapi-recents.json"), dialog)
	openSpec(t, h, dialog, spec)

	sep := string(filepath.Separator)
	for _, p := range []string{secret, sub + sep + ".." + sep + "secret.yaml"} {
		if _, err := h.ReadFile(p); !errors.Is(err, openapidomain.ErrFileAccessDenied) {
			t.Errorf("ReadFile(%s): want ErrFileAccessDenied, got %v", p, err)
		}
		if err := h.WriteFile(p, "overwritten"); !errors.Is(err, openapidomain.ErrFileAccessDenied) {
			t.Errorf("WriteFile(%s): want ErrFileAccessDenied, got %v", p, err)
		}
	}
	assertFileContent(t, secret, "secret")
}
