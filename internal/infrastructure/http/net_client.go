// Package httpinfra は HTTP インフラストラクチャ層を提供する。
package httpinfra

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	domain "github.com/f0reth/Wirexa/internal/domain/http"
)

const (
	defaultTimeoutSec    = 30
	defaultMaxResponseMB = 10
	// maxCachedTransports は transports の上限。ProxyURL はユーザー入力のため
	// キーが際限なく増えうるので、上限に達したらまとめて破棄して作り直す。
	maxCachedTransports = 16
)

var _ domain.HttpTransport = (*NetClient)(nil)

// transportKey は Transport の挙動を決める設定のみを抜き出したキャッシュキー。
// Timeout / CheckRedirect は http.Client 側の責務なので含めない。
type transportKey struct {
	proxyMode          string
	proxyURL           string
	insecureSkipVerify bool
}

// NetClient は net/http を使った domain.HttpTransport の実装。
type NetClient struct {
	transports map[transportKey]*http.Transport
	tempFiles  sync.Map // requestID → tempFilePath (string)
	mu         sync.Mutex
}

// NewNetClient は NetClient を生成する。
func NewNetClient() *NetClient {
	return &NetClient{transports: make(map[transportKey]*http.Transport)}
}

// ConsumeTempFilePath は指定リクエストIDのテンポラリファイルパスを返し、マップから削除する。
// 上限超過がなかった場合は空文字列を返す。
func (c *NetClient) ConsumeTempFilePath(requestID string) string {
	if v, ok := c.tempFiles.LoadAndDelete(requestID); ok {
		s, ok := v.(string)
		if !ok {
			return ""
		}
		return s
	}
	return ""
}

// Cleanup は tempFiles に残る打ち切りレスポンスの一時ファイルを全削除し、
// キャッシュした Transport のアイドルコネクションを閉じる。
// フロントへ未受け渡し (ConsumeTempFilePath されていない) のまま終了したものを回収する。
func (c *NetClient) Cleanup() {
	c.tempFiles.Range(func(k, v any) bool {
		if s, ok := v.(string); ok {
			_ = os.Remove(s) //nolint:errcheck // best-effort cleanup
		}
		c.tempFiles.Delete(k)
		return true
	})

	c.mu.Lock()
	defer c.mu.Unlock()
	c.closeTransportsLocked()
}

// closeTransportsLocked はキャッシュ済み Transport を全て破棄する。呼び出し元は c.mu を保持していること。
func (c *NetClient) closeTransportsLocked() {
	for k, t := range c.transports {
		t.CloseIdleConnections()
		delete(c.transports, k)
	}
}

// SweepStaleTempFiles は前回セッションで残った wirexa-response-* を削除する。
// 単一インスタンスロックにより同時起動が無いため、全一致を安全に削除できる。
// フロントへ渡されたまま保存されなかったファイルは tempFiles で追跡できないため、
// 次回起動時のこの掃除が唯一の回収経路になる。
func SweepStaleTempFiles() {
	matches, err := filepath.Glob(filepath.Join(os.TempDir(), "wirexa-response-*"))
	if err != nil {
		return
	}
	for _, p := range matches {
		_ = os.Remove(p) //nolint:errcheck // best-effort cleanup
	}
}

// Do は HttpRequest を実行して HttpResponse を返す。
func (c *NetClient) Do(ctx context.Context, req domain.HttpRequest) (domain.HttpResponse, error) {
	timeout := resolveTimeout(req.Settings)
	// タイムアウトは http.Client.Timeout ではなく context で表現し、
	// RequestUseCase 側のキャンセルと同じ経路に一本化する。
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	parsedURL, err := url.Parse(req.URL)
	if err != nil {
		return domain.HttpResponse{}, fmt.Errorf("invalid URL: %w", err)
	}

	q := parsedURL.Query()
	for _, p := range req.Params {
		if p.Enabled && p.Key != "" {
			q.Add(p.Key, p.Value)
		}
	}
	parsedURL.RawQuery = q.Encode()

	bodyContent := req.Body.Contents[req.Body.Type]
	var bodyReader io.Reader
	contentType := ""
	switch req.Body.Type {
	case "json":
		bodyReader = strings.NewReader(bodyContent)
		contentType = "application/json"
	case "text":
		bodyReader = strings.NewReader(bodyContent)
		contentType = "text/plain"
	case "form-urlencoded":
		bodyReader = strings.NewReader(bodyContent)
		contentType = "application/x-www-form-urlencoded"
	case "form-data":
		bodyReader = strings.NewReader(bodyContent)
	case "file":
		if bodyContent != "" {
			var fileData []byte
			fileData, err = os.ReadFile(bodyContent) //nolint:gosec // user-selected file path
			if err != nil {
				return domain.HttpResponse{}, fmt.Errorf("failed to read file: %w", err)
			}
			bodyReader = bytes.NewReader(fileData)
			if ct := mime.TypeByExtension(filepath.Ext(bodyContent)); ct != "" {
				contentType = ct
			} else {
				contentType = "application/octet-stream"
			}
		}
	}

	httpReq, err := http.NewRequestWithContext(ctx, req.Method, parsedURL.String(), bodyReader)
	if err != nil {
		return domain.HttpResponse{}, fmt.Errorf("failed to create request: %w", err)
	}

	for _, h := range req.Headers {
		if h.Enabled && h.Key != "" {
			httpReq.Header.Set(h.Key, h.Value)
		}
	}
	switch req.Auth.Type {
	case "basic":
		httpReq.SetBasicAuth(req.Auth.Username, req.Auth.Password)
	case "bearer":
		httpReq.Header.Set("Authorization", "Bearer "+req.Auth.Token)
	}
	if contentType != "" && httpReq.Header.Get("Content-Type") == "" {
		httpReq.Header.Set("Content-Type", contentType)
	}

	client := c.buildHTTPClient(req.Settings)
	maxBody := resolveMaxResponseBody(req.Settings)

	start := time.Now()
	resp, err := client.Do(httpReq)
	elapsed := time.Since(start).Milliseconds()

	if err != nil {
		// context 由来のタイムアウトは "context deadline exceeded" としか出ないため、
		// ユーザーに意味の伝わる文言へ置き換える。
		if errors.Is(err, context.DeadlineExceeded) {
			return domain.HttpResponse{}, fmt.Errorf("request timed out after %s", timeout)
		}
		return domain.HttpResponse{}, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }() //nolint:errcheck // best-effort cleanup

	// ボディをテンポラリファイルへストリーミング（メモリを圧迫しない）
	tmpFile, err := os.CreateTemp("", "wirexa-response-*")
	if err != nil {
		return domain.HttpResponse{}, fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpName := tmpFile.Name()

	if _, err = io.Copy(tmpFile, resp.Body); err != nil {
		_ = tmpFile.Close()    //nolint:errcheck // best-effort cleanup
		_ = os.Remove(tmpName) //nolint:errcheck // best-effort cleanup
		if errors.Is(err, context.DeadlineExceeded) {
			return domain.HttpResponse{}, fmt.Errorf("request timed out after %s", timeout)
		}
		return domain.HttpResponse{}, fmt.Errorf("failed to read response: %w", err)
	}
	_ = tmpFile.Close() //nolint:errcheck // best-effort cleanup

	info, err := os.Stat(tmpName)
	if err != nil {
		_ = os.Remove(tmpName) //nolint:errcheck // best-effort cleanup
		return domain.HttpResponse{}, fmt.Errorf("failed to stat temp file: %w", err)
	}

	var body []byte
	bodyTruncated := false
	if info.Size() <= maxBody {
		// 上限以内: メモリへ読み込み、テンポラリファイルを削除
		body, err = os.ReadFile(tmpName) //nolint:gosec // path is app-generated temp file
		_ = os.Remove(tmpName)           //nolint:errcheck // best-effort cleanup
		if err != nil {
			return domain.HttpResponse{}, fmt.Errorf("failed to read temp file: %w", err)
		}
	} else {
		// 上限超過: 先頭 maxBody バイトのみ読み込み、テンポラリファイルは保持
		f, err := os.Open(tmpName) //nolint:gosec // path is app-generated temp file
		if err != nil {
			_ = os.Remove(tmpName) //nolint:errcheck // best-effort cleanup
			return domain.HttpResponse{}, fmt.Errorf("failed to open temp file: %w", err)
		}
		body = make([]byte, maxBody)
		if _, err = io.ReadFull(f, body); err != nil {
			_ = f.Close()          //nolint:errcheck // best-effort cleanup
			_ = os.Remove(tmpName) //nolint:errcheck // best-effort cleanup
			return domain.HttpResponse{}, fmt.Errorf("failed to read temp file: %w", err)
		}
		_ = f.Close() //nolint:errcheck // best-effort cleanup
		bodyTruncated = true
		// 同一 req.ID の再送で旧パスを上書きするとオーファン化するため、先に回収削除する。
		if old, ok := c.tempFiles.LoadAndDelete(req.ID); ok {
			if s, ok := old.(string); ok {
				_ = os.Remove(s) //nolint:errcheck // best-effort cleanup
			}
		}
		c.tempFiles.Store(req.ID, tmpName)
	}

	headers := make(map[string]string, len(resp.Header))
	for k, v := range resp.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	respContentType := resp.Header.Get("Content-Type")
	// 非 UTF-8 のバイナリボディは string 変換で壊れるため base64 で渡す。
	bodyStr := string(body)
	bodyBase64 := false
	if !utf8.Valid(body) {
		bodyStr = base64.StdEncoding.EncodeToString(body)
		bodyBase64 = true
	}

	return domain.HttpResponse{
		StatusCode:    resp.StatusCode,
		StatusText:    resp.Status,
		Headers:       headers,
		Body:          bodyStr,
		ContentType:   respContentType,
		Size:          info.Size(),
		TimingMs:      elapsed,
		BodyTruncated: bodyTruncated,
		BodyBase64:    bodyBase64,
	}, nil
}

// buildHTTPClient はリクエスト設定に対応する http.Client を組み立てる。
// Transport はキャッシュから再利用し、http.Client 自体は軽量なので毎回生成する。
func (c *NetClient) buildHTTPClient(s domain.RequestSettings) *http.Client {
	client := &http.Client{Transport: c.transportFor(s)}
	if s.DisableRedirects {
		client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}
	return client
}

// transportFor は設定に対応する Transport をキャッシュから返す (無ければ生成する)。
// リクエストごとに Transport を作ると keep-alive が効かず毎回 TCP + TLS ハンドシェイクが走るため、
// Transport の挙動を決める設定が同じリクエスト間ではコネクションプールを共有する。
func (c *NetClient) transportFor(s domain.RequestSettings) *http.Transport {
	key := transportKey{
		proxyMode:          s.ProxyMode,
		proxyURL:           s.ProxyURL,
		insecureSkipVerify: s.InsecureSkipVerify,
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if t, ok := c.transports[key]; ok {
		return t
	}

	t := newTransport(s)
	if len(c.transports) >= maxCachedTransports {
		c.closeTransportsLocked()
	}
	c.transports[key] = t
	return t
}

// newTransport は標準の DefaultTransport (コネクションプール設定 / HTTP2) を土台に Transport を生成する。
func newTransport(s domain.RequestSettings) *http.Transport {
	var t *http.Transport
	if dt, ok := http.DefaultTransport.(*http.Transport); ok {
		t = dt.Clone()
	} else {
		t = &http.Transport{}
	}
	t.Proxy = resolveProxy(s)
	if s.InsecureSkipVerify {
		t.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // ユーザーが明示的に設定した場合のみ有効
	}
	return t
}

func resolveTimeout(s domain.RequestSettings) time.Duration {
	if s.TimeoutSec > 0 {
		return time.Duration(s.TimeoutSec) * time.Second
	}
	return time.Duration(defaultTimeoutSec) * time.Second
}

func resolveProxy(s domain.RequestSettings) func(*http.Request) (*url.URL, error) {
	switch s.ProxyMode {
	case "none":
		return nil
	case "custom":
		if s.ProxyURL == "" {
			return nil
		}
		proxyURL, err := url.Parse(s.ProxyURL)
		if err != nil {
			return nil
		}
		return http.ProxyURL(proxyURL)
	default: // "" | "system"
		return http.ProxyFromEnvironment
	}
}

func resolveMaxResponseBody(s domain.RequestSettings) int64 {
	if s.MaxResponseBodyMB > 0 {
		return int64(s.MaxResponseBodyMB) * 1024 * 1024
	}
	return int64(defaultMaxResponseMB) * 1024 * 1024
}
