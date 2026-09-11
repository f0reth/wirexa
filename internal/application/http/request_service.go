package httpapp

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	cmn "github.com/f0reth/Wirexa/internal/domain"
	domain "github.com/f0reth/Wirexa/internal/domain/http"
)

var _ domain.RequestUseCase = (*HTTPRequestService)(nil)

var validMethods = map[string]bool{
	"GET": true, "POST": true, "PUT": true, "DELETE": true,
	"PATCH": true, "HEAD": true, "OPTIONS": true,
}

// errShuttingDown は終了処理の開始後に送信しようとした場合に返す。
var errShuttingDown = errors.New("application is shutting down")

// HTTPRequestService は HTTP リクエスト送信ユースケースを提供する。
// req.ID は送信ごとの execution ID として扱い、キャンセルと一時ファイルをこの ID で関連付ける。
type HTTPRequestService struct {
	transport domain.HTTPTransport
	logger    cmn.Logger
	cancels   map[string]context.CancelFunc
	wg        sync.WaitGroup
	mu        sync.Mutex
	closed    bool
}

// NewHTTPRequestService は HTTPRequestService を生成する。
func NewHTTPRequestService(transport domain.HTTPTransport, logger cmn.Logger) *HTTPRequestService {
	return &HTTPRequestService{transport: transport, logger: logger, cancels: make(map[string]context.CancelFunc)}
}

// SendRequest は HTTP リクエストを実行してレスポンスを返す。
// ネットワーク障害・入力不正は error を返す。HTTP 4xx/5xx は正常レスポンスとして扱う。
// 同じ execution ID のリクエストが実行中なら拒否し、他のリクエストのキャンセル登録を上書きしない。
// ID が空の場合は内部で採番する (frontend からは保存・破棄できないが、上限と TTL で回収される)。
func (s *HTTPRequestService) SendRequest(req domain.HTTPRequest) (domain.HTTPResponse, error) { //nolint:gocritic // hugeParam: preserve request DTO value semantics across application boundaries.
	if !validMethods[req.Method] {
		return domain.HTTPResponse{}, &cmn.ValidationError{Field: "method", Message: req.Method}
	}
	if req.ID == "" {
		req.ID = uuid.NewString()
	}
	// file ボディの送信元は File の token だけ。Contents に置かれたパスは受け付けない。
	req.Body.DropFileContents()
	ctx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		cancel()
		return domain.HTTPResponse{}, errShuttingDown
	}
	if _, dup := s.cancels[req.ID]; dup {
		s.mu.Unlock()
		cancel()
		return domain.HTTPResponse{}, domain.ErrExecutionInProgress
	}
	s.cancels[req.ID] = cancel
	s.wg.Add(1)
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.cancels, req.ID)
		s.mu.Unlock()
		cancel()
		s.wg.Done()
	}()
	s.logger.Info("HTTP request sent", "source", "http", "method", req.Method, "url", req.URL, "body_bytes", len(req.Body.Contents[req.Body.Type]))
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

// Shutdown は新規送信を止めて実行中の全リクエストをキャンセルし、timeout まで終了を待つ。
// 全て終了したら true を返す。終了しなかったリクエストの一時ファイルは
// 次回起動時の sweep に任せる。
func (s *HTTPRequestService) Shutdown(timeout time.Duration) bool {
	s.mu.Lock()
	s.closed = true
	for _, cancel := range s.cancels {
		cancel()
	}
	s.mu.Unlock()

	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return true
	case <-time.After(timeout):
		return false
	}
}
