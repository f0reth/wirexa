package httpinfra

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	domain "github.com/f0reth/Wirexa/internal/domain/http"
)

// 開いている間は、他のハンドルからの書き込み・削除・リネームが失敗する。閉じた後はどれもできる。
func TestOpenSelectedFile_WindowsLocksWhileOpen(t *testing.T) {
	path := writeTempFile(t, "locked.txt", "original")
	r := NewFileRegistry()
	sel, err := r.Register(path)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	got, err := r.OpenSelectedFile(sel.Token)
	if err != nil {
		t.Fatalf("OpenSelectedFile: %v", err)
	}

	if err = os.WriteFile(path, []byte("changed"), 0o600); err == nil {
		t.Error("WriteFile succeeded while the file is open")
	}
	if err = os.Remove(path); err == nil {
		t.Error("Remove succeeded while the file is open")
	}
	renamed := filepath.Join(filepath.Dir(path), "renamed.txt")
	if err = os.Rename(path, renamed); err == nil {
		t.Error("Rename succeeded while the file is open")
	}
	// 読み取りは共有するので、他のハンドルからも読める。
	if data, rerr := os.ReadFile(path); rerr != nil || string(data) != "original" {
		t.Errorf("ReadFile while open = (%q, %v)", data, rerr)
	}
	if err = got.File.CheckUnchanged(); err != nil {
		t.Errorf("CheckUnchanged: %v", err)
	}

	if err = got.File.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err = os.WriteFile(path, []byte("changed"), 0o600); err != nil {
		t.Errorf("WriteFile after close: %v", err)
	}
	if err = os.Rename(path, renamed); err != nil {
		t.Errorf("Rename after close: %v", err)
	}
	if err = os.Remove(renamed); err != nil {
		t.Errorf("Remove after close: %v", err)
	}
}

// 他のハンドルが書き込み用に開いているファイルは ErrSelectedFileInUse で開かない。
func TestOpenSelectedFile_WindowsRejectsFileOpenForWriting(t *testing.T) {
	path := writeTempFile(t, "writing.log", "line 1\n")
	r := NewFileRegistry()
	sel, err := r.Register(path)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	w, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		t.Fatalf("OpenFile: %v", err)
	}

	_, err = r.OpenSelectedFile(sel.Token)
	if !errors.Is(err, domain.ErrSelectedFileInUse) {
		t.Fatalf("OpenSelectedFile while open for writing: want ErrSelectedFileInUse, got %v", err)
	}
	if strings.Contains(err.Error(), path) {
		t.Fatalf("error leaks the path: %q", err)
	}

	if err = w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	got, err := r.OpenSelectedFile(sel.Token)
	if err != nil {
		t.Fatalf("OpenSelectedFile after the writer closed: %v", err)
	}
	_ = got.File.Close()
}

func TestLongPath(t *testing.T) {
	long := `C:\` + strings.Repeat(`a\`, 130) + "f.txt"
	tests := []struct{ in, want string }{
		{in: `C:\short\f.txt`, want: `C:\short\f.txt`},
		{in: long, want: `\\?\` + long},
		{in: `\\server\share\` + long[3:], want: `\\?\UNC\server\share\` + long[3:]},
		{in: `\\?\` + long, want: `\\?\` + long},
	}
	for _, tc := range tests {
		if got := longPath(tc.in); got != tc.want {
			t.Errorf("longPath(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
