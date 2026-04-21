package httpapp

import (
	"context"
	"errors"
	"testing"

	domain "github.com/f0reth/Wirexa/internal/domain/http"
	"github.com/f0reth/Wirexa/internal/testutil"
)

// mockTransport は HttpTransport のインメモリモック。
type mockTransport struct {
	doFn func(req domain.HttpRequest) (domain.HttpResponse, error)
}

func (m *mockTransport) Do(_ context.Context, req domain.HttpRequest) (domain.HttpResponse, error) {
	if m.doFn != nil {
		return m.doFn(req)
	}
	return domain.HttpResponse{StatusCode: 200}, nil
}

func TestHttpRequestService_SendRequest_ValidMethods(t *testing.T) {
	validMethods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}
	transport := &mockTransport{}
	svc := NewHTTPRequestService(transport, testutil.NoopLogger{})

	for _, method := range validMethods {
		t.Run(method, func(t *testing.T) {
			resp, err := svc.SendRequest(domain.HttpRequest{Method: method, URL: "http://example.com"})
			if err != nil {
				t.Fatalf("SendRequest(%q): unexpected error %v", method, err)
			}
			if resp.StatusCode != 200 {
				t.Errorf("expected 200, got %d", resp.StatusCode)
			}
		})
	}
}

func TestHttpRequestService_SendRequest_InvalidMethods(t *testing.T) {
	invalidMethods := []string{"get", "post", "TRACE", "CONNECT", "", "INVALID", "get "}
	transport := &mockTransport{}
	svc := NewHTTPRequestService(transport, testutil.NoopLogger{})

	for _, method := range invalidMethods {
		t.Run("invalid_"+method, func(t *testing.T) {
			_, err := svc.SendRequest(domain.HttpRequest{Method: method, URL: "http://example.com"})
			if err == nil {
				t.Fatalf("SendRequest(%q): expected error, got nil", method)
			}
		})
	}
}

func TestHttpRequestService_SendRequest_TransportError(t *testing.T) {
	wantErr := errors.New("network unreachable")
	transport := &mockTransport{
		doFn: func(_ domain.HttpRequest) (domain.HttpResponse, error) {
			return domain.HttpResponse{}, wantErr
		},
	}
	svc := NewHTTPRequestService(transport, testutil.NoopLogger{})

	_, err := svc.SendRequest(domain.HttpRequest{Method: "GET", URL: "http://example.com"})
	if !errors.Is(err, wantErr) {
		t.Errorf("expected network error, got %v", err)
	}
}

func TestHttpRequestService_SendRequest_TransportResponse(t *testing.T) {
	wantResp := domain.HttpResponse{
		StatusCode:  404,
		StatusText:  "Not Found",
		Body:        `{"error":"not found"}`,
		ContentType: "application/json",
		TimingMs:    42,
	}
	transport := &mockTransport{
		doFn: func(_ domain.HttpRequest) (domain.HttpResponse, error) {
			return wantResp, nil
		},
	}
	svc := NewHTTPRequestService(transport, testutil.NoopLogger{})

	got, err := svc.SendRequest(domain.HttpRequest{Method: "GET", URL: "http://example.com"})
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

func TestHttpRequestService_SendRequest_PassesRequestToTransport(t *testing.T) {
	var capturedReq domain.HttpRequest
	transport := &mockTransport{
		doFn: func(r domain.HttpRequest) (domain.HttpResponse, error) {
			capturedReq = r
			return domain.HttpResponse{StatusCode: 200}, nil
		},
	}
	svc := NewHTTPRequestService(transport, testutil.NoopLogger{})

	input := domain.HttpRequest{
		Method: "POST",
		URL:    "http://example.com/api",
		Headers: []domain.KeyValuePair{
			{Key: "Content-Type", Value: "application/json", Enabled: true},
		},
		Body: domain.RequestBody{Type: "json", Contents: map[string]string{"json": `{"x":1}`}},
		Auth: domain.RequestAuth{Type: "bearer", Token: "tok123"},
	}
	svc.SendRequest(input)

	if capturedReq.URL != input.URL {
		t.Errorf("URL = %q, want %q", capturedReq.URL, input.URL)
	}
	if capturedReq.Auth.Token != "tok123" {
		t.Errorf("Auth.Token = %q, want %q", capturedReq.Auth.Token, "tok123")
	}
}

// mockTransportCtx はコンテキストを受け取る doFn を持つ transport モック。
type mockTransportCtx struct {
	doFn func(ctx context.Context, req domain.HttpRequest) (domain.HttpResponse, error)
}

func (m *mockTransportCtx) Do(ctx context.Context, req domain.HttpRequest) (domain.HttpResponse, error) {
	if m.doFn != nil {
		return m.doFn(ctx, req)
	}
	return domain.HttpResponse{StatusCode: 200}, nil
}

func TestHttpRequestService_CancelRequest_NonExistentID(_ *testing.T) {
	svc := NewHTTPRequestService(&mockTransport{}, testutil.NoopLogger{})
	svc.CancelRequest("no-such-id")
}

func TestHttpRequestService_CancelRequest_ValidID(t *testing.T) {
	started := make(chan struct{})
	transport := &mockTransportCtx{
		doFn: func(ctx context.Context, _ domain.HttpRequest) (domain.HttpResponse, error) {
			close(started)
			<-ctx.Done()
			return domain.HttpResponse{}, ctx.Err()
		},
	}
	svc := NewHTTPRequestService(transport, testutil.NoopLogger{})

	const reqID = "req-cancel-test"
	done := make(chan error, 1)
	go func() {
		_, err := svc.SendRequest(domain.HttpRequest{ID: reqID, Method: "GET", URL: "http://example.com"})
		done <- err
	}()

	<-started
	svc.CancelRequest(reqID)
	if err := <-done; err == nil {
		t.Error("expected context cancellation error, got nil")
	}
}

func TestHttpRequestService_ConcurrentSendAndCancel(_ *testing.T) {
	// go test -race でデータ競合が検出されないことを確認する。
	// transport に入った時点でキャンセル登録済みなので、started 受信後に CancelRequest する。
	const n = 10
	started := make(chan struct{}, n)
	transport := &mockTransportCtx{
		doFn: func(ctx context.Context, _ domain.HttpRequest) (domain.HttpResponse, error) {
			started <- struct{}{}
			<-ctx.Done()
			return domain.HttpResponse{}, ctx.Err()
		},
	}
	svc := NewHTTPRequestService(transport, testutil.NoopLogger{})

	done := make(chan struct{}, n)
	ids := make([]string, n)
	for i := range n {
		ids[i] = "req-" + string(rune('a'+i))
		go func(id string) {
			svc.SendRequest(domain.HttpRequest{ID: id, Method: "GET", URL: "http://example.com"})
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
