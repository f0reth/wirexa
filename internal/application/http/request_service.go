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
	mu        sync.Mutex
	cancel    context.CancelFunc
}

// NewHTTPRequestService は HttpRequestService を生成する。
func NewHTTPRequestService(transport domain.HttpTransport) *HttpRequestService {
	return &HttpRequestService{transport: transport}
}

// SendRequest は HTTP リクエストを実行してレスポンスを返す。
// ネットワーク障害・入力不正は error を返す。HTTP 4xx/5xx は正常レスポンスとして扱う。
func (s *HttpRequestService) SendRequest(req domain.HttpRequest) (domain.HttpResponse, error) {
	if !validMethods[req.Method] {
		return domain.HttpResponse{}, &cmn.ValidationError{Field: "method", Message: req.Method}
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.cancel = cancel
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.cancel = nil
		s.mu.Unlock()
		cancel()
	}()
	return s.transport.Do(ctx, req)
}

// CancelRequest は実行中の HTTP リクエストをキャンセルする。
func (s *HttpRequestService) CancelRequest() {
	s.mu.Lock()
	cancel := s.cancel
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}
