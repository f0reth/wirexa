package httpinfra

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	domain "github.com/f0reth/Wirexa/internal/domain/http"
)

// captureServer は受信したボディと Content-Type を記録するテストサーバを起動する。
func captureServer(t *testing.T) (url string, body, contentType *string, hits *atomic.Int32) {
	t.Helper()
	body, contentType, hits = new(string), new(string), new(atomic.Int32)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		data, _ := io.ReadAll(r.Body)
		*body = string(data)
		*contentType = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	return srv.URL, body, contentType, hits
}

// file body は token を registry で解決したファイルを送り、名前と Content-Type も registry の値を使う。
func TestNetClient_FileBodyUsesRegistry(t *testing.T) {
	path := writeTempFile(t, "payload.txt", "file bytes")
	reg := NewFileRegistry()
	sel, err := reg.Register(path)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	url, body, contentType, _ := captureServer(t)

	_, err = NewNetClient(reg, t.TempDir()).Do(context.Background(), "exec-1", domain.HTTPRequest{
		ID:     "file-1",
		Method: http.MethodPost,
		URL:    url,
		Body: domain.RequestBody{
			Type: domain.BodyTypeFile,
			File: domain.FileReference{Token: sel.Token, Name: "spoofed.exe", ContentType: "application/x-spoofed"},
		},
	})
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if *body != "file bytes" {
		t.Fatalf("body = %q, want the selected file", *body)
	}
	if *contentType != domain.GuessFileContentType(path) {
		t.Fatalf("Content-Type = %q, want the registry's %q", *contentType, domain.GuessFileContentType(path))
	}
}

// Contents に生のパスを入れても読まない (token が無ければ空ボディ)。
func TestNetClient_FileBodyIgnoresRawPath(t *testing.T) {
	path := writeTempFile(t, "secret.txt", "top secret")
	url, body, _, hits := captureServer(t)

	_, err := NewNetClient(NewFileRegistry(), t.TempDir()).Do(context.Background(), "exec-1", domain.HTTPRequest{
		ID:     "file-raw",
		Method: http.MethodPost,
		URL:    url,
		Body:   domain.RequestBody{Type: domain.BodyTypeFile, Contents: map[string]string{domain.BodyTypeFile: path}},
	})
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if hits.Load() != 1 || *body != "" {
		t.Fatalf("server got body %q, want an empty body", *body)
	}
}

// 解決できない参照はファイルを開く前に拒否し、リクエスト自体を送らない。
func TestNetClient_FileBodyRejectsUnresolvedReferences(t *testing.T) {
	url, _, _, hits := captureServer(t)
	c := NewNetClient(NewFileRegistry(), t.TempDir())

	for i, ref := range []domain.FileReference{
		{Token: "00112233445566778899aabbccddeeff"},
		{Name: "old.bin", NeedsReselect: true},
	} {
		_, err := c.Do(context.Background(), "exec-"+string(rune('a'+i)), domain.HTTPRequest{
			ID:     "denied-" + string(rune('a'+i)),
			Method: http.MethodPost,
			URL:    url,
			Body:   domain.RequestBody{Type: domain.BodyTypeFile, File: ref},
		})
		if !errors.Is(err, domain.ErrFileAccessDenied) {
			t.Fatalf("ref %+v: want ErrFileAccessDenied, got %v", ref, err)
		}
	}
	if hits.Load() != 0 {
		t.Fatalf("denied requests must not reach the server, got %d hits", hits.Load())
	}
}

// received は受信側が見たリクエスト 1 件分。
type received struct {
	err              error
	body             []byte
	transferEncoding []string
	contentLength    int64
	protoMajor       int
}

// recordingServer は受信したリクエストを ch へ流すテストサーバを起動する。
// ボディを最後まで読んでから (途中で切れたらそこまでで) 流す。
func recordingServer(t *testing.T, h2 bool) (*httptest.Server, chan received) {
	t.Helper()
	ch := make(chan received, 8)
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, err := io.ReadAll(r.Body)
		ch <- received{
			err:              err,
			body:             data,
			transferEncoding: r.TransferEncoding,
			contentLength:    r.ContentLength,
			protoMajor:       r.ProtoMajor,
		}
		w.WriteHeader(http.StatusOK)
	}))
	if h2 {
		srv.EnableHTTP2 = true
		srv.StartTLS()
	} else {
		srv.Start()
	}
	t.Cleanup(srv.Close)
	return srv, ch
}

func receive(t *testing.T, ch chan received) received {
	t.Helper()
	select {
	case r := <-ch:
		return r
	case <-time.After(10 * time.Second):
		t.Fatal("the server received no request")
		return received{}
	}
}

// handleOpener は決めたハンドルを返す SelectedFileOpener。
type handleOpener struct {
	h    domain.SelectedFileHandle
	size int64
}

func (o *handleOpener) OpenSelectedFile(string) (domain.OpenedSelectedFile, error) {
	return domain.OpenedSelectedFile{File: o.h, Name: "f.bin", ContentType: "application/octet-stream", Size: o.size}, nil
}

// countingOpener は内側の SelectedFileOpener で開いた回数を数える。
type countingOpener struct {
	inner domain.SelectedFileOpener
	opens atomic.Int32
}

func (o *countingOpener) OpenSelectedFile(token string) (domain.OpenedSelectedFile, error) {
	o.opens.Add(1)
	return o.inner.OpenSelectedFile(token)
}

func fileRequest(url, token string) domain.HTTPRequest {
	return domain.HTTPRequest{
		Method: http.MethodPost,
		URL:    url,
		Body:   domain.RequestBody{Type: domain.BodyTypeFile, File: domain.FileReference{Token: token}},
	}
}

func formRequest(url, token string) domain.HTTPRequest {
	return domain.HTTPRequest{
		Method: http.MethodPost,
		URL:    url,
		Body: domain.RequestBody{Type: domain.BodyTypeFormData, FormData: []domain.FormRow{
			{Key: "t", Value: "v", Enabled: true},
			{Key: "f", File: domain.FileReference{Token: token}, Kind: domain.FormRowKindFile, Enabled: true},
		}},
	}
}

// file body も multipart も Content-Length 付きの固定長で送り、chunked にしない。
func TestNetClient_StreamedBodyHasContentLength(t *testing.T) {
	content := strings.Repeat("x", 100_000)
	path := writeTempFile(t, "payload.bin", content)
	reg := NewFileRegistry()
	sel, err := reg.Register(path)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	srv, ch := recordingServer(t, false)
	c := NewNetClient(reg, t.TempDir())

	for name, req := range map[string]domain.HTTPRequest{
		"file body": fileRequest(srv.URL, sel.Token),
		"multipart": formRequest(srv.URL, sel.Token),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := c.Do(context.Background(), "exec-"+name, req); err != nil {
				t.Fatalf("Do: %v", err)
			}
			got := receive(t, ch)
			if got.err != nil || got.contentLength != int64(len(got.body)) || len(got.transferEncoding) != 0 {
				t.Fatalf("received Content-Length %d, %d bytes, Transfer-Encoding %v, err %v",
					got.contentLength, len(got.body), got.transferEncoding, got.err)
			}
			if !bytes.Contains(got.body, []byte(content)) {
				t.Fatal("the body does not contain the file")
			}
		})
	}
}

// 307 のリダイレクト先にも同じボディが届き、その間ファイルは 1 回しか開かない。
// 送信後はハンドルが閉じられ、ファイルを削除できる。
func TestNetClient_StreamedBodyFollows307WithSameHandle(t *testing.T) {
	path := writeTempFile(t, "payload.txt", "redirected bytes")
	reg := NewFileRegistry()
	sel, err := reg.Register(path)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	var final atomic.Value
	mux := http.NewServeMux()
	mux.HandleFunc("/start", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		http.Redirect(w, r, "/final", http.StatusTemporaryRedirect)
	})
	mux.HandleFunc("/final", func(_ http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		final.Store(string(data))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	opener := &countingOpener{inner: reg}

	for name, req := range map[string]domain.HTTPRequest{
		"file body": fileRequest(srv.URL+"/start", sel.Token),
		"multipart": formRequest(srv.URL+"/start", sel.Token),
	} {
		t.Run(name, func(t *testing.T) {
			opener.opens.Store(0)
			final.Store("")
			if _, derr := NewNetClient(opener, t.TempDir()).Do(context.Background(), "exec-1", req); derr != nil {
				t.Fatalf("Do: %v", derr)
			}
			if got, _ := final.Load().(string); !strings.Contains(got, "redirected bytes") {
				t.Fatalf("redirect target got %q, want the file", got)
			}
			if n := opener.opens.Load(); n != 1 {
				t.Fatalf("opened the file %d times, want 1", n)
			}
		})
	}

	// ハンドルが残っていれば、削除を共有しないで開く Windows では消せない。
	// トランスポートは Do が戻った後にボディを閉じることがあるので、少し待つ。
	deadline := time.Now().Add(5 * time.Second)
	for {
		if err = os.Remove(path); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the file is still open after sending: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// 送信後、ハンドルは 1 回だけ閉じられる。
func TestNetClient_StreamedBodyClosesHandle(t *testing.T) {
	srv, ch := recordingServer(t, false)
	h := &memHandle{data: []byte("abc")}
	opener := &handleOpener{h: h, size: 3}
	if _, err := NewNetClient(opener, t.TempDir()).Do(context.Background(), "exec-1", fileRequest(srv.URL, "tok")); err != nil {
		t.Fatalf("Do: %v", err)
	}
	receive(t, ch)
	waitClosed(t, h)
}

// waitClosed はハンドルが閉じられるのを待ち、1 回だけ閉じられたことを確かめる。
// トランスポートは Do が戻った後に別の goroutine でボディを閉じることがある。
func waitClosed(t *testing.T, h *memHandle) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for h.closes.Load() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("the handle was not closed")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if n := h.closes.Load(); n != 1 {
		t.Fatalf("the handle was closed %d times, want 1", n)
	}
}

// 大きなファイルを送っても、メモリの割り当てはファイルサイズに比例しない。
func TestNetClient_StreamedBodyDoesNotBufferFile(t *testing.T) {
	const size = 64 << 20
	path := filepath.Join(t.TempDir(), "large.bin")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	chunk := bytes.Repeat([]byte("0123456789abcdef"), 1<<16) // 1 MiB
	for range size / len(chunk) {
		if _, err = f.Write(chunk); err != nil {
			t.Fatalf("Write: %v", err)
		}
	}
	if err = f.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	reg := NewFileRegistry()
	sel, err := reg.Register(path)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	var got atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		n, _ := io.Copy(io.Discard, r.Body)
		got.Store(n)
	}))
	t.Cleanup(srv.Close)
	c := NewNetClient(reg, t.TempDir())

	for name, req := range map[string]domain.HTTPRequest{
		"file body": fileRequest(srv.URL, sel.Token),
		"multipart": formRequest(srv.URL, sel.Token),
	} {
		t.Run(name, func(t *testing.T) {
			runtime.GC()
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			if _, err := c.Do(context.Background(), "exec-"+name, req); err != nil {
				t.Fatalf("Do: %v", err)
			}
			runtime.ReadMemStats(&after)
			if got.Load() < size {
				t.Fatalf("server received %d bytes, want at least %d", got.Load(), size)
			}
			if alloc := after.TotalAlloc - before.TotalAlloc; alloc >= 16<<20 {
				t.Fatalf("allocated %d bytes to send a %d byte file", alloc, size)
			}
		})
	}
}

// ReadAt がパスを含む *os.PathError を返しても、Do のエラーにパスは出ず、
// ErrSelectedFileUnavailable として判定できる (HTTP/1.1 と HTTP/2 の両方)。
func TestNetClient_StreamedBodyHidesPathErrors(t *testing.T) {
	const secret = `C:\secret\data.bin`
	for name, h2 := range map[string]bool{"HTTP/1.1": false, "HTTP/2": true} {
		t.Run(name, func(t *testing.T) {
			srv, ch := recordingServer(t, h2)
			src := &fakeSource{
				data:    bytes.Repeat([]byte("x"), 100_000),
				modTime: time.Unix(100, 0),
				readErr: &os.PathError{Op: "read", Path: secret, Err: errors.New("device error")},
			}
			opener := &handleOpener{h: newSelectedFileHandle(src, src.info()), size: 100_000}
			req := fileRequest(srv.URL, "tok")
			req.Settings.InsecureSkipVerify = true

			_, err := NewNetClient(opener, t.TempDir()).Do(context.Background(), "exec-1", req)
			if !errors.Is(err, domain.ErrSelectedFileUnavailable) {
				t.Fatalf("Do: want ErrSelectedFileUnavailable, got %v", err)
			}
			if strings.Contains(err.Error(), secret) || strings.Contains(err.Error(), "device error") {
				t.Fatalf("error leaks the OS error: %q", err)
			}
			// サーバーに届いていれば、想定したプロトコルで送ったことを確かめる。
			select {
			case got := <-ch:
				if want := map[bool]int{false: 1, true: 2}[h2]; got.protoMajor != want {
					t.Fatalf("protocol HTTP/%d, want HTTP/%d", got.protoMajor, want)
				}
			case <-time.After(time.Second):
			}
		})
	}
}

// 送信中に変更を検出したら ErrSelectedFileChanged にし、サーバーは
// Content-Length に届かないボディを受け取る。
func TestNetClient_StreamedBodyStopsOnChange(t *testing.T) {
	srv, ch := recordingServer(t, false)
	data := bytes.Repeat([]byte("y"), 100_000)
	h := &memHandle{data: data, check: failFrom(1, domain.ErrSelectedFileChanged)}
	opener := &handleOpener{h: h, size: int64(len(data))}

	_, err := NewNetClient(opener, t.TempDir()).Do(context.Background(), "exec-1", fileRequest(srv.URL, "tok"))
	if !errors.Is(err, domain.ErrSelectedFileChanged) {
		t.Fatalf("Do: want ErrSelectedFileChanged, got %v", err)
	}
	got := receive(t, ch)
	if got.contentLength != int64(len(data)) || int64(len(got.body)) >= got.contentLength || got.err == nil {
		t.Fatalf("server received %d of %d bytes (err %v), want an incomplete body", len(got.body), got.contentLength, got.err)
	}
	waitClosed(t, h)
}

// 1 回目の送信と 307 の間でだけ変更されたら、送り直さずに ErrSelectedFileChanged にする。
func TestNetClient_StreamedBodyStopsRedirectOnChange(t *testing.T) {
	var finalHits atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/start", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		http.Redirect(w, r, "/final", http.StatusTemporaryRedirect)
	})
	mux.HandleFunc("/final", func(http.ResponseWriter, *http.Request) {
		finalHits.Add(1)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	// 1 回目は区間を読み切るときの確認が通り、GetBody での確認から失敗する。
	h := &memHandle{data: []byte("abc"), check: failFrom(2, domain.ErrSelectedFileChanged)}
	opener := &handleOpener{h: h, size: 3}

	_, err := NewNetClient(opener, t.TempDir()).Do(context.Background(), "exec-1", fileRequest(srv.URL+"/start", "tok"))
	if !errors.Is(err, domain.ErrSelectedFileChanged) {
		t.Fatalf("Do: want ErrSelectedFileChanged, got %v", err)
	}
	if n := finalHits.Load(); n != 0 {
		t.Fatalf("the redirect target received %d requests, want 0", n)
	}
	waitClosed(t, h)
}
