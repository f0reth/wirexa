package adapters

import (
	"fmt"
	"reflect"
	"testing"

	httpdomain "github.com/f0reth/Wirexa/internal/domain/http"
)

// fullHTTPRequest はすべてのフィールドに非ゼロ値を入れたドメインリクエストを返す。
// ドメイン構造体にフィールドを足したらここも更新する必要があり（assertNoZeroFields が強制）、
// その際に変換関数のコピー漏れが往復テストで露見する。
func fullHTTPRequest() httpdomain.HttpRequest {
	return httpdomain.HttpRequest{
		ID:      "id-1",
		Name:    "name-1",
		Method:  "POST",
		URL:     "https://example.com",
		Doc:     "doc-1",
		Headers: []httpdomain.KeyValuePair{{Key: "h", Value: "v", Enabled: true}},
		Params:  []httpdomain.KeyValuePair{{Key: "p", Value: "q", Enabled: true}},
		Body:    httpdomain.RequestBody{Type: "json", Contents: map[string]string{"json": "{}"}},
		Auth:    httpdomain.RequestAuth{Type: "basic", Username: "u", Password: "p", Token: "t"},
		Settings: httpdomain.RequestSettings{
			ProxyMode:          "custom",
			ProxyURL:           "http://proxy",
			TimeoutSec:         30,
			MaxResponseBodyMB:  10,
			InsecureSkipVerify: true,
			DisableRedirects:   true,
		},
	}
}

// fullHTTPResponse はすべてのフィールドに非ゼロ値を入れたドメインレスポンスを返す。
func fullHTTPResponse() httpdomain.HttpResponse {
	return httpdomain.HttpResponse{
		Headers:       map[string]string{"a": "b"},
		StatusText:    "OK",
		Body:          "body",
		ContentType:   "application/json",
		Error:         "err",
		StatusCode:    200,
		Size:          12,
		TimingMs:      34,
		BodyTruncated: true,
	}
}

// TestHTTPRequestDTORoundTrip はドメイン→DTO→ドメインの往復で全フィールドが保持されることを検証する。
// 変換関数（to/fromHTTPRequestDTO）でフィールドのコピーを忘れると往復結果が一致せず失敗する。
func TestHTTPRequestDTORoundTrip(t *testing.T) {
	orig := fullHTTPRequest()
	// フィクスチャ自体が全フィールドを埋めていることを保証する（埋め漏れがあると往復テストが無意味になるため）。
	assertNoZeroFields(t, "HttpRequest(domain)", reflect.ValueOf(orig))

	got := fromHTTPRequestDTO(toHTTPRequestDTO(orig))
	if !reflect.DeepEqual(orig, got) {
		t.Fatalf("round trip mismatch:\n orig=%+v\n got =%+v", orig, got)
	}
}

// TestHTTPResponseDTOMapping はドメイン→DTO変換でレスポンスの全フィールドが欠落しないことを検証する。
// レスポンスには逆変換が無いため、DTO 側にゼロ値フィールドが残っていないかで欠落を検出する。
func TestHTTPResponseDTOMapping(t *testing.T) {
	orig := fullHTTPResponse()
	assertNoZeroFields(t, "HttpResponse(domain)", reflect.ValueOf(orig))

	dto := toHTTPResponseDTO(orig, "/tmp/body")
	assertNoZeroFields(t, "HttpResponse(dto)", reflect.ValueOf(dto))
}

// assertNoZeroFields は構造体・スライス・マップを再帰的に走査し、ゼロ値の末端フィールドがあれば失敗させる。
// 「全フィールドが埋まっている」ことを構造体定義に対して機械的に保証することで、
// フィールド追加時にフィクスチャ更新と変換漏れチェックを強制する。
func assertNoZeroFields(t *testing.T, path string, v reflect.Value) {
	t.Helper()
	switch v.Kind() {
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			f := v.Type().Field(i)
			if !f.IsExported() {
				continue
			}
			assertNoZeroFields(t, path+"."+f.Name, v.Field(i))
		}
	case reflect.Slice, reflect.Map:
		if v.Len() == 0 {
			t.Errorf("%s is empty; fixture must populate every field", path)
			return
		}
		if v.Kind() == reflect.Slice {
			for i := 0; i < v.Len(); i++ {
				assertNoZeroFields(t, fmt.Sprintf("%s[%d]", path, i), v.Index(i))
			}
			return
		}
		for _, k := range v.MapKeys() {
			assertNoZeroFields(t, fmt.Sprintf("%s[%v]", path, k.Interface()), v.MapIndex(k))
		}
	case reflect.Pointer:
		if v.IsNil() {
			t.Errorf("%s is nil; fixture must populate every field", path)
			return
		}
		assertNoZeroFields(t, path, v.Elem())
	default:
		if v.IsZero() {
			t.Errorf("%s is zero; fixture must populate every field so the round-trip test stays meaningful", path)
		}
	}
}
