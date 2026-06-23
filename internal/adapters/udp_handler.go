package adapters

import (
	udpdomain "github.com/f0reth/Wirexa/internal/domain/udp"
)

// UdpHandler は Wails RPC アダプターとして UDP ユースケースを公開する。
type UdpHandler struct {
	sendSvc   udpdomain.SendUseCase
	targetSvc udpdomain.TargetUseCase
	listenSvc udpdomain.ListenUseCase
}

// SetupUdpHandler は既存の UdpHandler インスタンスにサービスを注入する。
func SetupUdpHandler(h *UdpHandler, sendSvc udpdomain.SendUseCase, targetSvc udpdomain.TargetUseCase, listenSvc udpdomain.ListenUseCase) {
	h.sendSvc = sendSvc
	h.targetSvc = targetSvc
	h.listenSvc = listenSvc
}

// Send は UDP パケットを送信する。
func (h *UdpHandler) Send(req udpdomain.UdpSendRequest) (udpdomain.UdpSendResult, error) {
	return h.sendSvc.Send(req)
}

// GetTargets は全ターゲットを返す。
func (h *UdpHandler) GetTargets() []udpdomain.UdpTarget {
	return h.targetSvc.GetTargets()
}

// SaveTarget はターゲットを保存する。
func (h *UdpHandler) SaveTarget(target udpdomain.UdpTarget) (udpdomain.UdpTarget, error) {
	return h.targetSvc.SaveTarget(target)
}

// DeleteTarget は ID でターゲットを削除する。
func (h *UdpHandler) DeleteTarget(id string) error {
	return h.targetSvc.DeleteTarget(id)
}

// StartListen は指定ポートで UDP リスニングを開始する。
// encoding は名前付き文字列型をそのまま引数に取ると Wails が models.ts に型定義を生成せず
// バインディングが壊れるため、string で受けてここでドメイン型へ変換する。
func (h *UdpHandler) StartListen(port int, encoding string) (udpdomain.UdpListenSession, error) {
	return h.listenSvc.StartListen(port, udpdomain.PayloadEncoding(encoding))
}

// StopListen は指定セッションのリスニングを停止する。
func (h *UdpHandler) StopListen(sessionID string) error {
	return h.listenSvc.StopListen(sessionID)
}

// GetListeners はアクティブなリスニングセッション一覧を返す。
func (h *UdpHandler) GetListeners() []udpdomain.UdpListenSession {
	return h.listenSvc.GetListeners()
}

// Shutdown は全リスニングセッションを停止する。
func (h *UdpHandler) Shutdown() {
	h.listenSvc.StopAll()
}
