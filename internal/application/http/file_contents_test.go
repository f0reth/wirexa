package httpapp

import (
	"context"
	"testing"

	domain "github.com/f0reth/Wirexa/internal/domain/http"
	"github.com/f0reth/Wirexa/internal/testutil"
)

// fileBody は Contents の file キーにパスを置いた (旧形式・偽装の) file ボディを返す。
func fileBody() domain.RequestBody {
	return domain.RequestBody{
		Type:     domain.BodyTypeFile,
		Contents: map[string]string{domain.BodyTypeFile: "/etc/passwd", "json": "{}"},
	}
}

// file ボディの送信元は token だけなので、Contents に置かれたパスは
// 追加・更新のどちらでもキャッシュ (= RPC 応答) に残さない。
func TestCollectionService_DropsFileContents(t *testing.T) {
	svc := newSvc(t)
	if _, err := svc.AddRequest(domain.RootCollectionID, "", domain.HTTPRequest{ID: "r1", Method: "POST", Body: fileBody()}); err != nil {
		t.Fatalf("AddRequest: %v", err)
	}
	assertNoFileContents(t, svc)

	if err := svc.UpdateRequest(domain.RootCollectionID, domain.HTTPRequest{ID: "r1", Method: "POST", Body: fileBody()}); err != nil {
		t.Fatalf("UpdateRequest: %v", err)
	}
	assertNoFileContents(t, svc)
}

func assertNoFileContents(t *testing.T, svc *CollectionService) {
	t.Helper()
	items := svc.GetRootItems()
	if len(items) != 1 {
		t.Fatalf("root items = %d, want 1", len(items))
	}
	contents := items[0].Request.Body.Contents
	if _, ok := contents[domain.BodyTypeFile]; ok {
		t.Fatalf("contents still hold a file path: %v", contents)
	}
	if contents["json"] != "{}" {
		t.Fatalf("other contents must be kept: %v", contents)
	}
}

func TestHTTPRequestService_SendRequest_DropsFileContents(t *testing.T) {
	var got domain.HTTPRequest
	transport := &mockTransport{doFn: func(r domain.HTTPRequest) (domain.HTTPResponse, error) {
		got = r
		return domain.HTTPResponse{StatusCode: 200}, nil
	}}
	svc := NewHTTPRequestService(context.Background(), transport, testutil.NoopLogger{})

	if _, err := svc.SendRequest("exec-1", domain.HTTPRequest{Method: "POST", URL: "http://example.com", Body: fileBody()}); err != nil {
		t.Fatalf("SendRequest: %v", err)
	}
	if _, ok := got.Body.Contents[domain.BodyTypeFile]; ok {
		t.Fatalf("transport received a file path in contents: %v", got.Body.Contents)
	}
}
