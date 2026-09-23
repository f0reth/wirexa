package infrastructure

import (
	"errors"
	"path/filepath"

	"github.com/f0reth/Wirexa/internal/domain"
)

// WindowState はウィンドウのサイズ・位置・最大化状態を表す。
// Width/Height/X/Y は最大化していない通常時のバウンズを保持する。
type WindowState struct {
	Width     int  `json:"width"`
	Height    int  `json:"height"`
	X         int  `json:"x"`
	Y         int  `json:"y"`
	Maximised bool `json:"maximised"`
}

// LoadWindowState は指定パスの JSON を読み込む。読めなかった場合は ok=false を返し、
// 呼び出し側は既定サイズを使う。
//
// window-state.json は既定値から作り直せる再生成可能データ (application/store の分類表を参照)。
// application 層を経由しないため、同じ方針をここに直接書く。
//   - 破損: .corrupt へ退避してログに残す。退避に失敗しても終了時の保存で上書きしてよい
//   - 破損以外の読み込み失敗: 退避せずログに残す
//
// logger は nil を許容し、その場合ログ出力をスキップする。
func LoadWindowState(path string, logger domain.Logger) (WindowState, bool) {
	s, found, err := ReadJSONFile[WindowState](path)
	switch {
	case errors.Is(err, domain.ErrCorruptData):
		dest, qerr := QuarantineFile(path)
		if qerr != nil {
			logError(logger, "window_state: failed to quarantine corrupt file, it will be overwritten on exit",
				"file", filepath.Base(path), "error", err, "quarantineError", qerr)
		} else {
			logError(logger, "window_state: quarantined corrupt file, using default size",
				"file", filepath.Base(path), "quarantined", filepath.Base(dest), "error", err)
		}
		return WindowState{}, false
	case err != nil:
		logError(logger, "window_state: failed to read file, using default size", "file", filepath.Base(path), "error", err)
		return WindowState{}, false
	}
	return s, found
}

// SaveWindowState は現在のウィンドウ状態を JSON へ原子的に書き出す。
func SaveWindowState(path string, s WindowState) error {
	return WriteJSONFile(path, s, 0o600)
}

// logError は logger が設定されている場合だけエラーを記録する。
func logError(logger domain.Logger, msg string, args ...any) {
	if logger != nil {
		logger.Error(msg, args...)
	}
}
