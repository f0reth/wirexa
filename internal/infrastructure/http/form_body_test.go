package httpinfra

import (
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"strings"
	"testing"

	domain "github.com/f0reth/Wirexa/internal/domain/http"
)

// fakeFiles は SelectedFileReader のテスト用実装。token → 選択済みファイル。
type fakeFiles map[string]domain.SelectedFileContent

func (f fakeFiles) ReadSelectedFile(token string) (domain.SelectedFileContent, error) {
	file, ok := f[token]
	if !ok {
		return domain.SelectedFileContent{}, domain.ErrFileAccessDenied
	}
	return file, nil
}

var testFiles = fakeFiles{
	"tok-json": {Name: "hello.json", ContentType: "application/json", Data: []byte(`{"from":"file"}`)},
	"tok-bin":  {Name: "a.bin", ContentType: "application/octet-stream", Data: []byte("bytes")},
}

// part は読み戻した multipart の 1 パート。
type part struct {
	name        string
	filename    string
	contentType string
	body        string
}

// readParts は buildMultipartBody の出力を解析して各パートを返す。
func readParts(t *testing.T, rows []domain.FormRow) []part {
	t.Helper()

	buf, contentType, err := buildMultipartBody(rows, testFiles)
	if err != nil {
		t.Fatalf("buildMultipartBody() error = %v", err)
	}

	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		t.Fatalf("ParseMediaType(%q) error = %v", contentType, err)
	}

	mr := multipart.NewReader(buf, params["boundary"])
	var parts []part
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("NextPart() error = %v", err)
		}
		body, err := io.ReadAll(p)
		if err != nil {
			t.Fatalf("ReadAll() error = %v", err)
		}
		parts = append(parts, part{
			name:        p.FormName(),
			filename:    p.FileName(),
			contentType: p.Header.Get("Content-Type"),
			body:        string(body),
		})
	}
	return parts
}

func TestBuildMultipartBody_SkipsRows(t *testing.T) {
	tests := []struct {
		name string
		row  domain.FormRow
	}{
		{
			name: "無効行は送らない",
			row:  domain.FormRow{Key: "a", Value: "1", Enabled: false},
		},
		{
			name: "空キー行は送らない（編集中の未入力行）",
			row:  domain.FormRow{Key: "", Value: "1", Enabled: true},
		},
		{
			name: "ファイル未選択の file 行は送らない（まだ書きかけの行）",
			row:  domain.FormRow{Key: "f", Kind: domain.FormRowKindFile, Enabled: true},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if parts := readParts(t, []domain.FormRow{tc.row}); len(parts) != 0 {
				t.Errorf("parts = %+v, want none", parts)
			}
		})
	}
}

// Content-Type 未指定の text 行は行機能の追加前と同じ出力（パートに Content-Type を
// 付けない）でなければならない。既存リクエストのワイヤ形式を変えないための保証。
func TestBuildMultipartBody_TextRow(t *testing.T) {
	t.Run("Content-Type 未指定なら付けない", func(t *testing.T) {
		parts := readParts(t, []domain.FormRow{
			{Key: "a", Value: "1", Kind: domain.FormRowKindText, Enabled: true},
		})
		want := []part{{name: "a", body: "1"}}
		assertParts(t, parts, want)
	})

	t.Run("Kind 未設定（旧データ）も text として送る", func(t *testing.T) {
		parts := readParts(t, []domain.FormRow{
			{Key: "a", Value: "1", Enabled: true},
		})
		want := []part{{name: "a", body: "1"}}
		assertParts(t, parts, want)
	})

	t.Run("Content-Type 指定時はパートに載せる", func(t *testing.T) {
		parts := readParts(t, []domain.FormRow{
			{Key: "a", Value: "1", Kind: domain.FormRowKindText, ContentType: "text/csv", Enabled: true},
		})
		want := []part{{name: "a", contentType: "text/csv", body: "1"}}
		assertParts(t, parts, want)
	})
}

func TestBuildMultipartBody_JSONRow(t *testing.T) {
	t.Run("Content-Type 未指定なら application/json を自動付与する", func(t *testing.T) {
		parts := readParts(t, []domain.FormRow{
			{Key: "j", Value: `{"k":"v"}`, Kind: domain.FormRowKindJSON, Enabled: true},
		})
		want := []part{{name: "j", contentType: "application/json", body: `{"k":"v"}`}}
		assertParts(t, parts, want)
	})

	t.Run("行の Content-Type は自動判定より優先する", func(t *testing.T) {
		parts := readParts(t, []domain.FormRow{
			{Key: "j", Value: "{}", Kind: domain.FormRowKindJSON, ContentType: "application/ld+json", Enabled: true},
		})
		want := []part{{name: "j", contentType: "application/ld+json", body: "{}"}}
		assertParts(t, parts, want)
	})
}

func TestBuildMultipartBody_FileRow(t *testing.T) {
	t.Run("選択済みファイルの中身と filename・Content-Type を載せる", func(t *testing.T) {
		parts := readParts(t, []domain.FormRow{
			{Key: "f", File: domain.FileReference{Token: "tok-json"}, Kind: domain.FormRowKindFile, Enabled: true},
		})
		want := []part{{name: "f", filename: "hello.json", contentType: "application/json", body: `{"from":"file"}`}}
		assertParts(t, parts, want)
	})

	t.Run("行の Content-Type は選択時の Content-Type より優先する", func(t *testing.T) {
		parts := readParts(t, []domain.FormRow{
			{Key: "f", File: domain.FileReference{Token: "tok-json"}, Kind: domain.FormRowKindFile, ContentType: "text/plain", Enabled: true},
		})
		if len(parts) != 1 || parts[0].contentType != "text/plain" {
			t.Errorf("parts = %+v, want contentType text/plain", parts)
		}
	})

	t.Run("frontend から渡された名前と Content-Type は使わない", func(t *testing.T) {
		parts := readParts(t, []domain.FormRow{
			{Key: "f", File: domain.FileReference{Token: "tok-json", Name: "evil.exe", ContentType: "application/x-evil"}, Kind: domain.FormRowKindFile, Enabled: true},
		})
		want := []part{{name: "f", filename: "hello.json", contentType: "application/json", body: `{"from":"file"}`}}
		assertParts(t, parts, want)
	})

	t.Run("Value ではなくファイルの中身を送る", func(t *testing.T) {
		parts := readParts(t, []domain.FormRow{
			{Key: "f", Value: "stale text", File: domain.FileReference{Token: "tok-json"}, Kind: domain.FormRowKindFile, Enabled: true},
		})
		if len(parts) != 1 || parts[0].body != `{"from":"file"}` {
			t.Errorf("parts = %+v, want the file contents", parts)
		}
	})

	rejected := []struct {
		name string
		ref  domain.FileReference
	}{
		{name: "未登録の token は拒否する", ref: domain.FileReference{Token: "forged"}},
		{name: "再選択待ちの参照は拒否する", ref: domain.FileReference{Name: "hello.json", NeedsReselect: true}},
	}
	for _, tc := range rejected {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := buildMultipartBody([]domain.FormRow{
				{Key: "f", File: tc.ref, Kind: domain.FormRowKindFile, Enabled: true},
			}, testFiles)
			if !errors.Is(err, domain.ErrFileAccessDenied) {
				t.Errorf("buildMultipartBody() error = %v, want ErrFileAccessDenied", err)
			}
		})
	}
}

// 混在ボディで行の順序が保たれ、種別ごとの扱いが同時に成立することを確認する。
func TestBuildMultipartBody_MixedRowsKeepOrder(t *testing.T) {
	parts := readParts(t, []domain.FormRow{
		{Key: "z", Value: "first", Kind: domain.FormRowKindText, Enabled: true},
		{Key: "j", Value: "{}", Kind: domain.FormRowKindJSON, Enabled: true},
		{Key: "f", File: domain.FileReference{Token: "tok-bin"}, Kind: domain.FormRowKindFile, Enabled: true},
		{Key: "a", Value: "last", Kind: domain.FormRowKindText, Enabled: true},
	})

	want := []part{
		{name: "z", body: "first"},
		{name: "j", contentType: "application/json", body: "{}"},
		{name: "f", filename: "a.bin", contentType: "application/octet-stream", body: "bytes"},
		{name: "a", body: "last"},
	}
	assertParts(t, parts, want)
}

// 引用符を含むキーやファイル名が Content-Disposition を壊さないことを確認する。
func TestBuildMultipartBody_EscapesQuotes(t *testing.T) {
	parts := readParts(t, []domain.FormRow{
		{Key: `we"ird`, Value: "1", Kind: domain.FormRowKindJSON, Enabled: true},
	})
	if len(parts) != 1 || parts[0].name != `we"ird` {
		t.Errorf("parts = %+v, want name we\"ird", parts)
	}
}

func assertParts(t *testing.T, got, want []part) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("parts = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("part[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// buildMultipartBody が返す Content-Type は採番済みの boundary を含む必要がある。
func TestBuildMultipartBody_ContentTypeCarriesBoundary(t *testing.T) {
	_, contentType, err := buildMultipartBody(nil, nil)
	if err != nil {
		t.Fatalf("buildMultipartBody() error = %v", err)
	}
	if !strings.HasPrefix(contentType, "multipart/form-data; boundary=") {
		t.Errorf("contentType = %q, want a multipart type with boundary", contentType)
	}
}
