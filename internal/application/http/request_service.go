package httpapp

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

var _ domain.RequestUseCase = (*HttpRequestService)(nil)

const (
	requestTimeout  = 30 * time.Second
	maxResponseBody = 10 * 1024 * 1024 // 10 MB
)

var validMethods = map[string]bool{
	"GET": true, "POST": true, "PUT": true, "DELETE": true,
	"PATCH": true, "HEAD": true, "OPTIONS": true,
}

// HttpRequestService は HTTP リクエスト送信ユースケースを提供する。
type HttpRequestService struct {
	client *http.Client
}

// NewHTTPRequestService は HttpRequestService を生成する。
func NewHTTPRequestService() *HttpRequestService {
	return &HttpRequestService{
		client: &http.Client{Timeout: requestTimeout},
	}
}

// SendRequest は HTTP リクエストを実行してレスポンスを返す。
// ネットワーク障害・入力不正は error を返す。HTTP 4xx/5xx は正常レスポンスとして扱う。
func (s *HttpRequestService) SendRequest(req domain.HttpRequest) (domain.HttpResponse, error) {
	if !validMethods[req.Method] {
		return domain.HttpResponse{}, &domain.ValidationError{Field: "method", Message: req.Method}
	}

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
	if contentType != "" && httpReq.Header.Get("Content-Type") == "" {
		httpReq.Header.Set("Content-Type", contentType)
	}

	start := time.Now()
	resp, err := s.client.Do(httpReq)
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
