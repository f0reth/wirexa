package adapters

import (
	domain "github.com/f0reth/Wirexa/internal/domain/udp"
)

// UdpHandler は Wails RPC アダプターとして UDP ユースケースを公開する。
type UdpHandler struct {
	sendSvc   domain.SendUseCase
	targetSvc domain.TargetUseCase
	listenSvc domain.ListenUseCase
}

// SetupUdpHandler は既存の UdpHandler インスタンスにサービスを注入する。
func SetupUdpHandler(h *UdpHandler, sendSvc domain.SendUseCase, targetSvc domain.TargetUseCase, listenSvc domain.ListenUseCase) {
	h.sendSvc = sendSvc
	h.targetSvc = targetSvc
	h.listenSvc = listenSvc
}

// Send は UDP パケットを送信する。
func (h *UdpHandler) Send(req domain.UdpSendRequest) (domain.UdpSendResult, error) {
	return h.sendSvc.Send(req)
}

// GetTargets は全ターゲットを返す。
func (h *UdpHandler) GetTargets() []domain.UdpTarget {
	return h.targetSvc.GetTargets()
}

// SaveTarget はターゲットを保存する。
func (h *UdpHandler) SaveTarget(target domain.UdpTarget) (domain.UdpTarget, error) {
	return h.targetSvc.SaveTarget(target)
}

// DeleteTarget は ID でターゲットを削除する。
func (h *UdpHandler) DeleteTarget(id string) error {
	return h.targetSvc.DeleteTarget(id)
}

// StartListen は指定ポートで UDP リスニングを開始する。
func (h *UdpHandler) StartListen(port int, encoding string) (domain.UdpListenSession, error) {
	return h.listenSvc.StartListen(port, domain.PayloadEncoding(encoding))
}

// StopListen は指定セッションのリスニングを停止する。
func (h *UdpHandler) StopListen(sessionID string) error {
	return h.listenSvc.StopListen(sessionID)
}

// GetListeners はアクティブなリスニングセッション一覧を返す。
func (h *UdpHandler) GetListeners() []domain.UdpListenSession {
	return h.listenSvc.GetListeners()
}

// Shutdown は全リスニングセッションを停止する。
func (h *UdpHandler) Shutdown() {
	h.listenSvc.StopAll()
}
