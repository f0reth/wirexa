package httpinfra

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
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
	_ domain.SelectedFileReader = (*FileRegistry)(nil)

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

// ReadSelectedFile は token を解決してファイルを読む。アクセス判定は読み込みの直前に行う。
// 空・未登録の token は ErrFileAccessDenied、登録後に読めなくなったファイルは
// ErrSelectedFileUnavailable を返す。どちらもパスを含む OS エラーを連結しない。
func (r *FileRegistry) ReadSelectedFile(token string) (domain.SelectedFileContent, error) {
	if token == "" {
		return domain.SelectedFileContent{}, domain.ErrFileAccessDenied
	}
	r.mu.RLock()
	e, ok := r.byToken[token]
	r.mu.RUnlock()
	if !ok {
		return domain.SelectedFileContent{}, domain.ErrFileAccessDenied
	}
	// e.path はファイルダイアログの戻り値で、登録済み token からしか引けない。
	data, err := os.ReadFile(e.path)
	if err != nil {
		return domain.SelectedFileContent{}, domain.ErrSelectedFileUnavailable
	}
	return domain.SelectedFileContent{Name: e.name, ContentType: e.contentType, Data: data}, nil
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

// resolveFile は file 参照を registry で解決して読み込む。token を持たない参照のうち、
// 保存済み・移行済み (再選択待ち) のものは拒否し、何も選ばれていなければ ok=false を返す。
// パスを受け取る経路は無く、未登録の token ではファイルを開かない。
func resolveFile(files domain.SelectedFileReader, ref domain.FileReference) (file domain.SelectedFileContent, ok bool, err error) {
	if ref.Token == "" {
		if ref.Name != "" || ref.NeedsReselect {
			return domain.SelectedFileContent{}, false, domain.ErrFileAccessDenied
		}
		return domain.SelectedFileContent{}, false, nil
	}
	if files == nil {
		return domain.SelectedFileContent{}, false, domain.ErrFileAccessDenied
	}
	file, err = files.ReadSelectedFile(ref.Token)
	if err != nil {
		return domain.SelectedFileContent{}, false, err
	}
	return file, true, nil
}
