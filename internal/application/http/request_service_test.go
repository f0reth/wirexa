package httpapp

import (
	"context"
	"errors"
	"testing"

	domain "github.com/f0reth/Wirexa/internal/domain/http"
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
	svc := NewHTTPRequestService(transport)

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
	svc := NewHTTPRequestService(transport)

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
	svc := NewHTTPRequestService(transport)

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
	svc := NewHTTPRequestService(transport)

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
	svc := NewHTTPRequestService(transport)

	input := domain.HttpRequest{
		Method: "POST",
		URL:    "http://example.com/api",
		Headers: []domain.KeyValuePair{
			{Key: "Content-Type", Value: "application/json", Enabled: true},
		},
		Body: domain.RequestBody{Type: "json", Content: `{"x":1}`},
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
