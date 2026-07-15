package infrastructure

import (
	"context"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// WindowManager はウィンドウのサイズ・位置・最大化状態の復元と保存を担う。
// Wails ランタイム呼び出しと window-state.json の永続化を仲介する。
type WindowManager struct {
	ctx       context.Context
	path      string
	minWidth  int
	minHeight int
	// winState は最後に把握した通常時 (非最大化) のウィンドウバウンズ。
	winState WindowState
}

// NewWindowManager は WindowManager を生成する。path は window-state.json の保存先、
// minWidth/minHeight はクランプ時の下限サイズ。
func NewWindowManager(ctx context.Context, path string, minWidth, minHeight int) *WindowManager {
	return &WindowManager{ctx: ctx, path: path, minWidth: minWidth, minHeight: minHeight}
}

// Restore は保存済みのウィンドウサイズ・位置・最大化状態を復元する。
// 保存値が無い / 不正な場合は options.App の既定サイズのままにする。
func (m *WindowManager) Restore() {
	ws, ok := LoadWindowState(m.path)
	if !ok || ws.Width <= 0 || ws.Height <= 0 {
		return
	}
	m.winState = ws
	ws = m.clamp(ws)
	runtime.WindowSetSize(m.ctx, ws.Width, ws.Height)
	runtime.WindowSetPosition(m.ctx, ws.X, ws.Y)
	if ws.Maximised {
		runtime.WindowMaximise(m.ctx)
	}
}

// clamp は画面外にウィンドウが出ないようサイズ・位置を補正する。
// 注: Wails v2.12 の Screen はスクリーン原点を公開しないため、位置クランプは
// プライマリスクリーン基準の best-effort となる (マルチモニタの負座標はプライマリ側へ寄る)。
// 画面外化を保守的に防ぐことを優先する。
func (m *WindowManager) clamp(ws WindowState) WindowState {
	screens, err := runtime.ScreenGetAll(m.ctx)
	if err != nil || len(screens) == 0 {
		return ws
	}
	sw, sh := 0, 0
	for _, s := range screens {
		if s.IsPrimary {
			sw, sh = s.Size.Width, s.Size.Height
			break
		}
	}
	if sw == 0 || sh == 0 {
		sw, sh = screens[0].Size.Width, screens[0].Size.Height
	}
	if sw <= 0 || sh <= 0 {
		return ws
	}

	if ws.Width > sw {
		ws.Width = sw
	}
	if ws.Height > sh {
		ws.Height = sh
	}
	if ws.Width < m.minWidth {
		ws.Width = m.minWidth
	}
	if ws.Height < m.minHeight {
		ws.Height = m.minHeight
	}

	maxX, maxY := sw-ws.Width, sh-ws.Height
	if ws.X < 0 {
		ws.X = 0
	}
	if ws.Y < 0 {
		ws.Y = 0
	}
	if ws.X > maxX {
		ws.X = maxX
	}
	if ws.Y > maxY {
		ws.Y = maxY
	}
	return ws
}

// Save は現在のウィンドウ状態を永続化する。ウィンドウが生存している
// beforeClose 内から呼ぶ。最大化中は通常時バウンズ (winState) を維持する。
func (m *WindowManager) Save() {
	if m.ctx == nil || m.path == "" {
		return
	}
	maximised := runtime.WindowIsMaximised(m.ctx)
	if !maximised {
		w, h := runtime.WindowGetSize(m.ctx)
		x, y := runtime.WindowGetPosition(m.ctx)
		if w > 0 && h > 0 {
			m.winState.Width, m.winState.Height = w, h
			m.winState.X, m.winState.Y = x, y
		}
	}
	m.winState.Maximised = maximised
	_ = SaveWindowState(m.path, m.winState) //nolint:errcheck // best-effort persistence
}
