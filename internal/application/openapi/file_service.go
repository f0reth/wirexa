// Package openapiapp は OpenAPI ファイル操作のユースケース層を提供する。
package openapiapp

import (
	"errors"
	"path/filepath"
	"slices"
	"sort"
	"sync"
	"time"

	cmn "github.com/f0reth/Wirexa/internal/domain"
	domain "github.com/f0reth/Wirexa/internal/domain/openapi"
)

// maxRecents は保持する最近使ったファイルの最大件数。
const maxRecents = 50

// コンパイル時に domain.FileUseCase を満たすことを検証
var _ domain.FileUseCase = (*FileService)(nil)

// FileService は OpenAPI ファイルの許可リストと recents を管理する。
//
// セキュリティ: Wails RPC は webview 上の JS から誰でも呼べるため、ReadFile /
// WriteFile が受理するパスは許可リストに登録済みのものに限定する。
// 許可リストへの登録は OpenSelected / SaveSelected (adapter のダイアログ処理が
// ダイアログの戻り値だけを渡す) と、起動時の recents seed でしか行われない。
// recents から外したパスは許可も取り消す。
type FileService struct {
	files   domain.FileAccess
	repo    domain.RecentRepository
	logger  cmn.Logger
	now     func() time.Time
	granted map[string]struct{}
	// recents は常に Order 昇順に並べて保持する。
	recents []domain.OpenAPIRecent
	// persist は recents をファイルへ保存してよいかを表す。
	// 元のファイルを退避できずに空で開始した場合は false にし、元のファイルを上書きしない。
	persist bool
	mu      sync.Mutex
}

// NewFileService は recents を読み込み、そのパスを許可リストへ seed した FileService を返す。
//
// recents の読み込みに失敗しても起動は止めず、空で開始する。ただし元のファイルを
// 次の保存で上書きして過去の recents を失わないよう、JSON が壊れている場合は退避してから
// 保存し、退避できない場合や読み込み自体に失敗した場合はこのセッションでは保存しない。
func NewFileService(repo domain.RecentRepository, files domain.FileAccess, logger cmn.Logger) *FileService {
	s := &FileService{
		files:   files,
		repo:    repo,
		logger:  logger,
		now:     time.Now,
		granted: make(map[string]struct{}),
		persist: true,
	}

	items, err := repo.Load()
	switch {
	case err == nil:
	case errors.Is(err, domain.ErrRecentsCorrupt):
		items = nil
		dest, qerr := repo.Quarantine()
		if qerr != nil {
			s.persist = false
			s.logger.Error("openapi_recents: failed to quarantine corrupt file, recents will not be saved this session",
				"error", err, "quarantineError", qerr)
		} else {
			s.logger.Error("openapi_recents: quarantined corrupt file, starting empty",
				"quarantined", filepath.Base(dest), "error", err)
		}
	default:
		items = nil
		s.persist = false
		s.logger.Error("openapi_recents: failed to read file, recents will not be saved this session", "error", err)
	}

	s.recents = slices.Clone(items)
	sortByOrder(s.recents)
	for _, it := range s.recents {
		s.granted[filepath.Clean(it.Path)] = struct{}{}
	}
	return s
}

// OpenSelected はダイアログで選ばれたパスを許可リストと recents へ登録し、正規化したパスを返す。
// recents の保存失敗は致命的でないため、ログに残してパスを返す。
func (s *FileService) OpenSelected(path string) string {
	cleaned := filepath.Clean(path)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.granted[cleaned] = struct{}{}
	s.addRecentLocked(cleaned)
	return cleaned
}

// SaveSelected はダイアログで選ばれたパスを許可リストへ登録して content を書き込み、
// 成功したら recents へ登録して正規化したパスを返す。書き込み失敗時も許可は残る。
func (s *FileService) SaveSelected(path, content string) (string, error) {
	cleaned := filepath.Clean(path)
	s.mu.Lock()
	s.granted[cleaned] = struct{}{}
	s.mu.Unlock()

	if err := s.files.WriteFile(cleaned, []byte(content)); err != nil {
		return "", err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.addRecentLocked(cleaned)
	return cleaned, nil
}

// ReadFile は許可リストに登録済みのパスの内容を文字列で返す。未許可なら拒否する。
func (s *FileService) ReadFile(path string) (string, error) {
	cleaned, ok := s.checkGranted(path)
	if !ok {
		return "", domain.ErrFileAccessDenied
	}
	data, err := s.files.ReadFile(cleaned)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// WriteFile は許可リストに登録済みのパスへ content を書き込む。未許可なら拒否する。
func (s *FileService) WriteFile(path, content string) error {
	cleaned, ok := s.checkGranted(path)
	if !ok {
		return domain.ErrFileAccessDenied
	}
	return s.files.WriteFile(cleaned, []byte(content))
}

// GetRecents は recents を Order 昇順で返す。
// RPC で null にならないよう、空でも nil ではないスライスを返す。
func (s *FileService) GetRecents() []domain.OpenAPIRecent {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]domain.OpenAPIRecent, len(s.recents))
	copy(out, s.recents)
	return out
}

// RemoveRecent は recents と許可リストからパスを削除する。
// 保存に失敗した場合は recents も許可も変更しない。
func (s *FileService) RemoveRecent(path string) error {
	cleaned := filepath.Clean(path)
	s.mu.Lock()
	defer s.mu.Unlock()

	next := make([]domain.OpenAPIRecent, 0, len(s.recents))
	for _, it := range s.recents {
		if it.Path != cleaned {
			next = append(next, it)
		}
	}
	reindex(next)
	if err := s.commitLocked(next); err != nil {
		return err
	}
	delete(s.granted, cleaned)
	return nil
}

// MoveRecent は recents 内でパスを index の位置へ並び替える。
// index は対象を取り除いた後の位置で、負または範囲外なら末尾へ移す。
func (s *FileService) MoveRecent(path string, index int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	from := slices.IndexFunc(s.recents, func(it domain.OpenAPIRecent) bool { return it.Path == path })
	if from == -1 {
		return nil
	}
	item := s.recents[from]
	next := slices.Delete(slices.Clone(s.recents), from, from+1)
	next = cmn.InsertAt(next, item, index)
	reindex(next)
	return s.commitLocked(next)
}

// checkGranted はパスを正規化し、許可リストに登録済みかを返す。
// ファイル I/O を lock の外で行えるよう、確認だけを lock 下で行う。
func (s *FileService) checkGranted(path string) (string, bool) {
	cleaned := filepath.Clean(path)
	s.mu.Lock()
	_, ok := s.granted[cleaned]
	s.mu.Unlock()
	return cleaned, ok
}

// addRecentLocked はパスを recents へ追加する。既存なら LastOpenedAt だけ更新する。
// 上限超過時は LastOpenedAt が古いものから間引く。保存失敗はログに残すだけにする。
// 呼び出し側で lock 済み。
func (s *FileService) addRecentLocked(path string) {
	now := s.now().UTC().Format(time.RFC3339)
	next := slices.Clone(s.recents)
	if i := slices.IndexFunc(next, func(it domain.OpenAPIRecent) bool { return it.Path == path }); i >= 0 {
		next[i].LastOpenedAt = now
	} else {
		next = append(next, domain.OpenAPIRecent{
			Path:         path,
			Name:         filepath.Base(path),
			LastOpenedAt: now,
		})
	}

	if len(next) > maxRecents {
		sort.SliceStable(next, func(i, j int) bool { return next[i].LastOpenedAt > next[j].LastOpenedAt })
		next = next[:maxRecents]
	}
	reindex(next)
	if err := s.commitLocked(next); err != nil {
		s.logger.Error("openapi_recents: failed to save recents", "error", err)
	}
}

// commitLocked は next を保存してから recents を差し替える (コピーオンライト)。
// 保存に失敗した場合は recents を変更しない。persist が false の間は保存せずに差し替える。
// 呼び出し側で lock 済み。
func (s *FileService) commitLocked(next []domain.OpenAPIRecent) error {
	if s.persist {
		if err := s.repo.Save(next); err != nil {
			return err
		}
	}
	s.recents = next
	return nil
}

// sortByOrder は items を Order 昇順に並べる。
func sortByOrder(items []domain.OpenAPIRecent) {
	sort.SliceStable(items, func(i, j int) bool { return items[i].Order < items[j].Order })
}

// reindex は現在の並び順に沿って Order を 0..n へ振り直す。
func reindex(items []domain.OpenAPIRecent) {
	for i := range items {
		items[i].Order = i
	}
}
