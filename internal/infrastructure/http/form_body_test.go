package httpinfra

import (
	"io"
	"mime"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"testing"

	domain "github.com/f0reth/Wirexa/internal/domain/http"
)

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

	buf, contentType, err := buildMultipartBody(rows)
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
			name: "パス未入力の file 行は送らない（まだ書きかけの行）",
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
	dir := t.TempDir()
	path := filepath.Join(dir, "hello.json")
	if err := os.WriteFile(path, []byte(`{"from":"file"}`), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	t.Run("ファイルの中身と filename を載せる", func(t *testing.T) {
		parts := readParts(t, []domain.FormRow{
			{Key: "f", FilePath: path, Kind: domain.FormRowKindFile, Enabled: true},
		})
		if len(parts) != 1 {
			t.Fatalf("parts = %+v, want 1 part", parts)
		}
		if parts[0].filename != "hello.json" {
			t.Errorf("filename = %q, want hello.json", parts[0].filename)
		}
		if parts[0].body != `{"from":"file"}` {
			t.Errorf("body = %q, want the file contents", parts[0].body)
		}
		// 具体的な MIME は OS 依存なので、拡張子から判定できていることだけ見る。
		if parts[0].contentType == "" || parts[0].contentType == "application/octet-stream" {
			t.Errorf("contentType = %q, want a type guessed from .json", parts[0].contentType)
		}
	})

	t.Run("行の Content-Type は拡張子判定より優先する", func(t *testing.T) {
		parts := readParts(t, []domain.FormRow{
			{Key: "f", FilePath: path, Kind: domain.FormRowKindFile, ContentType: "text/plain", Enabled: true},
		})
		if len(parts) != 1 || parts[0].contentType != "text/plain" {
			t.Errorf("parts = %+v, want contentType text/plain", parts)
		}
	})

	t.Run("Value ではなくファイルの中身を送る", func(t *testing.T) {
		parts := readParts(t, []domain.FormRow{
			{Key: "f", Value: "stale text", FilePath: path, Kind: domain.FormRowKindFile, Enabled: true},
		})
		if len(parts) != 1 || parts[0].body != `{"from":"file"}` {
			t.Errorf("parts = %+v, want the file contents", parts)
		}
	})

	t.Run("読めないファイルはエラーにする", func(t *testing.T) {
		_, _, err := buildMultipartBody([]domain.FormRow{
			{Key: "f", FilePath: filepath.Join(dir, "missing.txt"), Kind: domain.FormRowKindFile, Enabled: true},
		})
		if err == nil {
			t.Error("buildMultipartBody() error = nil, want an error")
		}
	})
}

// 混在ボディで行の順序が保たれ、種別ごとの扱いが同時に成立することを確認する。
func TestBuildMultipartBody_MixedRowsKeepOrder(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.bin")
	if err := os.WriteFile(path, []byte("bytes"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	parts := readParts(t, []domain.FormRow{
		{Key: "z", Value: "first", Kind: domain.FormRowKindText, Enabled: true},
		{Key: "j", Value: "{}", Kind: domain.FormRowKindJSON, Enabled: true},
		{Key: "f", FilePath: path, Kind: domain.FormRowKindFile, ContentType: "application/octet-stream", Enabled: true},
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
	_, contentType, err := buildMultipartBody(nil)
	if err != nil {
		t.Fatalf("buildMultipartBody() error = %v", err)
	}
	if !strings.HasPrefix(contentType, "multipart/form-data; boundary=") {
		t.Errorf("contentType = %q, want a multipart type with boundary", contentType)
	}
}
