package httpinfra

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/textproto"
	"strings"

	domain "github.com/f0reth/Wirexa/internal/domain/http"
)

// quoteEscaper は Content-Disposition のパラメータ値をエスケープする。
// mime/multipart の同等品が非公開のため同じ変換をここに置く。
var quoteEscaper = strings.NewReplacer("\\", "\\\\", `"`, "\\\"")

// buildMultipartBody は form-data の行から multipart ボディと Content-Type を組み立てる。
// 返す Content-Type は採番済みの boundary を含むため、呼び出し側で必ず優先させる。
// file 行は files で token を解決したファイルだけを開き、内容は読まずにファイル区間として差し込む。
// 成功したら返したボディの release を呼び出し側が呼ぶ。失敗したら開いたファイルはすべて閉じる。
func buildMultipartBody(rows []domain.FormRow, files domain.SelectedFileOpener) (*bodySegments, string, error) {
	body := &bodySegments{}
	// multipart.Writer は boundary とパートヘッダを下位の writer に書くだけなので、
	// バイト区間へ追記する body に向ければ、従来と同じバイト列を区間に分けて得られる。
	mw := multipart.NewWriter(body)
	for i := range rows {
		r := &rows[i]
		if !r.Enabled || r.Key == "" {
			continue
		}
		if err := writeFormRow(mw, body, r, files); err != nil {
			body.release()
			return nil, "", err
		}
	}
	if err := mw.Close(); err != nil {
		body.release()
		return nil, "", fmt.Errorf("failed to close multipart writer: %w", err)
	}
	return body, mw.FormDataContentType(), nil
}

// writeFormRow は 1 行をパートとして書き出す。
func writeFormRow(mw *multipart.Writer, body *bodySegments, r *domain.FormRow, files domain.SelectedFileOpener) error {
	switch r.EffectiveKind() {
	case domain.FormRowKindFile:
		// 送信元はダイアログで選ばれ token で解決できるファイルだけで、パスを受け取る経路は無い。
		file, ok, err := openFile(files, r.File)
		if err != nil {
			return err
		}
		// 何も選ばれていない行はまだ書きかけなので、無効行と同じく黙って飛ばす。
		if !ok {
			return nil
		}
		// filename と自動 Content-Type は registry の値を正とし、frontend の値は使わない。
		// ユーザーが明示したパートの Content-Type だけは上書き値として扱う。
		contentType := r.ContentType
		if contentType == "" {
			contentType = file.ContentType
		}
		if _, err = createPart(mw, r.Key, file.Name, contentType); err != nil {
			_ = file.File.Close() //nolint:errcheck // 読み取り専用ハンドルの後始末
			return err
		}
		// パートの本文は下位の writer にそのまま書かれるだけなので、同じ位置にファイル区間を挟む。
		body.appendFile(file)
		return nil
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
func writePart(mw *multipart.Writer, name, filename, contentType string, data []byte) error {
	part, err := createPart(mw, name, filename, contentType)
	if err != nil {
		return err
	}
	if _, err = part.Write(data); err != nil {
		return fmt.Errorf("failed to write form part: %w", err)
	}
	return nil
}

// createPart は Content-Type 付きのパートヘッダを書く。
// multipart.CreateFormFile は Content-Type を application/octet-stream に固定するため使えない。
func createPart(mw *multipart.Writer, name, filename, contentType string) (io.Writer, error) {
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
		return nil, fmt.Errorf("failed to create form part: %w", err)
	}
	return part, nil
}
