package httpinfra

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
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

	domain "github.com/f0reth/Wirexa/internal/domain/http"
)

const (
	defaultTimeoutSec    = 30
	defaultMaxResponseMB = 10
)

var _ domain.HttpTransport = (*NetClient)(nil)

// NetClient は net/http を使った domain.HttpTransport の実装。
type NetClient struct {
	tempFiles sync.Map // requestID → tempFilePath (string)
}

// NewNetClient は NetClient を生成する。
func NewNetClient() *NetClient {
	return &NetClient{}
}

// ConsumeTempFilePath は指定リクエストIDのテンポラリファイルパスを返し、マップから削除する。
// 上限超過がなかった場合は空文字列を返す。
func (c *NetClient) ConsumeTempFilePath(requestID string) string {
	if v, ok := c.tempFiles.LoadAndDelete(requestID); ok {
		s, _ := v.(string)
		return s
	}
	return ""
}

// Do は HttpRequest を実行して HttpResponse を返す。
func (c *NetClient) Do(ctx context.Context, req domain.HttpRequest) (domain.HttpResponse, error) {
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
			fileData, err = os.ReadFile(bodyContent)
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

	client := buildHTTPClient(req.Settings)
	maxBody := resolveMaxResponseBody(req.Settings)

	start := time.Now()
	resp, err := client.Do(httpReq)
	elapsed := time.Since(start).Milliseconds()

	if err != nil {
		return domain.HttpResponse{}, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// ボディをテンポラリファイルへストリーミング（メモリを圧迫しない）
	tmpFile, err := os.CreateTemp("", "wirexa-response-*")
	if err != nil {
		return domain.HttpResponse{}, fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpName := tmpFile.Name()

	if _, err = io.Copy(tmpFile, resp.Body); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpName)
		return domain.HttpResponse{}, fmt.Errorf("failed to read response: %w", err)
	}
	_ = tmpFile.Close()

	info, err := os.Stat(tmpName)
	if err != nil {
		_ = os.Remove(tmpName)
		return domain.HttpResponse{}, fmt.Errorf("failed to stat temp file: %w", err)
	}

	var body []byte
	bodyTruncated := false
	if info.Size() <= maxBody {
		// 上限以内: メモリへ読み込み、テンポラリファイルを削除
		body, err = os.ReadFile(tmpName)
		_ = os.Remove(tmpName)
		if err != nil {
			return domain.HttpResponse{}, fmt.Errorf("failed to read temp file: %w", err)
		}
	} else {
		// 上限超過: 先頭 maxBody バイトのみ読み込み、テンポラリファイルは保持
		f, err := os.Open(tmpName)
		if err != nil {
			_ = os.Remove(tmpName)
			return domain.HttpResponse{}, fmt.Errorf("failed to open temp file: %w", err)
		}
		body = make([]byte, maxBody)
		if _, err = io.ReadFull(f, body); err != nil {
			_ = f.Close()
			_ = os.Remove(tmpName)
			return domain.HttpResponse{}, fmt.Errorf("failed to read temp file: %w", err)
		}
		_ = f.Close()
		bodyTruncated = true
		c.tempFiles.Store(req.ID, tmpName)
	}

	headers := make(map[string]string, len(resp.Header))
	for k, v := range resp.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	respContentType := resp.Header.Get("Content-Type")
	bodyStr := string(body)
	if strings.HasPrefix(strings.ToLower(respContentType), "image/") {
		bodyStr = base64.StdEncoding.EncodeToString(body)
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
	}, nil
}

func buildHTTPClient(s domain.RequestSettings) *http.Client {
	timeout := time.Duration(defaultTimeoutSec) * time.Second
	if s.TimeoutSec > 0 {
		timeout = time.Duration(s.TimeoutSec) * time.Second
	}

	var tlsConfig *tls.Config
	if s.InsecureSkipVerify {
		tlsConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // ユーザーが明示的に設定した場合のみ有効
	}

	transport := &http.Transport{
		Proxy:           resolveProxy(s),
		TLSClientConfig: tlsConfig,
	}

	client := &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}

	if s.DisableRedirects {
		client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}

	return client
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
