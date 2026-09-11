package adapters

import (
	"context"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	httpdomain "github.com/f0reth/Wirexa/internal/domain/http"
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

// fakeResponseStore は ResponseBodyStore のテスト用実装。
type fakeResponseStore struct {
	lease      *fakeLease
	acquireErr error
	acquired   []string
	discarded  []string
}

func (s *fakeResponseStore) AcquireSave(executionID string) (httpdomain.ResponseBodyLease, error) {
	s.acquired = append(s.acquired, executionID)
	if s.acquireErr != nil {
		return nil, s.acquireErr
	}
	return s.lease, nil
}

func (s *fakeResponseStore) Discard(executionID string) error {
	s.discarded = append(s.discarded, executionID)
	return nil
}

type fakeLease struct {
	saveErr     error
	contentType string
	savedTo     []string
	released    int
}

func (l *fakeLease) ContentType() string { return l.contentType }

func (l *fakeLease) SaveTo(dst string) error {
	l.savedTo = append(l.savedTo, dst)
	return l.saveErr
}

func (l *fakeLease) Release() { l.released++ }

func newResponseHandler(d *fakeDialog, store *fakeResponseStore) *HTTPHandler {
	h := &HTTPHandler{}
	SetupHTTPHandler(context.Background(), h, HTTPHandlerDeps{Dialog: d, Responses: store})
	return h
}

// 未追跡・処理中の ID は保存ダイアログを開く前に拒否する。
func TestSaveResponseBody_RejectsBeforeDialog(t *testing.T) {
	for _, storeErr := range []error{httpdomain.ErrResponseUnavailable, httpdomain.ErrResponseBusy} {
		t.Run(storeErr.Error(), func(t *testing.T) {
			d := &fakeDialog{savePath: "/tmp/out"}
			h := newResponseHandler(d, &fakeResponseStore{acquireErr: storeErr})

			saved, err := h.SaveResponseBody("exec-secret-id")
			if !errors.Is(err, storeErr) || saved {
				t.Fatalf("SaveResponseBody = (%v, %v), want (false, %v)", saved, err, storeErr)
			}
			if strings.Contains(err.Error(), "exec-secret-id") {
				t.Fatalf("error must not echo the execution ID: %q", err)
			}
			if len(d.saveCalls) != 0 {
				t.Fatalf("save dialog must not open, got %d calls", len(d.saveCalls))
			}
		})
	}
}

func TestSaveResponseBody_CancelReleasesLease(t *testing.T) {
	d := &fakeDialog{}
	lease := &fakeLease{contentType: "text/csv"}
	h := newResponseHandler(d, &fakeResponseStore{lease: lease})

	saved, err := h.SaveResponseBody("exec-1")
	if err != nil || saved {
		t.Fatalf("SaveResponseBody on cancel = (%v, %v), want (false, nil)", saved, err)
	}
	if lease.released != 1 || len(lease.savedTo) != 0 {
		t.Fatalf("lease released=%d savedTo=%v, want released once and no save", lease.released, lease.savedTo)
	}
	// 拡張子は呼び出し側ではなく、追跡中の Content-Type から決める。
	if got := d.saveCalls[0].DefaultFilename; got != "response.csv" {
		t.Fatalf("DefaultFilename = %q, want response.csv", got)
	}
}

func TestSaveResponseBody_SavesToSelectedPath(t *testing.T) {
	d := &fakeDialog{savePath: "/chosen/out.json"}
	lease := &fakeLease{contentType: "application/json"}
	store := &fakeResponseStore{lease: lease}
	h := newResponseHandler(d, store)

	saved, err := h.SaveResponseBody("exec-1")
	if err != nil || !saved {
		t.Fatalf("SaveResponseBody = (%v, %v), want (true, nil)", saved, err)
	}
	if len(store.acquired) != 1 || store.acquired[0] != "exec-1" {
		t.Fatalf("acquired = %v, want [exec-1]", store.acquired)
	}
	if len(lease.savedTo) != 1 || lease.savedTo[0] != "/chosen/out.json" {
		t.Fatalf("savedTo = %v, want [/chosen/out.json]", lease.savedTo)
	}
	if lease.released != 0 {
		t.Fatalf("a completed save must not release the lease")
	}
}

func TestSaveResponseBody_DialogErrorIsClassified(t *testing.T) {
	d := &fakeDialog{saveErr: errors.New("default directory '/Users/secret' does not exist")}
	lease := &fakeLease{}
	h := newResponseHandler(d, &fakeResponseStore{lease: lease})

	_, err := h.SaveResponseBody("exec-1")
	if !errors.Is(err, errSaveDialog) {
		t.Fatalf("want errSaveDialog, got %v", err)
	}
	if strings.Contains(err.Error(), "/Users/secret") {
		t.Fatalf("error must not leak the dialog path: %q", err)
	}
	if lease.released != 1 {
		t.Fatalf("lease must be released after a dialog failure")
	}
}

func TestSaveResponseBody_SaveErrorPropagates(t *testing.T) {
	d := &fakeDialog{savePath: "/chosen/out"}
	lease := &fakeLease{saveErr: httpdomain.ErrSaveResponseFailed}
	h := newResponseHandler(d, &fakeResponseStore{lease: lease})

	if _, err := h.SaveResponseBody("exec-1"); !errors.Is(err, httpdomain.ErrSaveResponseFailed) {
		t.Fatalf("want ErrSaveResponseFailed, got %v", err)
	}
}

func TestDiscardResponseBody_Delegates(t *testing.T) {
	store := &fakeResponseStore{}
	h := newResponseHandler(&fakeDialog{}, store)

	if err := h.DiscardResponseBody("exec-1"); err != nil {
		t.Fatalf("DiscardResponseBody: %v", err)
	}
	if len(store.discarded) != 1 || store.discarded[0] != "exec-1" {
		t.Fatalf("discarded = %v, want [exec-1]", store.discarded)
	}
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
