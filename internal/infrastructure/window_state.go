package infrastructure

import (
	"encoding/json"
	"os"
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

// LoadWindowState は指定パスの JSON を読み込む。
// 読み込み・パースに失敗した場合は ok=false を返す (呼び出し側は既定値を使う)。
func LoadWindowState(path string) (WindowState, bool) {
	data, err := os.ReadFile(path) //nolint:gosec // path is app-generated config file
	if err != nil {
		return WindowState{}, false
	}
	var s WindowState
	if err := json.Unmarshal(data, &s); err != nil {
		return WindowState{}, false
	}
	return s, true
}

// SaveWindowState は現在のウィンドウ状態を JSON へ原子的に書き出す。
func SaveWindowState(path string, s WindowState) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return AtomicWriteFile(path, data, 0o600)
}
