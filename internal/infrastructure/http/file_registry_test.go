package httpinfra

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

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

func TestFileRegistry_RegisterAndRead(t *testing.T) {
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

	got, err := r.ReadSelectedFile(sel.Token)
	if err != nil {
		t.Fatalf("ReadSelectedFile: %v", err)
	}
	if string(got.Data) != `{"a":1}` || got.Name != "payload.json" {
		t.Fatalf("read = %+v", got)
	}
}

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
		if _, err := r.ReadSelectedFile(token); !errors.Is(err, domain.ErrFileAccessDenied) {
			t.Errorf("ReadSelectedFile(%q): want ErrFileAccessDenied, got %v", token, err)
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

	_, err = r.ReadSelectedFile(sel.Token)
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
			if _, err := r.ReadSelectedFile(sel.Token); err != nil {
				t.Errorf("ReadSelectedFile: %v", err)
			}
			_, _ = r.ReadSelectedFile(fmt.Sprintf("unknown-%d", i))
		})
	}
	wg.Wait()
}

func TestResolveFile(t *testing.T) {
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
		{name: "登録済み token は読む", ref: domain.FileReference{Token: sel.Token}, wantOK: true},
		{name: "未登録 token は拒否", ref: domain.FileReference{Token: "forged"}, wantErr: domain.ErrFileAccessDenied},
		{name: "再選択待ちは拒否", ref: domain.FileReference{Name: "a.txt", NeedsReselect: true}, wantErr: domain.ErrFileAccessDenied},
		{name: "名前だけの参照も拒否", ref: domain.FileReference{Name: "a.txt"}, wantErr: domain.ErrFileAccessDenied},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, ok, err := resolveFile(r, tc.ref)
			if !errors.Is(err, tc.wantErr) || ok != tc.wantOK {
				t.Fatalf("resolveFile = (ok=%v, err=%v), want (ok=%v, err=%v)", ok, err, tc.wantOK, tc.wantErr)
			}
		})
	}

	if _, _, err := resolveFile(nil, domain.FileReference{Token: sel.Token}); !errors.Is(err, domain.ErrFileAccessDenied) {
		t.Fatalf("resolveFile without a registry: want ErrFileAccessDenied, got %v", err)
	}
}
