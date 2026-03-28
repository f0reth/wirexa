// Package udpinfra は UDP インフラストラクチャ層を提供する。
package udpinfra

import (
	"context"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	domain "github.com/f0reth/Wirexa/internal/domain/udp"
)

// コンパイル時に domain.UdpEmitter を満たすことを検証。
var _ domain.UdpEmitter = (*WailsEmitter)(nil)

// WailsEmitter は Wails ランタイムを使ってフロントエンドへイベントを送信する。
type WailsEmitter struct {
	ctx context.Context
}

// NewWailsEmitter は WailsEmitter を生成する。
func NewWailsEmitter(ctx context.Context) *WailsEmitter {
	return &WailsEmitter{ctx: ctx}
}

// Emit はフロントエンドへイベントを送信する。
func (e *WailsEmitter) Emit(event string, data any) {
	runtime.EventsEmit(e.ctx, event, data)
}
