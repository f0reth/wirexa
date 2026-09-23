package httpinfra

import (
	"bytes"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/textproto"
	"strings"
	"sync/atomic"
	"testing"

	domain "github.com/f0reth/Wirexa/internal/domain/http"
)

// fakeFile は選択済みファイルのテスト用の名前・Content-Type・内容。
type fakeFile struct {
	name        string
	contentType string
	data        string
}

// fakeFiles は SelectedFileOpener のテスト用実装。token → 選択済みファイル。
type fakeFiles map[string]fakeFile

func (f fakeFiles) OpenSelectedFile(token string) (domain.OpenedSelectedFile, error) {
	file, ok := f[token]
	if !ok {
		return domain.OpenedSelectedFile{}, domain.ErrFileAccessDenied
	}
	return domain.OpenedSelectedFile{
		File:        &memHandle{data: []byte(file.data)},
		Name:        file.name,
		ContentType: file.contentType,
		Size:        int64(len(file.data)),
	}, nil
}

var testFiles = fakeFiles{
	"tok-json": {name: "hello.json", contentType: "application/json", data: `{"from":"file"}`},
	"tok-bin":  {name: "a.bin", contentType: "application/octet-stream", data: "bytes"},
}

// memHandle は SelectedFileHandle のテスト用実装。CheckUnchanged の結果と呼び出し回数、
// Close の回数を確かめられる。
type memHandle struct {
	// check は n 回目 (1 始まり) の CheckUnchanged の結果を返す。nil なら常に成功する。
	check  func(n int32) error
	data   []byte
	checks atomic.Int32
	closes atomic.Int32
}

func (h *memHandle) ReadAt(p []byte, off int64) (int, error) {
	return bytes.NewReader(h.data).ReadAt(p, off)
}

func (h *memHandle) CheckUnchanged() error {
	n := h.checks.Add(1)
	if h.check != nil {
		return h.check(n)
	}
	return nil
}

func (h *memHandle) Close() error {
	h.closes.Add(1)
	return nil
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

	body, contentType, err := buildMultipartBody(rows, testFiles)
	if err != nil {
		t.Fatalf("buildMultipartBody() error = %v", err)
	}
	data := readBody(t, body)

	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		t.Fatalf("ParseMediaType(%q) error = %v", contentType, err)
	}

	mr := multipart.NewReader(bytes.NewReader(data), params["boundary"])
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

// readBody はボディを先頭から読み、読めたバイト数が合計長と一致することを確かめてから release する。
func readBody(t *testing.T, body *bodySegments) []byte {
	t.Helper()
	defer body.release()
	var errs bodyError
	rc, err := body.newReader(&errs)
	if err != nil {
		t.Fatalf("newReader() error = %v", err)
	}
	defer func() { _ = rc.Close() }()
	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if int64(len(data)) != body.length {
		t.Fatalf("read %d bytes, but the body length is %d", len(data), body.length)
	}
	if err = errs.get(); err != nil {
		t.Fatalf("bodyError = %v", err)
	}
	return data
}

// ストリーミングにしても、multipart の出力は従来の bytes.Buffer 実装と 1 バイトも違わない。
// 従来の実装 (パートヘッダを CreatePart で書き、text 行は WriteField、json と file 行は
// 内容を part.Write) を同じ boundary で組み立てて比べる。
func TestBuildMultipartBody_MatchesBufferedOutput(t *testing.T) {
	files := fakeFiles{
		"tok-a":     {name: `ファイル "a".json`, contentType: "application/json", data: `{"from":"file"}`},
		"tok-b":     {name: `b\c.bin`, contentType: "application/octet-stream", data: "\x00\x01\r\n--boundary-like\r\n"},
		"tok-empty": {name: "empty.txt", contentType: "text/plain", data: ""},
	}
	rows := []domain.FormRow{
		{Key: "z", Value: "first", Kind: domain.FormRowKindText, Enabled: true},
		{Key: "a", File: domain.FileReference{Token: "tok-a"}, Kind: domain.FormRowKindFile, Enabled: true},
		{Key: `we"ird`, Value: `{"k":"v"}`, Kind: domain.FormRowKindJSON, Enabled: true},
		{Key: "skip", Value: "x", Enabled: false},
		{Key: "b", File: domain.FileReference{Token: "tok-b"}, Kind: domain.FormRowKindFile, ContentType: "image/png", Enabled: true},
		{Key: "b", File: domain.FileReference{Token: "tok-b"}, Kind: domain.FormRowKindFile, Enabled: true},
		{Key: "e", File: domain.FileReference{Token: "tok-empty"}, Kind: domain.FormRowKindFile, Enabled: true},
		{Key: "csv", Value: "1,2", Kind: domain.FormRowKindText, ContentType: "text/csv", Enabled: true},
		{Key: "last", Value: "日本語", Enabled: true},
	}

	body, contentType, err := buildMultipartBody(rows, files)
	if err != nil {
		t.Fatalf("buildMultipartBody() error = %v", err)
	}
	got := readBody(t, body)
	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		t.Fatalf("ParseMediaType(%q) error = %v", contentType, err)
	}

	var want bytes.Buffer
	mw := multipart.NewWriter(&want)
	if err = mw.SetBoundary(params["boundary"]); err != nil {
		t.Fatalf("SetBoundary() error = %v", err)
	}
	writeBuffered := func(name, filename, contentType, data string) {
		disposition := `form-data; name="` + quoteEscaper.Replace(name) + `"`
		if filename != "" {
			disposition += `; filename="` + quoteEscaper.Replace(filename) + `"`
		}
		h := textproto.MIMEHeader{}
		h.Set("Content-Disposition", disposition)
		h.Set("Content-Type", contentType)
		p, perr := mw.CreatePart(h)
		if perr != nil {
			t.Fatalf("CreatePart() error = %v", perr)
		}
		_, _ = p.Write([]byte(data))
	}
	_ = mw.WriteField("z", "first")
	writeBuffered("a", files["tok-a"].name, "application/json", files["tok-a"].data)
	writeBuffered(`we"ird`, "", "application/json", `{"k":"v"}`)
	writeBuffered("b", files["tok-b"].name, "image/png", files["tok-b"].data)
	writeBuffered("b", files["tok-b"].name, "application/octet-stream", files["tok-b"].data)
	writeBuffered("e", "empty.txt", "text/plain", "")
	writeBuffered("csv", "", "text/csv", "1,2")
	_ = mw.WriteField("last", "日本語")
	if err = mw.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	if !bytes.Equal(got, want.Bytes()) {
		t.Fatalf("streamed body differs from the buffered one\n got: %q\nwant: %q", got, want.Bytes())
	}
}

// recordingFiles は開いたハンドルを覚えておき、閉じられたかを確かめられる SelectedFileOpener。
type recordingFiles struct {
	files  fakeFiles
	opened []*memHandle
}

func (r *recordingFiles) OpenSelectedFile(token string) (domain.OpenedSelectedFile, error) {
	file, err := r.files.OpenSelectedFile(token)
	if err != nil {
		return file, err
	}
	h, _ := file.File.(*memHandle)
	r.opened = append(r.opened, h)
	return file, nil
}

// 途中の行でエラーになったら、それまでに開いたファイルはすべて閉じる。
func TestBuildMultipartBody_ClosesOpenedFilesOnError(t *testing.T) {
	files := &recordingFiles{files: testFiles}
	_, _, err := buildMultipartBody([]domain.FormRow{
		{Key: "a", File: domain.FileReference{Token: "tok-json"}, Kind: domain.FormRowKindFile, Enabled: true},
		{Key: "b", File: domain.FileReference{Token: "tok-bin"}, Kind: domain.FormRowKindFile, Enabled: true},
		{Key: "c", File: domain.FileReference{Token: "forged"}, Kind: domain.FormRowKindFile, Enabled: true},
	}, files)
	if !errors.Is(err, domain.ErrFileAccessDenied) {
		t.Fatalf("buildMultipartBody() error = %v, want ErrFileAccessDenied", err)
	}
	if len(files.opened) != 2 {
		t.Fatalf("opened %d files, want 2", len(files.opened))
	}
	for i, h := range files.opened {
		if n := h.closes.Load(); n != 1 {
			t.Errorf("file #%d closed %d times, want 1", i, n)
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
