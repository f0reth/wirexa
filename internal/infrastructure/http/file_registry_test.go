package httpinfra

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	domain "github.com/f0reth/Wirexa/internal/domain/http"
)

func writeTempFile(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

// openSelected は OpenSelectedFile で開き、テスト終了時に閉じる。
func openSelected(t *testing.T, r *FileRegistry, token string) domain.OpenedSelectedFile {
	t.Helper()
	got, err := r.OpenSelectedFile(token)
	if err != nil {
		t.Fatalf("OpenSelectedFile: %v", err)
	}
	t.Cleanup(func() { _ = got.File.Close() })
	return got
}

// readAll は開いたファイルを開いた時点のサイズ分だけ読む。
func readAll(t *testing.T, file domain.OpenedSelectedFile) string {
	t.Helper()
	data, err := io.ReadAll(io.NewSectionReader(file.File, 0, file.Size))
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	return string(data)
}

func TestFileRegistry_RegisterAndOpen(t *testing.T) {
	path := writeTempFile(t, "payload.json", `{"a":1}`)
	r := NewFileRegistry()

	sel, err := r.Register(path)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if !regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString(sel.Token) {
		t.Fatalf("token = %q, want 128-bit hex", sel.Token)
	}
	if sel.Name != "payload.json" || sel.ContentType != domain.GuessFileContentType(path) {
		t.Fatalf("selected = %+v", sel)
	}

	got := openSelected(t, r, sel.Token)
	if got.Name != "payload.json" || got.ContentType != sel.ContentType || got.Size != int64(len(`{"a":1}`)) {
		t.Fatalf("opened = %+v", got)
	}
	if body := readAll(t, got); body != `{"a":1}` {
		t.Fatalf("content = %q", body)
	}
	if err := got.File.CheckUnchanged(); err != nil {
		t.Fatalf("CheckUnchanged on an untouched file: %v", err)
	}
}

// ディレクトリは送れるファイルではないので、開けても ErrSelectedFileUnavailable にする。
func TestFileRegistry_RejectsDirectory(t *testing.T) {
	dir := t.TempDir()
	r := NewFileRegistry()
	sel, err := r.Register(dir)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	_, err = r.OpenSelectedFile(sel.Token)
	if !errors.Is(err, domain.ErrSelectedFileUnavailable) {
		t.Fatalf("want ErrSelectedFileUnavailable, got %v", err)
	}
	if strings.Contains(err.Error(), dir) {
		t.Fatalf("error leaks the path: %q", err)
	}
}

// CheckUnchanged は開いた時点からのサイズの変化と更新時刻の変化をそれぞれ検出する。
func TestSelectedFileHandle_CheckUnchangedDetectsChanges(t *testing.T) {
	t.Run("サイズの変化", func(t *testing.T) {
		src := &fakeSource{data: []byte("abc"), modTime: time.Unix(100, 0)}
		h := newSelectedFileHandle(src, src.info())
		src.data = []byte("abcd")
		if err := h.CheckUnchanged(); !errors.Is(err, domain.ErrSelectedFileChanged) {
			t.Fatalf("want ErrSelectedFileChanged, got %v", err)
		}
	})
	t.Run("更新時刻の変化", func(t *testing.T) {
		src := &fakeSource{data: []byte("abc"), modTime: time.Unix(100, 0)}
		h := newSelectedFileHandle(src, src.info())
		src.modTime = time.Unix(100, 1)
		if err := h.CheckUnchanged(); !errors.Is(err, domain.ErrSelectedFileChanged) {
			t.Fatalf("want ErrSelectedFileChanged, got %v", err)
		}
	})
	t.Run("実ファイルの更新時刻の変化 (os.Chtimes)", func(t *testing.T) {
		path := writeTempFile(t, "a.txt", "abc")
		r := NewFileRegistry()
		sel, err := r.Register(path)
		if err != nil {
			t.Fatalf("Register: %v", err)
		}
		got := openSelected(t, r, sel.Token)
		later := time.Now().Add(time.Hour)
		if err = os.Chtimes(path, later, later); err != nil {
			t.Fatalf("Chtimes: %v", err)
		}
		if err = got.File.CheckUnchanged(); !errors.Is(err, domain.ErrSelectedFileChanged) {
			t.Fatalf("want ErrSelectedFileChanged, got %v", err)
		}
	})
}

// OS がパスを含むエラーを返しても、ReadAt・CheckUnchanged・Close はパスを含まない
// ErrSelectedFileUnavailable に置き換える。io.EOF はそのまま返す。
func TestSelectedFileHandle_HidesPathErrors(t *testing.T) {
	const secret = `C:\secret\data.bin`
	pathErr := &os.PathError{Op: "read", Path: secret, Err: errors.New("device error")}
	src := &fakeSource{data: []byte("abc"), modTime: time.Unix(100, 0)}
	h := newSelectedFileHandle(src, src.info())

	buf := make([]byte, 8)
	if n, err := h.ReadAt(buf, 0); n != 3 || err != io.EOF { //nolint:errorlint // io.EOF そのものを返す契約を確かめる
		t.Fatalf("ReadAt past the end = (%d, %v), want (3, io.EOF)", n, err)
	}

	src.readErr, src.statErr, src.closeErr = pathErr, pathErr, pathErr
	_, readErr := h.ReadAt(buf, 0)
	for name, err := range map[string]error{
		"ReadAt":         readErr,
		"CheckUnchanged": h.CheckUnchanged(),
		"Close":          h.Close(),
	} {
		if !errors.Is(err, domain.ErrSelectedFileUnavailable) {
			t.Errorf("%s: want ErrSelectedFileUnavailable, got %v", name, err)
		}
		var pe *os.PathError
		if errors.As(err, &pe) || strings.Contains(err.Error(), secret) {
			t.Errorf("%s leaks the path: %q", name, err)
		}
	}
}

// fakeSource は selectedFileSource のテスト用実装。内容・更新時刻・各操作のエラーを決められる。
type fakeSource struct {
	modTime  time.Time
	readErr  error
	statErr  error
	closeErr error
	data     []byte
}

func (s *fakeSource) ReadAt(p []byte, off int64) (int, error) {
	if s.readErr != nil {
		return 0, s.readErr
	}
	return bytes.NewReader(s.data).ReadAt(p, off)
}

func (s *fakeSource) Stat() (os.FileInfo, error) {
	if s.statErr != nil {
		return nil, s.statErr
	}
	return s.info(), nil
}

func (s *fakeSource) Close() error { return s.closeErr }

func (s *fakeSource) info() os.FileInfo {
	return fakeFileInfo{size: int64(len(s.data)), modTime: s.modTime}
}

type fakeFileInfo struct {
	modTime time.Time
	size    int64
}

func (i fakeFileInfo) Name() string       { return "fake" }
func (i fakeFileInfo) Size() int64        { return i.size }
func (i fakeFileInfo) Mode() os.FileMode  { return 0o600 }
func (i fakeFileInfo) ModTime() time.Time { return i.modTime }
func (i fakeFileInfo) IsDir() bool        { return false }
func (i fakeFileInfo) Sys() any           { return nil }

func TestFileRegistry_ReusesTokenForSamePath(t *testing.T) {
	path := writeTempFile(t, "a.txt", "x")
	r := NewFileRegistry()
	first, err := r.Register(path)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	second, err := r.Register(filepath.Join(filepath.Dir(path), ".", "a.txt"))
	if err != nil {
		t.Fatalf("Register again: %v", err)
	}
	if first.Token != second.Token {
		t.Fatalf("re-selecting the same path issued a new token: %q vs %q", first.Token, second.Token)
	}
}

// ダイアログを通さずに作った任意の token、空、改ざん済みの token ではファイルを開かない。
func TestFileRegistry_RejectsUnknownTokens(t *testing.T) {
	path := writeTempFile(t, "a.txt", "secret")
	r := NewFileRegistry()
	sel, err := r.Register(path)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	tampered := []byte(sel.Token)
	tampered[0] ^= 1

	for _, token := range []string{"", "not-a-token", strings.Repeat("0", 32), string(tampered), path} {
		if _, err := r.OpenSelectedFile(token); !errors.Is(err, domain.ErrFileAccessDenied) {
			t.Errorf("OpenSelectedFile(%q): want ErrFileAccessDenied, got %v", token, err)
		}
	}
}

// 登録後に消えたファイルは access denied と区別し、エラーにパスを含めない。
func TestFileRegistry_UnavailableFileHidesPath(t *testing.T) {
	path := writeTempFile(t, "gone.txt", "x")
	r := NewFileRegistry()
	sel, err := r.Register(path)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if err = os.Remove(path); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	_, err = r.OpenSelectedFile(sel.Token)
	if !errors.Is(err, domain.ErrSelectedFileUnavailable) {
		t.Fatalf("want ErrSelectedFileUnavailable, got %v", err)
	}
	if strings.Contains(err.Error(), path) || strings.Contains(err.Error(), sel.Token) {
		t.Fatalf("error leaks the path or token: %q", err)
	}
}

func TestFileRegistry_RejectsSelectionsOverLimit(t *testing.T) {
	r := NewFileRegistry()
	dir := t.TempDir()
	for i := range maxSelectedFiles {
		if _, err := r.Register(filepath.Join(dir, fmt.Sprintf("f%d", i))); err != nil {
			t.Fatalf("Register #%d: %v", i, err)
		}
	}
	if _, err := r.Register(filepath.Join(dir, "overflow")); !errors.Is(err, domain.ErrFileSelectionLimit) {
		t.Fatalf("Register over the limit: want ErrFileSelectionLimit, got %v", err)
	}
	// 登録済みパスの再選択は上限に関係なく同じ token で通る。
	if _, err := r.Register(filepath.Join(dir, "f0")); err != nil {
		t.Fatalf("re-selecting a registered path: %v", err)
	}
	if len(r.byToken) != maxSelectedFiles {
		t.Fatalf("registry grew to %d entries", len(r.byToken))
	}
}

func TestFileRegistry_ConcurrentAccess(t *testing.T) {
	path := writeTempFile(t, "a.txt", "x")
	r := NewFileRegistry()
	var wg sync.WaitGroup
	for i := range 32 {
		wg.Go(func() {
			sel, err := r.Register(path)
			if err != nil {
				t.Errorf("Register: %v", err)
				return
			}
			got, err := r.OpenSelectedFile(sel.Token)
			if err != nil {
				t.Errorf("OpenSelectedFile: %v", err)
				return
			}
			_ = got.File.Close()
			_, _ = r.OpenSelectedFile(fmt.Sprintf("unknown-%d", i))
		})
	}
	wg.Wait()
}

func TestOpenFile(t *testing.T) {
	path := writeTempFile(t, "a.txt", "content")
	r := NewFileRegistry()
	sel, err := r.Register(path)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	tests := []struct {
		wantErr error
		name    string
		ref     domain.FileReference
		wantOK  bool
	}{
		{name: "未選択は送らない", ref: domain.FileReference{}},
		{name: "登録済み token は開く", ref: domain.FileReference{Token: sel.Token}, wantOK: true},
		{name: "未登録 token は拒否", ref: domain.FileReference{Token: "forged"}, wantErr: domain.ErrFileAccessDenied},
		{name: "再選択待ちは拒否", ref: domain.FileReference{Name: "a.txt", NeedsReselect: true}, wantErr: domain.ErrFileAccessDenied},
		{name: "名前だけの参照も拒否", ref: domain.FileReference{Name: "a.txt"}, wantErr: domain.ErrFileAccessDenied},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			file, ok, err := openFile(r, tc.ref)
			if ok {
				_ = file.File.Close()
			}
			if !errors.Is(err, tc.wantErr) || ok != tc.wantOK {
				t.Fatalf("openFile = (ok=%v, err=%v), want (ok=%v, err=%v)", ok, err, tc.wantOK, tc.wantErr)
			}
		})
	}

	if _, _, err := openFile(nil, domain.FileReference{Token: sel.Token}); !errors.Is(err, domain.ErrFileAccessDenied) {
		t.Fatalf("openFile without a registry: want ErrFileAccessDenied, got %v", err)
	}
}
