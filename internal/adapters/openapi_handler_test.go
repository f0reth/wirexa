package adapters

import (
	"context"
	"errors"
	"testing"

	openapidomain "github.com/f0reth/Wirexa/internal/domain/openapi"
)

var _ OpenAPIFileUseCase = (*fakeFileUseCase)(nil)

// fakeFileUseCase は OpenAPIFileUseCase のテスト用実装。受け取った引数を記録する。
type fakeFileUseCase struct {
	calls   []string
	args    [][]any
	ret     string
	err     error
	recents []openapidomain.OpenAPIRecent
}

func (f *fakeFileUseCase) record(name string, args ...any) {
	f.calls = append(f.calls, name)
	f.args = append(f.args, args)
}

func (f *fakeFileUseCase) OpenSelected(path string) string {
	f.record("OpenSelected", path)
	return f.ret
}

func (f *fakeFileUseCase) SaveSelected(path, content string) (string, error) {
	f.record("SaveSelected", path, content)
	return f.ret, f.err
}

func (f *fakeFileUseCase) ReadFile(path string) (string, error) {
	f.record("ReadFile", path)
	return f.ret, f.err
}

func (f *fakeFileUseCase) WriteFile(path, content string) error {
	f.record("WriteFile", path, content)
	return f.err
}

func (f *fakeFileUseCase) GetRecents() []openapidomain.OpenAPIRecent {
	f.record("GetRecents")
	return f.recents
}

func (f *fakeFileUseCase) RemoveRecent(path string) error {
	f.record("RemoveRecent", path)
	return f.err
}

func (f *fakeFileUseCase) MoveRecent(path string, index int) error {
	f.record("MoveRecent", path, index)
	return f.err
}

func newOpenAPIHandler(d *fakeDialog, files *fakeFileUseCase) *OpenAPIHandler {
	h := &OpenAPIHandler{}
	SetupOpenAPIHandler(context.Background(), h, OpenAPIHandlerDeps{Files: files, Dialog: d})
	return h
}

// assertCall は i 番目の呼び出しが name(args...) であることを検証する。
func assertCall(t *testing.T, f *fakeFileUseCase, i int, name string, args ...any) {
	t.Helper()
	if len(f.calls) <= i {
		t.Fatalf("call[%d]: want %s, got calls %v", i, name, f.calls)
	}
	if f.calls[i] != name {
		t.Fatalf("call[%d] = %s, want %s", i, f.calls[i], name)
	}
	if len(f.args[i]) != len(args) {
		t.Fatalf("call[%d] args = %v, want %v", i, f.args[i], args)
	}
	for j := range args {
		if f.args[i][j] != args[j] {
			t.Fatalf("call[%d] args = %v, want %v", i, f.args[i], args)
		}
	}
}

func TestOpenAPIOpenFilePicker_CancelDoesNotCallUseCase(t *testing.T) {
	files := &fakeFileUseCase{}
	h := newOpenAPIHandler(&fakeDialog{}, files)

	got, err := h.OpenFilePicker()
	if got != "" || err != nil {
		t.Fatalf("OpenFilePicker on cancel = (%q, %v), want (\"\", nil)", got, err)
	}
	if len(files.calls) != 0 {
		t.Fatalf("use case must not be called on cancel: %v", files.calls)
	}
}

func TestOpenAPIOpenFilePicker_DialogErrorPassesThrough(t *testing.T) {
	dialogErr := errors.New("dialog failed")
	files := &fakeFileUseCase{}
	h := newOpenAPIHandler(&fakeDialog{openPath: "ignored.yaml", openErr: dialogErr}, files)

	got, err := h.OpenFilePicker()
	if got != "" || !errors.Is(err, dialogErr) {
		t.Fatalf("OpenFilePicker on dialog error = (%q, %v), want (\"\", %v)", got, err, dialogErr)
	}
	if len(files.calls) != 0 {
		t.Fatalf("use case must not be called on dialog error: %v", files.calls)
	}
}

func TestOpenAPIOpenFilePicker_PassesSelectedPath(t *testing.T) {
	files := &fakeFileUseCase{ret: "cleaned.yaml"}
	d := &fakeDialog{openPath: "selected.yaml"}
	h := newOpenAPIHandler(d, files)

	got, err := h.OpenFilePicker()
	if err != nil {
		t.Fatalf("OpenFilePicker: %v", err)
	}
	if got != "cleaned.yaml" {
		t.Fatalf("OpenFilePicker = %q, want use case result", got)
	}
	assertCall(t, files, 0, "OpenSelected", "selected.yaml")
	if len(d.openCalls) != 1 || len(d.openCalls[0].Filters) == 0 {
		t.Fatalf("dialog should be opened once with OpenAPI filters: %+v", d.openCalls)
	}
}

func TestOpenAPISaveFileAs_CancelDoesNotCallUseCase(t *testing.T) {
	files := &fakeFileUseCase{}
	h := newOpenAPIHandler(&fakeDialog{}, files)

	got, err := h.SaveFileAs("spec.yaml", "openapi: 3.1.0")
	if got != "" || err != nil {
		t.Fatalf("SaveFileAs on cancel = (%q, %v), want (\"\", nil)", got, err)
	}
	if len(files.calls) != 0 {
		t.Fatalf("use case must not be called on cancel: %v", files.calls)
	}
}

func TestOpenAPISaveFileAs_DialogErrorPassesThrough(t *testing.T) {
	dialogErr := errors.New("dialog failed")
	files := &fakeFileUseCase{}
	h := newOpenAPIHandler(&fakeDialog{savePath: "ignored.yaml", saveErr: dialogErr}, files)

	got, err := h.SaveFileAs("spec.yaml", "openapi: 3.1.0")
	if got != "" || !errors.Is(err, dialogErr) {
		t.Fatalf("SaveFileAs on dialog error = (%q, %v), want (\"\", %v)", got, err, dialogErr)
	}
	if len(files.calls) != 0 {
		t.Fatalf("use case must not be called on dialog error: %v", files.calls)
	}
}

func TestOpenAPISaveFileAs_PassesSelectedPathAndContent(t *testing.T) {
	writeErr := errors.New("write failed")
	files := &fakeFileUseCase{ret: "cleaned.yaml", err: writeErr}
	d := &fakeDialog{savePath: "selected.yaml"}
	h := newOpenAPIHandler(d, files)

	got, err := h.SaveFileAs("spec.yaml", "openapi: 3.1.0")
	if got != "cleaned.yaml" || !errors.Is(err, writeErr) {
		t.Fatalf("SaveFileAs = (%q, %v), want use case result", got, err)
	}
	assertCall(t, files, 0, "SaveSelected", "selected.yaml", "openapi: 3.1.0")
	if len(d.saveCalls) != 1 || d.saveCalls[0].DefaultFilename != "spec.yaml" {
		t.Fatalf("dialog should be opened once with the default name: %+v", d.saveCalls)
	}
}

func TestOpenAPIHandler_DelegatesToUseCase(t *testing.T) {
	useCaseErr := errors.New("use case error")
	recents := []openapidomain.OpenAPIRecent{{Path: "a.yaml", Name: "a.yaml"}}
	files := &fakeFileUseCase{ret: "content", err: useCaseErr, recents: recents}
	d := &fakeDialog{}
	h := newOpenAPIHandler(d, files)

	if got, err := h.ReadFile("r.yaml"); got != "content" || !errors.Is(err, useCaseErr) {
		t.Fatalf("ReadFile = (%q, %v)", got, err)
	}
	if err := h.WriteFile("w.yaml", "data"); !errors.Is(err, useCaseErr) {
		t.Fatalf("WriteFile = %v", err)
	}
	if got := h.GetRecents(); len(got) != 1 || got[0] != recents[0] {
		t.Fatalf("GetRecents = %+v", got)
	}
	if err := h.RemoveRecent("x.yaml"); !errors.Is(err, useCaseErr) {
		t.Fatalf("RemoveRecent = %v", err)
	}
	if err := h.MoveRecent("m.yaml", 3); !errors.Is(err, useCaseErr) {
		t.Fatalf("MoveRecent = %v", err)
	}

	assertCall(t, files, 0, "ReadFile", "r.yaml")
	assertCall(t, files, 1, "WriteFile", "w.yaml", "data")
	assertCall(t, files, 2, "GetRecents")
	assertCall(t, files, 3, "RemoveRecent", "x.yaml")
	assertCall(t, files, 4, "MoveRecent", "m.yaml", 3)
	if len(d.openCalls)+len(d.saveCalls) != 0 {
		t.Fatal("delegating methods must not open a dialog")
	}
}
