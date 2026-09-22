package httpapp

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	cmn "github.com/f0reth/Wirexa/internal/domain"
	domain "github.com/f0reth/Wirexa/internal/domain/http"
	"github.com/f0reth/Wirexa/internal/testutil"
)

// mockTransport は HTTPTransport のインメモリモック。渡された execution ID を記録する。
type mockTransport struct {
	doFn    func(req domain.HTTPRequest) (domain.HTTPResponse, error)
	mu      sync.Mutex
	execIDs []string
}

func (m *mockTransport) Do(_ context.Context, executionID string, req domain.HTTPRequest) (domain.HTTPResponse, error) {
	m.mu.Lock()
	m.execIDs = append(m.execIDs, executionID)
	m.mu.Unlock()
	if m.doFn != nil {
		return m.doFn(req)
	}
	return domain.HTTPResponse{StatusCode: 200}, nil
}

// executions は Do に渡された execution ID を呼ばれた順に返す。
func (m *mockTransport) executions() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return slices.Clone(m.execIDs)
}

func TestHTTPRequestService_SendRequest_ValidMethods(t *testing.T) {
	validMethods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}
	transport := &mockTransport{}
	svc := NewHTTPRequestService(context.Background(), transport, testutil.NoopLogger{})

	for _, method := range validMethods {
		t.Run(method, func(t *testing.T) {
			resp, err := svc.SendRequest("exec-"+method, domain.HTTPRequest{Method: method, URL: "http://example.com"})
			if err != nil {
				t.Fatalf("SendRequest(%q): unexpected error %v", method, err)
			}
			if resp.StatusCode != http.StatusOK {
				t.Errorf("expected 200, got %d", resp.StatusCode)
			}
		})
	}
}

func TestHTTPRequestService_SendRequest_InvalidMethods(t *testing.T) {
	invalidMethods := []string{"get", "post", "TRACE", "CONNECT", "", "INVALID", "get "}
	transport := &mockTransport{}
	svc := NewHTTPRequestService(context.Background(), transport, testutil.NoopLogger{})

	for _, method := range invalidMethods {
		t.Run("invalid_"+method, func(t *testing.T) {
			_, err := svc.SendRequest("exec-1", domain.HTTPRequest{Method: method, URL: "http://example.com"})
			if err == nil {
				t.Fatalf("SendRequest(%q): expected error, got nil", method)
			}
		})
	}
}

func TestHTTPRequestService_SendRequest_TransportError(t *testing.T) {
	wantErr := errors.New("network unreachable")
	transport := &mockTransport{
		doFn: func(_ domain.HTTPRequest) (domain.HTTPResponse, error) {
			return domain.HTTPResponse{}, wantErr
		},
	}
	svc := NewHTTPRequestService(context.Background(), transport, testutil.NoopLogger{})

	_, err := svc.SendRequest("exec-1", domain.HTTPRequest{Method: "GET", URL: "http://example.com"})
	if !errors.Is(err, wantErr) {
		t.Errorf("expected network error, got %v", err)
	}
}

func TestHTTPRequestService_SendRequest_TransportResponse(t *testing.T) {
	wantResp := domain.HTTPResponse{
		StatusCode:  404,
		StatusText:  "Not Found",
		Body:        `{"error":"not found"}`,
		ContentType: "application/json",
		TimingMs:    42,
	}
	transport := &mockTransport{
		doFn: func(_ domain.HTTPRequest) (domain.HTTPResponse, error) {
			return wantResp, nil
		},
	}
	svc := NewHTTPRequestService(context.Background(), transport, testutil.NoopLogger{})

	got, err := svc.SendRequest("exec-1", domain.HTTPRequest{Method: "GET", URL: "http://example.com"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.StatusCode != wantResp.StatusCode {
		t.Errorf("StatusCode = %d, want %d", got.StatusCode, wantResp.StatusCode)
	}
	if got.Body != wantResp.Body {
		t.Errorf("Body = %q, want %q", got.Body, wantResp.Body)
	}
}

func TestHTTPRequestService_SendRequest_PassesRequestToTransport(t *testing.T) {
	var capturedReq domain.HTTPRequest
	transport := &mockTransport{
		doFn: func(r domain.HTTPRequest) (domain.HTTPResponse, error) {
			capturedReq = r
			return domain.HTTPResponse{StatusCode: 200}, nil
		},
	}
	svc := NewHTTPRequestService(context.Background(), transport, testutil.NoopLogger{})

	input := domain.HTTPRequest{
		ID:     "saved-1",
		Method: "POST",
		URL:    "http://example.com/api",
		Headers: []domain.KeyValuePair{
			{Key: "Content-Type", Value: "application/json", Enabled: true},
		},
		Body: domain.RequestBody{Type: "json", Contents: map[string]string{"json": `{"x":1}`}},
		Auth: domain.RequestAuth{Type: "bearer", Token: "tok123"},
	}
	svc.SendRequest("exec-1", input)

	if capturedReq.URL != input.URL {
		t.Errorf("URL = %q, want %q", capturedReq.URL, input.URL)
	}
	if capturedReq.Auth.Token != "tok123" {
		t.Errorf("Auth.Token = %q, want %q", capturedReq.Auth.Token, "tok123")
	}
	// 永続 ID は execution ID に差し替えられず、そのまま transport へ渡る。
	if capturedReq.ID != "saved-1" {
		t.Errorf("req.ID = %q, want %q", capturedReq.ID, "saved-1")
	}
	if got := transport.executions(); !slices.Equal(got, []string{"exec-1"}) {
		t.Errorf("execution IDs = %v, want [exec-1]", got)
	}
}

// execution ID が形式を満たさない場合は ValidationError を返し、transport を呼ばない。
func TestHTTPRequestService_SendRequest_InvalidExecutionID(t *testing.T) {
	cases := map[string]string{
		"empty":        "",
		"too_long":     strings.Repeat("a", maxExecutionIDLen+1),
		"invalid_char": "exec/1",
		"space":        "exec 1",
	}
	for name, execID := range cases {
		t.Run(name, func(t *testing.T) {
			transport := &mockTransport{}
			svc := NewHTTPRequestService(context.Background(), transport, testutil.NoopLogger{})
			_, err := svc.SendRequest(execID, domain.HTTPRequest{Method: "GET", URL: "http://example.com"})
			if _, ok := errors.AsType[*cmn.ValidationError](err); !ok {
				t.Fatalf("SendRequest(%q): want ValidationError, got %v", execID, err)
			}
			if got := transport.executions(); len(got) != 0 {
				t.Errorf("transport must not be called, got %v", got)
			}
		})
	}
}

// mockTransportCtx はコンテキストと execution ID を受け取る doFn を持つ transport モック。
type mockTransportCtx struct {
	doFn func(ctx context.Context, executionID string, req domain.HTTPRequest) (domain.HTTPResponse, error)
}

func (m *mockTransportCtx) Do(ctx context.Context, executionID string, req domain.HTTPRequest) (domain.HTTPResponse, error) {
	if m.doFn != nil {
		return m.doFn(ctx, executionID, req)
	}
	return domain.HTTPResponse{StatusCode: 200}, nil
}

func TestHTTPRequestService_CancelRequest_NonExistentID(_ *testing.T) {
	svc := NewHTTPRequestService(context.Background(), &mockTransport{}, testutil.NoopLogger{})
	svc.CancelRequest("no-such-id")
}

func TestHTTPRequestService_CancelRequest_ValidID(t *testing.T) {
	started := make(chan struct{})
	transport := &mockTransportCtx{
		doFn: func(ctx context.Context, _ string, _ domain.HTTPRequest) (domain.HTTPResponse, error) {
			close(started)
			<-ctx.Done()
			return domain.HTTPResponse{}, ctx.Err()
		},
	}
	svc := NewHTTPRequestService(context.Background(), transport, testutil.NoopLogger{})

	const execID = "exec-cancel-test"
	done := make(chan error, 1)
	go func() {
		_, err := svc.SendRequest(execID, domain.HTTPRequest{ID: "saved-1", Method: "GET", URL: "http://example.com"})
		done <- err
	}()

	<-started
	svc.CancelRequest(execID)
	if err := <-done; err == nil {
		t.Error("expected context cancellation error, got nil")
	}
}

func TestHTTPRequestService_SendRequest_InvalidMethod_ReturnsValidationError(t *testing.T) {
	svc := NewHTTPRequestService(context.Background(), &mockTransport{}, testutil.NoopLogger{})
	_, err := svc.SendRequest("exec-1", domain.HTTPRequest{Method: "INVALID", URL: "http://example.com"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if _, ok := errors.AsType[*cmn.ValidationError](err); !ok {
		t.Errorf("expected ValidationError, got %T", err)
	}
}

// 同じ execution ID の並行送信は拒否し、先行リクエストのキャンセル登録を上書きしない。
func TestHTTPRequestService_SendRequest_RejectsDuplicateExecutionID(t *testing.T) {
	started := make(chan struct{})
	transport := &mockTransportCtx{
		doFn: func(ctx context.Context, _ string, _ domain.HTTPRequest) (domain.HTTPResponse, error) {
			close(started)
			<-ctx.Done()
			return domain.HTTPResponse{}, ctx.Err()
		},
	}
	svc := NewHTTPRequestService(context.Background(), transport, testutil.NoopLogger{})

	const execID = "exec-dup"
	done := make(chan error, 1)
	go func() {
		_, err := svc.SendRequest(execID, domain.HTTPRequest{Method: "GET", URL: "http://example.com"})
		done <- err
	}()
	<-started

	if _, err := svc.SendRequest(execID, domain.HTTPRequest{Method: "GET", URL: "http://example.com"}); !errors.Is(err, domain.ErrExecutionInProgress) {
		t.Fatalf("duplicate send: want ErrExecutionInProgress, got %v", err)
	}

	// 先行リクエストは引き続きキャンセルできる。
	svc.CancelRequest(execID)
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected the first request to be canceled")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("first request was not canceled: its cancel entry was overwritten")
	}
}

// 同じ保存済みリクエスト (同じ req.ID) でも execution ID が違えば並行送信でき、
// 片方をキャンセルしても他方は完走する。
func TestHTTPRequestService_SendRequest_SameRequestIDInParallel(t *testing.T) {
	started := make(chan string, 2)
	release := make(chan struct{})
	transport := &mockTransportCtx{
		doFn: func(ctx context.Context, executionID string, _ domain.HTTPRequest) (domain.HTTPResponse, error) {
			started <- executionID
			select {
			case <-ctx.Done():
				return domain.HTTPResponse{}, ctx.Err()
			case <-release:
				return domain.HTTPResponse{StatusCode: 200}, nil
			}
		},
	}
	svc := NewHTTPRequestService(context.Background(), transport, testutil.NoopLogger{})
	saved := domain.HTTPRequest{ID: "saved-1", Method: "GET", URL: "http://example.com"}

	results := make(map[string]chan error, 2)
	for _, execID := range []string{"exec-1", "exec-2"} {
		done := make(chan error, 1)
		results[execID] = done
		go func() {
			_, err := svc.SendRequest(execID, saved)
			done <- err
		}()
	}
	for range 2 {
		<-started
	}

	svc.CancelRequest("exec-1")
	select {
	case err := <-results["exec-1"]:
		if err == nil {
			t.Fatal("exec-1 must fail after being canceled")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("canceling exec-1 did not reach its execution")
	}

	close(release)
	select {
	case err := <-results["exec-2"]:
		if err != nil {
			t.Fatalf("exec-2 must finish even though exec-1 was canceled: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("exec-2 did not finish")
	}
}

func TestHTTPRequestService_Shutdown_CancelsInFlightAndRejectsNewSends(t *testing.T) {
	started := make(chan struct{})
	transport := &mockTransportCtx{
		doFn: func(ctx context.Context, _ string, _ domain.HTTPRequest) (domain.HTTPResponse, error) {
			close(started)
			<-ctx.Done()
			return domain.HTTPResponse{}, ctx.Err()
		},
	}
	svc := NewHTTPRequestService(context.Background(), transport, testutil.NoopLogger{})

	done := make(chan error, 1)
	go func() {
		_, err := svc.SendRequest("exec-1", domain.HTTPRequest{Method: "GET", URL: "http://example.com"})
		done <- err
	}()
	<-started

	if !svc.Shutdown(5 * time.Second) {
		t.Fatal("Shutdown timed out waiting for the in-flight request")
	}
	if err := <-done; err == nil {
		t.Fatal("expected the in-flight request to be canceled")
	}
	if _, err := svc.SendRequest("exec-2", domain.HTTPRequest{Method: "GET", URL: "http://example.com"}); err == nil {
		t.Fatal("SendRequest after Shutdown must fail")
	}
}

func TestHTTPRequestService_Shutdown_ReturnsFalseOnTimeout(t *testing.T) {
	release := make(chan struct{})
	started := make(chan struct{})
	transport := &mockTransportCtx{
		doFn: func(_ context.Context, _ string, _ domain.HTTPRequest) (domain.HTTPResponse, error) {
			close(started)
			<-release // キャンセルに応じない transport を模す
			return domain.HTTPResponse{}, nil
		},
	}
	svc := NewHTTPRequestService(context.Background(), transport, testutil.NoopLogger{})
	go func() {
		_, _ = svc.SendRequest("stuck", domain.HTTPRequest{Method: "GET", URL: "http://example.com"})
	}()
	<-started
	defer close(release)

	if svc.Shutdown(10 * time.Millisecond) {
		t.Fatal("Shutdown must report a timeout when a request ignores cancellation")
	}
}

// 注入した親 context のキャンセルは実行中リクエストの context まで伝わる。
func TestHTTPRequestService_ParentContextCancel_CancelsInFlight(t *testing.T) {
	started := make(chan struct{})
	transport := &mockTransportCtx{
		doFn: func(ctx context.Context, _ string, _ domain.HTTPRequest) (domain.HTTPResponse, error) {
			close(started)
			<-ctx.Done()
			return domain.HTTPResponse{}, ctx.Err()
		},
	}
	parent, cancelParent := context.WithCancel(context.Background())
	svc := NewHTTPRequestService(parent, transport, testutil.NoopLogger{})

	done := make(chan error, 1)
	go func() {
		_, err := svc.SendRequest("exec-1", domain.HTTPRequest{Method: "GET", URL: "http://example.com"})
		done <- err
	}()
	<-started

	cancelParent()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected the in-flight request to be canceled by the parent context")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("canceling the parent context did not reach the in-flight request")
	}
}

func TestHTTPRequestService_ConcurrentSendAndCancel(_ *testing.T) {
	// go test -race でデータ競合が検出されないことを確認する。
	// transport に入った時点でキャンセル登録済みなので、started 受信後に CancelRequest する。
	const n = 10
	started := make(chan struct{}, n)
	transport := &mockTransportCtx{
		doFn: func(ctx context.Context, _ string, _ domain.HTTPRequest) (domain.HTTPResponse, error) {
			started <- struct{}{}
			<-ctx.Done()
			return domain.HTTPResponse{}, ctx.Err()
		},
	}
	svc := NewHTTPRequestService(context.Background(), transport, testutil.NoopLogger{})

	done := make(chan struct{}, n)
	ids := make([]string, n)
	for i := range n {
		ids[i] = "exec-" + string(rune('a'+i))
		go func(id string) {
			svc.SendRequest(id, domain.HTTPRequest{ID: "saved-1", Method: "GET", URL: "http://example.com"})
			done <- struct{}{}
		}(ids[i])
	}

	for range n {
		<-started
	}
	for _, id := range ids {
		svc.CancelRequest(id)
	}
	for range n {
		<-done
	}
}
