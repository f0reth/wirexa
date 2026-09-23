package httpinfra

import (
	"bytes"
	"crypto/rand"
	"errors"
	"io"
	"os"
	"strings"
	"sync"
	"testing"

	domain "github.com/f0reth/Wirexa/internal/domain/http"
)

// memBody は内容 data・基準のサイズ size のファイル区間を、バイト区間で挟んだボディを返す。
func memBody(h *memHandle, size int64) *bodySegments {
	b := &bodySegments{}
	_, _ = b.Write([]byte("<"))
	b.appendFile(domain.OpenedSelectedFile{File: h, Size: size})
	_, _ = b.Write([]byte(">"))
	return b
}

// readChunks は chunk バイトずつ読み、読めた内容と最後のエラーを返す。
func readChunks(r io.Reader, chunk int) ([]byte, error) {
	var out []byte
	buf := make([]byte, chunk)
	for {
		n, err := r.Read(buf)
		out = append(out, buf[:n]...)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return out, nil
			}
			return out, err
		}
	}
}

func failFrom(from int32, err error) func(n int32) error {
	return func(n int32) error {
		if n >= from {
			return err
		}
		return nil
	}
}

// 基準のサイズちょうどなら最後まで読め、区間を読み切るときに 1 回だけ変更を確かめる。
// 基準のサイズに達する前に EOF になったら ErrSelectedFileChanged にする。
func TestSegmentReader_FileSize(t *testing.T) {
	t.Run("ちょうど", func(t *testing.T) {
		h := &memHandle{data: []byte("hello")}
		b := memBody(h, 5)
		defer b.release()
		var errs bodyError
		rc, err := b.newReader(&errs)
		if err != nil {
			t.Fatalf("newReader: %v", err)
		}
		defer func() { _ = rc.Close() }()
		got, err := readChunks(rc, 2)
		if err != nil || string(got) != "<hello>" {
			t.Fatalf("read = (%q, %v), want <hello>", got, err)
		}
		if n := h.checks.Load(); n != 1 {
			t.Fatalf("CheckUnchanged called %d times, want 1", n)
		}
	})
	t.Run("短い", func(t *testing.T) {
		h := &memHandle{data: []byte("hel")}
		b := memBody(h, 5)
		defer b.release()
		var errs bodyError
		rc, err := b.newReader(&errs)
		if err != nil {
			t.Fatalf("newReader: %v", err)
		}
		defer func() { _ = rc.Close() }()
		got, err := readChunks(rc, 64)
		if !errors.Is(err, domain.ErrSelectedFileChanged) {
			t.Fatalf("err = %v, want ErrSelectedFileChanged", err)
		}
		if int64(len(got)) >= b.length {
			t.Fatalf("read %d bytes, want fewer than %d", len(got), b.length)
		}
		if !errors.Is(errs.get(), domain.ErrSelectedFileChanged) {
			t.Fatalf("bodyError = %v, want ErrSelectedFileChanged", errs.get())
		}
	})
	t.Run("空のファイル区間も変更を確かめる", func(t *testing.T) {
		h := &memHandle{data: []byte("x"), check: failFrom(1, domain.ErrSelectedFileChanged)}
		b := memBody(h, 0)
		defer b.release()
		rc, err := b.newReader(&bodyError{})
		if err != nil {
			t.Fatalf("newReader: %v", err)
		}
		defer func() { _ = rc.Close() }()
		if _, err = io.ReadAll(rc); !errors.Is(err, domain.ErrSelectedFileChanged) {
			t.Fatalf("err = %v, want ErrSelectedFileChanged", err)
		}
	})
}

// 読み切る直前の CheckUnchanged が失敗したら、最後のバイトを返さずにエラーにする。
func TestSegmentReader_WithholdsLastBytesOnChange(t *testing.T) {
	for _, chunk := range []int{1, 2, 64} {
		h := &memHandle{data: []byte("hello"), check: failFrom(1, domain.ErrSelectedFileChanged)}
		b := memBody(h, 5)
		var errs bodyError
		rc, err := b.newReader(&errs)
		if err != nil {
			t.Fatalf("newReader: %v", err)
		}
		got, err := readChunks(rc, chunk)
		if !errors.Is(err, domain.ErrSelectedFileChanged) {
			t.Fatalf("chunk %d: err = %v, want ErrSelectedFileChanged", chunk, err)
		}
		// 開始のバイト区間 "<" の後、ファイルの最後のバイト "o" より前までしか出さない。
		const upTo = "<hell"
		if len(got) > len(upTo) || string(got) != upTo[:len(got)] {
			t.Fatalf("chunk %d: read %q, want a prefix of %q", chunk, got, upTo)
		}
		if _, err = rc.Read(make([]byte, 8)); !errors.Is(err, domain.ErrSelectedFileChanged) {
			t.Fatalf("chunk %d: read after the failure = %v, want the same error", chunk, err)
		}
		_ = rc.Close()
		b.release()
	}
}

// ハンドルは、ボディ自身の参照とすべてのリーダーの参照が外れたときに 1 回だけ閉じる。
func TestSharedFile_ClosesOnceAfterAllReferences(t *testing.T) {
	h := &memHandle{data: []byte("abc")}
	b := memBody(h, 3)
	var errs bodyError
	first, err := b.newReader(&errs)
	if err != nil {
		t.Fatalf("newReader: %v", err)
	}
	second, err := b.getBody(&errs)()
	if err != nil {
		t.Fatalf("GetBody: %v", err)
	}

	b.release()
	b.release()
	if n := h.closes.Load(); n != 0 {
		t.Fatalf("closed while readers remain (%d)", n)
	}
	_ = first.Close()
	_ = first.Close()
	if n := h.closes.Load(); n != 0 {
		t.Fatalf("closed while a reader remains (%d)", n)
	}
	_ = second.Close()
	_ = second.Close()
	if n := h.closes.Load(); n != 1 {
		t.Fatalf("closed %d times, want 1", n)
	}
	// 閉じた後は新しいリーダーを作らない。
	if _, err = b.newReader(&errs); !errors.Is(err, domain.ErrSelectedFileUnavailable) {
		t.Fatalf("newReader after close = %v, want ErrSelectedFileUnavailable", err)
	}
}

// GetBody は送り直す前に変更を確かめ、失敗をそのまま返す。成功すれば同じ内容を先頭から返す。
func TestBodySegments_GetBody(t *testing.T) {
	t.Run("変更なし", func(t *testing.T) {
		h := &memHandle{data: []byte("abc")}
		b := memBody(h, 3)
		defer b.release()
		var errs bodyError
		getBody := b.getBody(&errs)
		for range 2 {
			rc, err := getBody()
			if err != nil {
				t.Fatalf("GetBody: %v", err)
			}
			got, err := io.ReadAll(rc)
			_ = rc.Close()
			if err != nil || string(got) != "<abc>" {
				t.Fatalf("read = (%q, %v), want <abc>", got, err)
			}
		}
	})
	t.Run("変更あり", func(t *testing.T) {
		h := &memHandle{data: []byte("abc"), check: failFrom(1, domain.ErrSelectedFileChanged)}
		b := memBody(h, 3)
		defer b.release()
		var errs bodyError
		if _, err := b.getBody(&errs)(); !errors.Is(err, domain.ErrSelectedFileChanged) {
			t.Fatalf("GetBody = %v, want ErrSelectedFileChanged", err)
		}
		if !errors.Is(errs.get(), domain.ErrSelectedFileChanged) {
			t.Fatalf("bodyError = %v, want ErrSelectedFileChanged", errs.get())
		}
	})
}

// 同じハンドルを 2 つのリーダーで並行して読んでも内容が混ざらない (-race でも確かめる)。
func TestSegmentReader_ConcurrentReaders(t *testing.T) {
	data := make([]byte, 1<<20)
	_, _ = rand.Read(data)
	h := &memHandle{data: data}
	b := memBody(h, int64(len(data)))
	defer b.release()
	want := append(append([]byte("<"), data...), '>')

	var errs bodyError
	getBody := b.getBody(&errs)
	var wg sync.WaitGroup
	for i := range 2 {
		rc, err := getBody()
		if err != nil {
			t.Fatalf("GetBody: %v", err)
		}
		wg.Go(func() {
			defer func() { _ = rc.Close() }()
			got, err := readChunks(rc, 1000+i*777)
			if err != nil || !bytes.Equal(got, want) {
				t.Errorf("reader #%d: err = %v, content matches = %v", i, err, bytes.Equal(got, want))
			}
		})
	}
	wg.Wait()
}

// bodyError には最初のエラーだけが残る。ファイルの読み込みエラーはパスを含まない domain エラーにする。
func TestBodyError_KeepsFirstAndHidesPaths(t *testing.T) {
	var errs bodyError
	errs.record(domain.ErrSelectedFileChanged)
	errs.record(domain.ErrSelectedFileUnavailable)
	if !errors.Is(errs.get(), domain.ErrSelectedFileChanged) {
		t.Fatalf("bodyError = %v, want the first error", errs.get())
	}

	const secret = `C:\secret\data.bin`
	h := &pathErrorHandle{err: &os.PathError{Op: "read", Path: secret, Err: errors.New("device error")}}
	b := &bodySegments{}
	b.appendFile(domain.OpenedSelectedFile{File: h, Size: 3})
	defer b.release()
	var errs2 bodyError
	rc, err := b.newReader(&errs2)
	if err != nil {
		t.Fatalf("newReader: %v", err)
	}
	defer func() { _ = rc.Close() }()
	_, err = io.ReadAll(rc)
	for _, e := range []error{err, errs2.get()} {
		if !errors.Is(e, domain.ErrSelectedFileUnavailable) || strings.Contains(e.Error(), secret) {
			t.Fatalf("error = %v, want ErrSelectedFileUnavailable without the path", e)
		}
	}
}

// pathErrorHandle は ReadAt が必ず err を返す SelectedFileHandle。
type pathErrorHandle struct {
	err error
	memHandle
}

func (h *pathErrorHandle) ReadAt([]byte, int64) (int, error) { return 0, h.err }
