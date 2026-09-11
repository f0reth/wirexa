package adapters

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// fakeDialog は FileDialog のテスト用実装。呼び出し回数と渡された options を記録する。
type fakeDialog struct {
	openPath  string
	savePath  string
	openErr   error
	saveErr   error
	openCalls []runtime.OpenDialogOptions
	saveCalls []runtime.SaveDialogOptions
}

func (d *fakeDialog) OpenFile(_ context.Context, options runtime.OpenDialogOptions) (string, error) {
	d.openCalls = append(d.openCalls, options)
	return d.openPath, d.openErr
}

func (d *fakeDialog) SaveFile(_ context.Context, options runtime.SaveDialogOptions) (string, error) {
	d.saveCalls = append(d.saveCalls, options)
	return d.savePath, d.saveErr
}

func newDialogHandler(d *fakeDialog) *HTTPHandler {
	h := &HTTPHandler{}
	SetupHTTPHandler(context.Background(), h, HTTPHandlerDeps{Dialog: d})
	return h
}

func TestSaveResponseBase64_WritesToSelectedPath(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "out.json")
	d := &fakeDialog{savePath: dst}
	h := newDialogHandler(d)

	if err := h.SaveResponseBase64(base64.StdEncoding.EncodeToString([]byte("payload")), "application/json"); err != nil {
		t.Fatalf("SaveResponseBase64: %v", err)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read saved file: %v", err)
	}
	if string(got) != "payload" {
		t.Fatalf("saved content = %q, want %q", got, "payload")
	}
	if len(d.saveCalls) != 1 || d.saveCalls[0].DefaultFilename != "response.json" {
		t.Fatalf("save dialog options = %+v, want DefaultFilename response.json", d.saveCalls)
	}
}

func TestSaveResponseBase64_CancelWritesNothing(t *testing.T) {
	d := &fakeDialog{}
	h := newDialogHandler(d)

	if err := h.SaveResponseBase64(base64.StdEncoding.EncodeToString([]byte("x")), "text/plain"); err != nil {
		t.Fatalf("SaveResponseBase64 on cancel: %v", err)
	}
	if len(d.saveCalls) != 1 {
		t.Fatalf("save dialog calls = %d, want 1", len(d.saveCalls))
	}
}

func TestSaveResponseBase64_InvalidBase64SkipsDialog(t *testing.T) {
	d := &fakeDialog{}
	h := newDialogHandler(d)

	if err := h.SaveResponseBase64("!!not base64!!", "text/plain"); err == nil {
		t.Fatal("expected an error for invalid base64")
	}
	if len(d.saveCalls) != 0 {
		t.Fatalf("save dialog must not open for invalid input, got %d calls", len(d.saveCalls))
	}
}
