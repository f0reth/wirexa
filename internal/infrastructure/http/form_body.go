package httpinfra

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"

	domain "github.com/f0reth/Wirexa/internal/domain/http"
)

// quoteEscaper は Content-Disposition のパラメータ値をエスケープする。
// mime/multipart の同等品が非公開のため同じ変換をここに置く。
var quoteEscaper = strings.NewReplacer("\\", "\\\\", `"`, "\\\"")

// buildMultipartBody は form-data の行から multipart ボディと Content-Type を組み立てる。
// 返す Content-Type は採番済みの boundary を含むため、呼び出し側で必ず優先させる。
func buildMultipartBody(rows []domain.FormRow) (*bytes.Buffer, string, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for i := range rows {
		r := &rows[i]
		if !r.Enabled || r.Key == "" {
			continue
		}
		if err := writeFormRow(mw, r); err != nil {
			return nil, "", err
		}
	}
	if err := mw.Close(); err != nil {
		return nil, "", fmt.Errorf("failed to close multipart writer: %w", err)
	}
	return &buf, mw.FormDataContentType(), nil
}

// writeFormRow は 1 行をパートとして書き出す。
func writeFormRow(mw *multipart.Writer, r *domain.FormRow) error {
	switch r.EffectiveKind() {
	case domain.FormRowKindFile:
		// パス未入力の行はまだ書きかけなので、無効行と同じく黙って飛ばす。
		if r.FilePath == "" {
			return nil
		}
		// パスはユーザーがファイルダイアログで選ぶ（既存の file ボディ型と同じ扱い）。
		data, err := os.ReadFile(r.FilePath)
		if err != nil {
			return fmt.Errorf("failed to read form file: %w", err)
		}
		contentType := r.ContentType
		if contentType == "" {
			contentType = domain.GuessFileContentType(r.FilePath)
		}
		return writePart(mw, r.Key, filepath.Base(r.FilePath), contentType, data)
	case domain.FormRowKindJSON:
		contentType := r.ContentType
		if contentType == "" {
			contentType = "application/json"
		}
		return writePart(mw, r.Key, "", contentType, []byte(r.Value))
	default:
		// Content-Type 未指定の text 行は WriteField に通す。パートに Content-Type を
		// 付けないのが従来の出力で、行の追加機能のために既存リクエストのワイヤ形式を
		// 変えてしまわないようにする。
		if r.ContentType == "" {
			if err := mw.WriteField(r.Key, r.Value); err != nil {
				return fmt.Errorf("failed to write form field: %w", err)
			}
			return nil
		}
		return writePart(mw, r.Key, "", r.ContentType, []byte(r.Value))
	}
}

// writePart は Content-Type 付きのパートを書く。
// multipart.CreateFormFile は Content-Type を application/octet-stream に固定するため使えない。
func writePart(mw *multipart.Writer, name, filename, contentType string, data []byte) error {
	// %q は使えない。エスケープが二重になるうえ、非 ASCII のファイル名を
	// \u エスケープへ潰してしまう（stdlib の multipart も同じく手組みしている）。
	disposition := `form-data; name="` + quoteEscaper.Replace(name) + `"`
	if filename != "" {
		disposition += `; filename="` + quoteEscaper.Replace(filename) + `"`
	}
	h := textproto.MIMEHeader{}
	h.Set("Content-Disposition", disposition)
	h.Set("Content-Type", contentType)

	part, err := mw.CreatePart(h)
	if err != nil {
		return fmt.Errorf("failed to create form part: %w", err)
	}
	if _, err = part.Write(data); err != nil {
		return fmt.Errorf("failed to write form part: %w", err)
	}
	return nil
}
