package httpapp

import (
	"context"
	"fmt"
	"sync"

	cmn "github.com/f0reth/Wirexa/internal/domain"
	domain "github.com/f0reth/Wirexa/internal/domain/http"
)

var _ domain.RequestUseCase = (*HTTPRequestService)(nil)

var validMethods = map[string]bool{
	"GET": true, "POST": true, "PUT": true, "DELETE": true,
	"PATCH": true, "HEAD": true, "OPTIONS": true,
}

// HTTPRequestService は HTTP リクエスト送信ユースケースを提供する。
type HTTPRequestService struct {
	transport domain.HTTPTransport
	logger    cmn.Logger
	cancels   map[string]context.CancelFunc
	mu        sync.Mutex
}

// NewHTTPRequestService は HTTPRequestService を生成する。
func NewHTTPRequestService(transport domain.HTTPTransport, logger cmn.Logger) *HTTPRequestService {
	return &HTTPRequestService{transport: transport, logger: logger, cancels: make(map[string]context.CancelFunc)}
}

// SendRequest は HTTP リクエストを実行してレスポンスを返す。
// ネットワーク障害・入力不正は error を返す。HTTP 4xx/5xx は正常レスポンスとして扱う。
func (s *HTTPRequestService) SendRequest(req domain.HTTPRequest) (domain.HTTPResponse, error) {
	if !validMethods[req.Method] {
		return domain.HTTPResponse{}, &cmn.ValidationError{Field: "method", Message: req.Method}
	}
	s.logger.Info("HTTP request sent", "source", "http", "method", req.Method, "url", req.URL, "body_bytes", len(req.Body.Contents[req.Body.Type]))
	ctx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.cancels[req.ID] = cancel
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.cancels, req.ID)
		s.mu.Unlock()
		cancel()
	}()
	resp, err := s.transport.Do(ctx, req)
	if err != nil {
		s.logger.Error("HTTP request failed", "source", "http", "method", req.Method, "url", req.URL, "error", err)
		return resp, fmt.Errorf("failed to send request: %w", err)
	}
	s.logger.Info("HTTP response received", "source", "http", "method", req.Method, "url", req.URL, "status", resp.StatusCode, "latency_ms", resp.TimingMs)
	return resp, nil
}

// CancelRequest は指定 ID の実行中 HTTP リクエストをキャンセルする。
func (s *HTTPRequestService) CancelRequest(id string) {
	s.mu.Lock()
	cancel, ok := s.cancels[id]
	s.mu.Unlock()
	if ok {
		cancel()
	}
}
