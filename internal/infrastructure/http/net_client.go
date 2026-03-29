package httpinfra

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	domain "github.com/f0reth/Wirexa/internal/domain/http"
)

const (
	defaultTimeout  = 30 * time.Second
	maxResponseBody = 10 * 1024 * 1024 // 10 MB
)

var _ domain.HttpTransport = (*NetClient)(nil)

// NetClient は net/http を使った domain.HttpTransport の実装。
type NetClient struct {
	client *http.Client
}

// NewNetClient は NetClient を生成する。
func NewNetClient() *NetClient {
	return &NetClient{
		client: &http.Client{Timeout: defaultTimeout},
	}
}

// Do は HttpRequest を実行して HttpResponse を返す。
func (c *NetClient) Do(req domain.HttpRequest) (domain.HttpResponse, error) {
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

	var bodyReader io.Reader
	contentType := ""
	switch req.Body.Type {
	case "json":
		bodyReader = strings.NewReader(req.Body.Content)
		contentType = "application/json"
	case "text":
		bodyReader = strings.NewReader(req.Body.Content)
		contentType = "text/plain"
	case "form-urlencoded":
		bodyReader = strings.NewReader(req.Body.Content)
		contentType = "application/x-www-form-urlencoded"
	case "form-data":
		bodyReader = strings.NewReader(req.Body.Content)
	}

	httpReq, err := http.NewRequestWithContext(context.Background(), req.Method, parsedURL.String(), bodyReader)
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

	start := time.Now()
	resp, err := c.client.Do(httpReq)
	elapsed := time.Since(start).Milliseconds()

	if err != nil {
		return domain.HttpResponse{}, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody+1))
	if err != nil {
		return domain.HttpResponse{}, fmt.Errorf("failed to read response: %w", err)
	}

	truncated := false
	if int64(len(body)) > maxResponseBody {
		body = body[:maxResponseBody]
		truncated = true
	}

	headers := make(map[string]string, len(resp.Header))
	for k, v := range resp.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	result := domain.HttpResponse{
		StatusCode:  resp.StatusCode,
		StatusText:  resp.Status,
		Headers:     headers,
		Body:        string(body),
		ContentType: resp.Header.Get("Content-Type"),
		Size:        int64(len(body)),
		TimingMs:    elapsed,
	}
	if truncated {
		result.Error = fmt.Sprintf("response body truncated at %d bytes", maxResponseBody)
	}
	return result, nil
}
