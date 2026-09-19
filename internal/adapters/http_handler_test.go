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
	// openFn が設定されていれば OpenFile はそれを使う (呼び出しごとに結果を変えるテスト用)。
	openFn    func(runtime.OpenDialogOptions) (string, error)
	openPath  string
	savePath  string
	openErr   error
	saveErr   error
	openCalls []runtime.OpenDialogOptions
	saveCalls []runtime.SaveDialogOptions
}

func (d *fakeDialog) OpenFile(_ context.Context, options runtime.OpenDialogOptions) (string, error) {
	d.openCalls = append(d.openCalls, options)
	if d.openFn != nil {
		return d.openFn(options)
	}
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

// fakeSelector は FileSelector のテスト用実装。登録されたパスを記録する。
type fakeSelector struct {
	err        error
	registered []string
}

func (s *fakeSelector) Register(path string) (httpdomain.SelectedFile, error) {
	s.registered = append(s.registered, path)
	if s.err != nil {
		return httpdomain.SelectedFile{}, s.err
	}
	return httpdomain.SelectedFile{Token: "tok-" + filepath.Base(path), Name: filepath.Base(path), ContentType: "text/plain"}, nil
}

func newPickerHandler(d *fakeDialog, sel *fakeSelector) *HTTPHandler {
	h := &HTTPHandler{}
	SetupHTTPHandler(context.Background(), h, HTTPHandlerDeps{Dialog: d, Files: sel})
	return h
}

// token はダイアログの戻り値だけから発行し、hint のパスは登録しない。
func TestOpenFilePicker_RegistersOnlyTheDialogResult(t *testing.T) {
	d := &fakeDialog{openPath: "/chosen/by-user.txt"}
	sel := &fakeSelector{}
	h := newPickerHandler(d, sel)

	got, err := h.OpenFilePicker("/etc/passwd")
	if err != nil {
		t.Fatalf("OpenFilePicker: %v", err)
	}
	if got.Token != "tok-by-user.txt" || got.Name != "by-user.txt" {
		t.Fatalf("selected = %+v", got)
	}
	if len(sel.registered) != 1 || sel.registered[0] != "/chosen/by-user.txt" {
		t.Fatalf("registered = %v, want only the dialog result", sel.registered)
	}
}

func TestOpenFilePicker_CancelRegistersNothing(t *testing.T) {
	sel := &fakeSelector{}
	h := newPickerHandler(&fakeDialog{}, sel)

	got, err := h.OpenFilePicker(filepath.Join(t.TempDir(), "typed.txt"))
	if err != nil || got.Token != "" {
		t.Fatalf("OpenFilePicker on cancel = (%+v, %v), want an empty selection", got, err)
	}
	if len(sel.registered) != 0 {
		t.Fatalf("cancel must not register anything, got %v", sel.registered)
	}
}

// hint はダイアログの初期位置にだけ使い、存在しないディレクトリや不正な値は既定位置へ落とす。
func TestOpenFilePicker_HintOnlySetsDialogLocation(t *testing.T) {
	dir := t.TempDir()
	tests := []struct {
		name     string
		hint     string
		wantDir  string
		wantFile string
	}{
		{name: "既存ディレクトリ", hint: dir, wantDir: dir},
		{name: "既存ディレクトリ内のファイル名", hint: filepath.Join(dir, "a.png"), wantDir: dir, wantFile: "a.png"},
		{name: "前後の空白は無視", hint: "  " + filepath.Join(dir, "a.png") + "\n", wantDir: dir, wantFile: "a.png"},
		{name: "存在しないディレクトリは捨てる", hint: filepath.Join(dir, "missing", "a.png")},
		{name: "空文字", hint: ""},
		{name: "相対パスは捨てる", hint: "relative/a.png"},
		{name: "極端に長い文字列は捨てる", hint: "/" + strings.Repeat("a", maxHintLength)},
		{name: "NUL を含む文字列は捨てる", hint: dir + "\x00"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := &fakeDialog{}
			h := newPickerHandler(d, &fakeSelector{})
			if _, err := h.OpenFilePicker(tc.hint); err != nil {
				t.Fatalf("OpenFilePicker: %v", err)
			}
			if len(d.openCalls) != 1 {
				t.Fatalf("dialog calls = %d, want 1", len(d.openCalls))
			}
			got := d.openCalls[0]
			if got.DefaultDirectory != tc.wantDir || got.DefaultFilename != tc.wantFile {
				t.Fatalf("dialog location = (%q, %q), want (%q, %q)", got.DefaultDirectory, got.DefaultFilename, tc.wantDir, tc.wantFile)
			}
		})
	}
}

// hint 由来でダイアログが失敗したら既定位置で開き直し、Wails のエラー (実パスを含む) は透過させない。
func TestOpenFilePicker_RetriesWithoutHintAndHidesDialogErrors(t *testing.T) {
	dir := t.TempDir()
	d := &fakeDialog{openFn: func(o runtime.OpenDialogOptions) (string, error) {
		if o.DefaultDirectory != "" {
			return "", errors.New("default directory '" + o.DefaultDirectory + "' does not exist")
		}
		return "/chosen/file.txt", nil
	}}
	sel := &fakeSelector{}
	h := newPickerHandler(d, sel)

	got, err := h.OpenFilePicker(filepath.Join(dir, "a.png"))
	if err != nil || got.Token == "" {
		t.Fatalf("OpenFilePicker = (%+v, %v), want a selection after retry", got, err)
	}
	if len(d.openCalls) != 2 || d.openCalls[1].DefaultDirectory != "" {
		t.Fatalf("expected a retry at the default location, calls = %+v", d.openCalls)
	}

	d.openFn = func(runtime.OpenDialogOptions) (string, error) {
		return "", errors.New("default directory '" + dir + "' does not exist")
	}
	_, err = h.OpenFilePicker(filepath.Join(dir, "a.png"))
	if !errors.Is(err, errFilePicker) || strings.Contains(err.Error(), dir) {
		t.Fatalf("want a classified error without the hint path, got %v", err)
	}
}

func TestOpenFilePicker_RegistryErrorPropagates(t *testing.T) {
	h := newPickerHandler(&fakeDialog{openPath: "/chosen/a.txt"}, &fakeSelector{err: httpdomain.ErrFileSelectionLimit})
	if _, err := h.OpenFilePicker(""); !errors.Is(err, httpdomain.ErrFileSelectionLimit) {
		t.Fatalf("want ErrFileSelectionLimit, got %v", err)
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
