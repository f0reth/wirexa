package httpinfra

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	domain "github.com/f0reth/Wirexa/internal/domain/http"
)

// response 一時ファイルの上限。frontend が DiscardResponseBody を呼ぶことを前提にせず、
// backend 側でディスク使用量を有界にする。値は安全側の内部定数から始める。
const (
	maxResponseFiles            = 8
	maxResponseTotalBytes int64 = 2 << 30 // 2 GiB
	maxConcurrentSpills         = 2
	responseReadyTTL            = 60 * time.Minute

	// responseSessionDirPrefix は Wirexa 専用 session directory の接頭辞。
	// 起動時 sweep はこの接頭辞のディレクトリだけを対象にする。
	responseSessionDirPrefix = "wirexa-http-"
	// legacyResponseFilePrefix は session directory 導入前の flat な一時ファイルの接頭辞。
	legacyResponseFilePrefix = "wirexa-response-"
)

var (
	_ domain.ResponseBodyStore = (*ResponseStore)(nil)
	_ domain.ResponseBodyLease = (*responseLease)(nil)

	// errSpillWrite は一時ファイルへの書き込み失敗。OS エラーはパスを含むため連結しない。
	errSpillWrite = errors.New("failed to store response body")
)

type responseState int

const (
	// stateRunning はリクエスト実行中。一時ファイルはまだ公開可能でない。
	stateRunning responseState = iota
	// stateReady は保存または破棄が可能。
	stateReady
	// stateSaving は保存ダイアログまたはコピー処理中。破棄・同じ ID の送信・TTL 回収を拒否する。
	stateSaving
)

type responseEntry struct {
	readyAt     time.Time
	path        string
	contentType string
	// reserved は総容量に計上中のバイト数。spill 中は絶対上限分、完了後は実サイズ。
	reserved int64
	state    responseState
	spilling bool
}

// hasFile はエントリが件数・総容量の上限に計上される一時ファイルを持つかを返す。
func (e *responseEntry) hasFile() bool {
	return e.spilling || e.path != ""
}

// responseFileOps は一時ファイルの操作。テストで copy / remove の失敗を注入するために差し替える。
type responseFileOps struct {
	copyFile func(src, dst string) error
	remove   func(path string) error
}

var defaultResponseFileOps = responseFileOps{copyFile: copyResponseFile, remove: os.Remove}

// ResponseStore は切り詰められたレスポンスの一時ファイルを execution ID で追跡する。
// 状態遷移 (running → ready ⇄ saving → 削除) を mutex 下で管理し、
// 同じ ID の送信・保存・破棄・終了処理を直列化する。
type ResponseStore struct {
	now     func() time.Time
	entries map[string]*responseEntry
	ops     responseFileOps
	// dir は 0700 の session directory。最初の spill で作成する。
	dir      string
	total    int64
	files    int
	spilling int
	mu       sync.Mutex
	closed   bool
}

// NewResponseStore は ResponseStore を生成する。
func NewResponseStore() *ResponseStore {
	return &ResponseStore{
		now:     time.Now,
		entries: make(map[string]*responseEntry),
		ops:     defaultResponseFileOps,
	}
}

// Begin は execution ID を running として予約する。
// 同じ ID のエントリが running / ready / saving のいずれかにある間は拒否し、暗黙に置換しない。
func (s *ResponseStore) Begin(executionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return domain.ErrResponseUnavailable
	}
	s.evictExpiredLocked()
	if _, ok := s.entries[executionID]; ok {
		return domain.ErrResponseBusy
	}
	s.entries[executionID] = &responseEntry{state: stateRunning}
	return nil
}

// Finish はリクエスト終了時に呼ぶ。一時ファイルを公開しなかった (running のままの) 予約を解放する。
func (s *ResponseStore) Finish(executionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e, ok := s.entries[executionID]; ok && e.state == stateRunning && !e.spilling {
		delete(s.entries, executionID)
	}
}

// Spill は running の execution ID に一時ファイルを割り当てて write で書き込み、ready にする。
// 件数・総容量・同時 spill の上限は書き込み前に検査し、総容量は reserve バイトを先に予約して
// 書き込み完了時に実サイズへ精算する。write は書き込んだバイト数を返し、reserve を超えてはならない。
func (s *ResponseStore) Spill(executionID string, reserve int64, contentType string, write func(f *os.File) (int64, error)) error {
	dir, err := s.startSpill(executionID, reserve)
	if err != nil {
		return err
	}

	f, err := os.CreateTemp(dir, "response-*")
	if err != nil {
		s.abortSpill(executionID, "")
		return errSpillWrite
	}
	name := f.Name()
	written, werr := write(f)
	if cerr := f.Close(); werr == nil && cerr != nil {
		werr = errSpillWrite
	}
	if werr != nil {
		s.abortSpill(executionID, name)
		return werr
	}
	return s.commitSpill(executionID, name, written, contentType)
}

func (s *ResponseStore) startSpill(executionID string, reserve int64) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return "", domain.ErrResponseUnavailable
	}
	s.evictExpiredLocked()
	e, ok := s.entries[executionID]
	if !ok || e.state != stateRunning || e.hasFile() {
		return "", domain.ErrResponseBusy
	}
	if s.files+1 > maxResponseFiles || s.total+reserve > maxResponseTotalBytes || s.spilling+1 > maxConcurrentSpills {
		return "", domain.ErrResponseStorageLimit
	}
	if s.dir == "" {
		// MkdirTemp は 0700 で作成する。
		dir, err := os.MkdirTemp("", responseSessionDirPrefix+"*")
		if err != nil {
			return "", errSpillWrite
		}
		s.dir = dir
	}
	e.spilling = true
	e.reserved = reserve
	s.total += reserve
	s.files++
	s.spilling++
	return s.dir, nil
}

// abortSpill は書き込みに失敗した spill の予約を解放し、作りかけのファイルを削除する。
func (s *ResponseStore) abortSpill(executionID, path string) {
	if path != "" {
		_ = s.ops.remove(path) //nolint:errcheck // best-effort cleanup; 残れば次回起動時 sweep で回収する
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.spilling--
	e, ok := s.entries[executionID]
	if !ok || !e.spilling {
		// Cleanup で追跡から外された後。計上は Cleanup 側で解放済み。
		return
	}
	e.spilling = false
	s.total -= e.reserved
	s.files--
	e.reserved = 0
}

func (s *ResponseStore) commitSpill(executionID, path string, written int64, contentType string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.spilling--
	e, ok := s.entries[executionID]
	if s.closed || !ok || !e.spilling {
		// 書き込み中に終了処理が走った。公開せずに回収する。
		_ = s.ops.remove(path) //nolint:errcheck // best-effort cleanup; 残れば次回起動時 sweep で回収する
		return domain.ErrResponseUnavailable
	}
	e.spilling = false
	e.path = path
	e.contentType = contentType
	s.total += written - e.reserved
	e.reserved = written
	e.state = stateReady
	e.readyAt = s.now()
	return nil
}

// AcquireSave は ready の一時ファイルを saving にして保存用 lease を返す。
// 未追跡・保存済み・期限切れの ID は ErrResponseUnavailable、running / saving は ErrResponseBusy。
// どちらも保存ダイアログを開く前に判定できるよう、lease 取得を先に行う。
func (s *ResponseStore) AcquireSave(executionID string) (domain.ResponseBodyLease, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, domain.ErrResponseUnavailable
	}
	s.evictExpiredLocked()
	e, ok := s.entries[executionID]
	if !ok {
		return nil, domain.ErrResponseUnavailable
	}
	if e.state != stateReady {
		return nil, domain.ErrResponseBusy
	}
	e.state = stateSaving
	return &responseLease{store: s, id: executionID, path: e.path, contentType: e.contentType}, nil
}

// Discard は ready の一時ファイルを削除して追跡を終える。saving / running は拒否する。
func (s *ResponseStore) Discard(executionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.entries[executionID]
	if !ok {
		return domain.ErrResponseUnavailable
	}
	if e.state != stateReady {
		return domain.ErrResponseBusy
	}
	if !s.removeLocked(e.path) {
		return domain.ErrDiscardResponseFailed
	}
	s.dropLocked(executionID)
	return nil
}

// Cleanup は新規操作を停止し、saving 以外の一時ファイルを回収する。
// saving の一時ファイルは保存処理と競合して削除せず、session directory ごと次回起動時 sweep に任せる。
// 実行中リクエストのキャンセルと待機は呼び出し側 (HTTPRequestService.Shutdown) が先に行う。
func (s *ResponseStore) Cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	saving := false
	for id, e := range s.entries {
		if e.state == stateSaving {
			saving = true
			continue
		}
		if e.path != "" {
			s.removeLocked(e.path)
		}
		s.dropLocked(id)
	}
	if !saving && s.dir != "" {
		_ = os.RemoveAll(s.dir) //nolint:errcheck // best-effort cleanup; 残れば次回起動時 sweep で回収する
	}
}

// evictExpiredLocked は TTL を過ぎた ready の一時ファイルを回収する。running / saving は対象外。
// 専用 goroutine を持たず、store の各操作の冒頭で遅延評価する (件数・容量上限があるため
// 操作が無い間に放置されても使用量は有界)。
func (s *ResponseStore) evictExpiredLocked() {
	now := s.now()
	for id, e := range s.entries {
		if e.state == stateReady && now.Sub(e.readyAt) >= responseReadyTTL && s.removeLocked(e.path) {
			s.dropLocked(id)
		}
	}
}

// removeLocked は一時ファイルを削除し、ファイルが無くなったかを返す。
func (s *ResponseStore) removeLocked(path string) bool {
	err := s.ops.remove(path)
	return err == nil || errors.Is(err, os.ErrNotExist)
}

// dropLocked はエントリを追跡から外し、上限の計上を解放する。
func (s *ResponseStore) dropLocked(executionID string) {
	e, ok := s.entries[executionID]
	if !ok {
		return
	}
	if e.hasFile() {
		s.total -= e.reserved
		s.files--
	}
	delete(s.entries, executionID)
}

// responseLease は saving 状態のエントリへの保存用参照。
type responseLease struct {
	store       *ResponseStore
	id          string
	path        string
	contentType string
}

func (l *responseLease) ContentType() string {
	return l.contentType
}

func (l *responseLease) SaveTo(dst string) error {
	s := l.store
	if err := s.ops.copyFile(l.path, dst); err != nil {
		l.restore()
		return domain.ErrSaveResponseFailed
	}
	if err := s.ops.remove(l.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		l.restore()
		return domain.ErrSaveResponseFailed
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dropLocked(l.id)
	return nil
}

func (l *responseLease) Release() {
	l.restore()
}

// restore は saving を ready へ戻す。元ファイルを失っている、または終了処理後の場合は追跡から外す。
func (l *responseLease) restore() {
	s := l.store
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.entries[l.id]
	if !ok || e.state != stateSaving {
		return
	}
	if s.closed {
		s.removeLocked(e.path)
		s.dropLocked(l.id)
		return
	}
	if _, err := os.Stat(e.path); err != nil {
		s.dropLocked(l.id)
		return
	}
	e.state = stateReady
}

// copyResponseFile は一時ファイルを保存先へコピーする。flush と close の失敗も失敗として扱う。
func copyResponseFile(src, dst string) error {
	in, err := os.Open(src) //nolint:gosec // G304: src is a temp file created and tracked by ResponseStore.
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }() //nolint:errcheck // best-effort cleanup

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600) //nolint:gosec // G304: dst comes from the OS save dialog.
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close() //nolint:errcheck // the copy error is reported instead
		return err
	}
	if err := out.Sync(); err != nil {
		_ = out.Close() //nolint:errcheck // the sync error is reported instead
		return err
	}
	return out.Close()
}

// SweepStaleTempFiles は前回セッションで残った session directory と、
// session directory 導入前の flat な一時ファイル (wirexa-response-*) を削除する。
// 単一インスタンスロックにより同時起動が無いため、接頭辞に一致するものを安全に削除できる。
func SweepStaleTempFiles() {
	tmp := os.TempDir()
	if dirs, err := filepath.Glob(filepath.Join(tmp, responseSessionDirPrefix+"*")); err == nil {
		for _, d := range dirs {
			_ = os.RemoveAll(d) //nolint:errcheck // best-effort cleanup
		}
	}
	if files, err := filepath.Glob(filepath.Join(tmp, legacyResponseFilePrefix+"*")); err == nil {
		for _, f := range files {
			_ = os.Remove(f) //nolint:errcheck // best-effort cleanup
		}
	}
}
