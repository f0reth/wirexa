package httpapp

import (
	"context"
	"sync"

	cmn "github.com/f0reth/Wirexa/internal/domain"
	domain "github.com/f0reth/Wirexa/internal/domain/http"
)

var _ domain.RequestUseCase = (*HttpRequestService)(nil)

var validMethods = map[string]bool{
	"GET": true, "POST": true, "PUT": true, "DELETE": true,
	"PATCH": true, "HEAD": true, "OPTIONS": true,
}

// HttpRequestService は HTTP リクエスト送信ユースケースを提供する。
type HttpRequestService struct {
	transport domain.HttpTransport
	logger    cmn.Logger
	mu        sync.Mutex
	cancels   map[string]context.CancelFunc
}

// NewHTTPRequestService は HttpRequestService を生成する。
func NewHTTPRequestService(transport domain.HttpTransport, logger cmn.Logger) *HttpRequestService {
	return &HttpRequestService{transport: transport, logger: logger, cancels: make(map[string]context.CancelFunc)}
}

// SendRequest は HTTP リクエストを実行してレスポンスを返す。
// ネットワーク障害・入力不正は error を返す。HTTP 4xx/5xx は正常レスポンスとして扱う。
func (s *HttpRequestService) SendRequest(req domain.HttpRequest) (domain.HttpResponse, error) {
	if !validMethods[req.Method] {
		return domain.HttpResponse{}, &cmn.ValidationError{Field: "method", Message: req.Method}
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
		return resp, err
	}
	s.logger.Info("HTTP response received", "source", "http", "method", req.Method, "url", req.URL, "status", resp.StatusCode, "latency_ms", resp.TimingMs)
	return resp, nil
}

// CancelRequest は指定 ID の実行中 HTTP リクエストをキャンセルする。
func (s *HttpRequestService) CancelRequest(id string) {
	s.mu.Lock()
	cancel, ok := s.cancels[id]
	s.mu.Unlock()
	if ok {
		cancel()
	}
}
