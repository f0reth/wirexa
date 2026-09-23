package httpinfra

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	domain "github.com/f0reth/Wirexa/internal/domain/http"
)

// maxSelectedFiles は request file registry の登録上限。悪意ある frontend による
// 無制限な増加を防ぐ。参照状態を誤判定して有効な token を追い出すより、
// 上限到達後の新規選択を明示的に拒否する。
const maxSelectedFiles = 256

var (
	_ domain.SelectedFileOpener = (*FileRegistry)(nil)
	_ domain.SelectedFileHandle = (*selectedFileHandle)(nil)

	// errFileToken は token の生成失敗。
	errFileToken = errors.New("failed to issue file token")
)

type selectedFileEntry struct {
	registeredAt time.Time
	path         string
	name         string
	contentType  string
}

// FileRegistry はファイルダイアログで選択されたファイルを session token で参照させる。
//
// token の発行源はダイアログの戻り値だけで、Register を呼ぶのは HTTPHandler のダイアログ処理に
// 限る (RPC からパスと token の組を登録する経路は設けない)。token は現在のアプリセッションでだけ
// 有効で、永続化しない。メモリには token とメタデータだけを持ち、ファイルの内容やハンドルは持たない。
type FileRegistry struct {
	now     func() time.Time
	byToken map[string]*selectedFileEntry
	byPath  map[string]string
	mu      sync.RWMutex
}

// NewFileRegistry は空の FileRegistry を生成する。
func NewFileRegistry() *FileRegistry {
	return &FileRegistry{
		now:     time.Now,
		byToken: make(map[string]*selectedFileEntry),
		byPath:  make(map[string]string),
	}
}

// Register はダイアログで選択されたパスを登録して token を返す。
// 同じパスの再選択には同じ token を返し、選択の繰り返しで上限を使い切らないようにする。
func (r *FileRegistry) Register(path string) (domain.SelectedFile, error) {
	cleaned := filepath.Clean(path)
	r.mu.Lock()
	defer r.mu.Unlock()
	if token, ok := r.byPath[cleaned]; ok {
		return r.byToken[token].selected(token), nil
	}
	if len(r.byToken) >= maxSelectedFiles {
		return domain.SelectedFile{}, domain.ErrFileSelectionLimit
	}
	token, err := newFileToken()
	if err != nil {
		return domain.SelectedFile{}, errFileToken
	}
	e := &selectedFileEntry{
		registeredAt: r.now(),
		path:         cleaned,
		name:         filepath.Base(cleaned),
		contentType:  domain.GuessFileContentType(cleaned),
	}
	r.byToken[token] = e
	r.byPath[cleaned] = token
	return e.selected(token), nil
}

// OpenSelectedFile は token を解決してファイルを開く。アクセス判定は開く直前に行う。
// 空・未登録の token は ErrFileAccessDenied、登録後に開けなくなったファイルや通常ファイルでない
// ものは ErrSelectedFileUnavailable を返す。どちらもパスを含む OS エラーを連結しない。
// 返すハンドルは開いた時点のサイズと更新時刻を基準として持ち、呼び出し側が Close する。
func (r *FileRegistry) OpenSelectedFile(token string) (domain.OpenedSelectedFile, error) {
	if token == "" {
		return domain.OpenedSelectedFile{}, domain.ErrFileAccessDenied
	}
	r.mu.RLock()
	e, ok := r.byToken[token]
	r.mu.RUnlock()
	if !ok {
		return domain.OpenedSelectedFile{}, domain.ErrFileAccessDenied
	}
	// e.path はファイルダイアログの戻り値で、登録済み token からしか引けない。
	f, err := os.Open(e.path)
	if err != nil {
		return domain.OpenedSelectedFile{}, domain.ErrSelectedFileUnavailable
	}
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		_ = f.Close() //nolint:errcheck // 読み取り専用ハンドルの後始末
		return domain.OpenedSelectedFile{}, domain.ErrSelectedFileUnavailable
	}
	return domain.OpenedSelectedFile{
		File:        newSelectedFileHandle(f, info),
		Name:        e.name,
		ContentType: e.contentType,
		Size:        info.Size(),
	}, nil
}

func (e *selectedFileEntry) selected(token string) domain.SelectedFile {
	return domain.SelectedFile{Token: token, Name: e.name, ContentType: e.contentType}
}

// newFileToken は crypto/rand の 16 byte (128 bit) を hex 符号化した token を返す。
func newFileToken() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

// selectedFileSource は selectedFileHandle が包むファイル。*os.File が満たし、テストで差し替える。
type selectedFileSource interface {
	io.ReaderAt
	io.Closer
	Stat() (os.FileInfo, error)
}

// selectedFileHandle は domain.SelectedFileHandle の実装。開いた時点のサイズと更新時刻を基準に持つ。
// OS のエラーはパスを含むため、io.EOF 以外はすべて ErrSelectedFileUnavailable に置き換え、
// *os.PathError を外へ出さない。
type selectedFileHandle struct {
	modTime time.Time
	f       selectedFileSource
	size    int64
}

func newSelectedFileHandle(f selectedFileSource, info os.FileInfo) *selectedFileHandle {
	return &selectedFileHandle{f: f, size: info.Size(), modTime: info.ModTime()}
}

// ReadAt は off から読む。io.EOF はそのまま返し、それ以外の失敗は ErrSelectedFileUnavailable にする。
func (h *selectedFileHandle) ReadAt(p []byte, off int64) (int, error) {
	n, err := h.f.ReadAt(p, off)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return n, io.EOF
		}
		return n, domain.ErrSelectedFileUnavailable
	}
	return n, nil
}

// CheckUnchanged は開き直さずに Stat し直し、サイズか更新時刻が基準と違えば ErrSelectedFileChanged を返す。
func (h *selectedFileHandle) CheckUnchanged() error {
	info, err := h.f.Stat()
	if err != nil {
		return domain.ErrSelectedFileUnavailable
	}
	if info.Size() != h.size || !info.ModTime().Equal(h.modTime) {
		return domain.ErrSelectedFileChanged
	}
	return nil
}

// Close はハンドルを閉じる。失敗もパスを含めない。
func (h *selectedFileHandle) Close() error {
	if err := h.f.Close(); err != nil {
		return domain.ErrSelectedFileUnavailable
	}
	return nil
}

// openFile は file 参照を registry で解決して開く。token を持たない参照のうち、
// 保存済み・移行済み (再選択待ち) のものは拒否し、何も選ばれていなければ ok=false を返す。
// パスを受け取る経路は無く、未登録の token ではファイルを開かない。ok=true のときは呼び出し側が
// file.File を閉じる。
func openFile(files domain.SelectedFileOpener, ref domain.FileReference) (file domain.OpenedSelectedFile, ok bool, err error) {
	if ref.Token == "" {
		if ref.Name != "" || ref.NeedsReselect {
			return domain.OpenedSelectedFile{}, false, domain.ErrFileAccessDenied
		}
		return domain.OpenedSelectedFile{}, false, nil
	}
	if files == nil {
		return domain.OpenedSelectedFile{}, false, domain.ErrFileAccessDenied
	}
	file, err = files.OpenSelectedFile(ref.Token)
	if err != nil {
		return domain.OpenedSelectedFile{}, false, err
	}
	return file, true, nil
}

// readSelectedFile は開いたファイルを開いた時点のサイズ分だけ読み、ハンドルを閉じる。
func readSelectedFile(file domain.OpenedSelectedFile) ([]byte, error) {
	defer func() { _ = file.File.Close() }() //nolint:errcheck // 読み取り専用ハンドルの後始末
	data, err := io.ReadAll(io.NewSectionReader(file.File, 0, file.Size))
	if err != nil {
		return nil, err
	}
	return data, nil
}
