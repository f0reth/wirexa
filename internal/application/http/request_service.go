package httpapp

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

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

// maxExecutionIDLen は execution ID の最大長。マップのキーになるため長さを有界にする。
const maxExecutionIDLen = 64

// HTTPRequestService は HTTP リクエスト送信ユースケースを提供する。
// キャンセルと一時ファイルは送信ごとの execution ID で関連付ける。
// req.ID は保存済みリクエストの永続 ID で、実行制御には使わずログにだけ出す。
type HTTPRequestService struct {
	transport domain.HTTPTransport
	logger    cmn.Logger
	// root は実行ごとの context の親。Shutdown で cancel する。
	root context.Context
	stop context.CancelFunc
	// cancels は実行中リクエストのキャンセル関数。
	cancels map[string]context.CancelFunc
	wg      sync.WaitGroup
	mu      sync.Mutex
	closed  bool
}

// NewHTTPRequestService は HTTPRequestService を生成する。
// parent から cancel 可能なルート context を派生させ、実行ごとの context をその子にする。
func NewHTTPRequestService(parent context.Context, transport domain.HTTPTransport, logger cmn.Logger) *HTTPRequestService {
	root, stop := context.WithCancel(parent)
	return &HTTPRequestService{
		transport: transport,
		logger:    logger,
		root:      root,
		stop:      stop,
		cancels:   make(map[string]context.CancelFunc),
	}
}

// validExecutionID は execution ID が受け入れられる形式かを返す。
// RPC 由来の文字列が cancels・pending・一時ファイル追跡のキーになるため、
// 長さと文字種を有界にして意図しない値を入口で弾く。
func validExecutionID(id string) bool {
	if id == "" || len(id) > maxExecutionIDLen {
		return false
	}
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
		default:
			return false
		}
	}
	return true
}

// SendRequest は HTTP リクエストを実行してレスポンスを返す。
// ネットワーク障害・入力不正は error を返す。HTTP 4xx/5xx は正常レスポンスとして扱う。
// 同じ execution ID のリクエストが実行中なら拒否し、他のリクエストのキャンセル登録を上書きしない。
// 登録より前にこの execution ID へのキャンセルが届いていた場合は送信を始めない。
func (s *HTTPRequestService) SendRequest(executionID string, req domain.HTTPRequest) (domain.HTTPResponse, error) {
	if !validExecutionID(executionID) {
		return domain.HTTPResponse{}, &cmn.ValidationError{Field: "executionID", Message: executionID}
	}
	if !validMethods[req.Method] {
		return domain.HTTPResponse{}, &cmn.ValidationError{Field: "method", Message: req.Method}
	}
	// file ボディの送信元は File の token だけ。Contents に置かれたパスは受け付けない。
	req.Body.DropFileContents()
	ctx, cancel := context.WithCancel(s.root)
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		cancel()
		return domain.HTTPResponse{}, errShuttingDown
	}
	if _, dup := s.cancels[executionID]; dup {
		s.mu.Unlock()
		cancel()
		return domain.HTTPResponse{}, domain.ErrExecutionInProgress
	}
	s.cancels[executionID] = cancel
	s.wg.Add(1)
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.cancels, executionID)
		s.mu.Unlock()
		cancel()
		s.wg.Done()
	}()
	s.logger.Info("HTTP request sent", "source", "http", "request_id", req.ID, "execution_id", executionID, "method", req.Method, "url", req.URL, "body_bytes", len(req.Body.Contents[req.Body.Type]))
	resp, err := s.transport.Do(ctx, executionID, req)
	if err != nil {
		s.logger.Error("HTTP request failed", "source", "http", "request_id", req.ID, "execution_id", executionID, "method", req.Method, "url", req.URL, "error", err)
		return resp, fmt.Errorf("failed to send request: %w", err)
	}
	s.logger.Info("HTTP response received", "source", "http", "request_id", req.ID, "execution_id", executionID, "method", req.Method, "url", req.URL, "status", resp.StatusCode, "latency_ms", resp.TimingMs)
	return resp, nil
}

// CancelRequest は指定 execution ID の実行中 HTTP リクエストをキャンセルする。
func (s *HTTPRequestService) CancelRequest(executionID string) {
	if !validExecutionID(executionID) {
		return
	}
	s.mu.Lock()
	cancel, ok := s.cancels[executionID]
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
	s.mu.Unlock()
	// ルート context の cancel で実行中の全リクエストの context が終了する。
	s.stop()

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
