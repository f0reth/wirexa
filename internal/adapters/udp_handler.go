package adapters

import (
	udpdomain "github.com/f0reth/Wirexa/internal/domain/udp"
)

// UDPHandler は Wails RPC アダプターとして UDP ユースケースを公開する。
type UDPHandler struct {
	sendSvc   udpdomain.SendUseCase
	targetSvc udpdomain.TargetUseCase
	listenSvc udpdomain.ListenUseCase
}

// SetupUDPHandler は既存の UDPHandler インスタンスにサービスを注入する。
func SetupUDPHandler(h *UDPHandler, sendSvc udpdomain.SendUseCase, targetSvc udpdomain.TargetUseCase, listenSvc udpdomain.ListenUseCase) {
	h.sendSvc = sendSvc
	h.targetSvc = targetSvc
	h.listenSvc = listenSvc
}

// Send は UDP パケットを送信する。
func (h *UDPHandler) Send(req udpdomain.UDPSendRequest) (udpdomain.UDPSendResult, error) {
	return h.sendSvc.Send(req)
}

// GetTargets は全ターゲットを返す。
func (h *UDPHandler) GetTargets() []udpdomain.UDPTarget {
	return h.targetSvc.GetTargets()
}

// SaveTarget はターゲットを保存する。
func (h *UDPHandler) SaveTarget(target udpdomain.UDPTarget) (udpdomain.UDPTarget, error) {
	return h.targetSvc.SaveTarget(target)
}

// DeleteTarget は ID でターゲットを削除する。
func (h *UDPHandler) DeleteTarget(id string) error {
	return h.targetSvc.DeleteTarget(id)
}

// StartListen は指定ポートで UDP リスニングを開始する。
// encoding は名前付き文字列型をそのまま引数に取ると Wails が models.ts に型定義を生成せず
// バインディングが壊れるため、string で受けてここでドメイン型へ変換する。
func (h *UDPHandler) StartListen(port int, encoding string) (udpdomain.UDPListenSession, error) {
	return h.listenSvc.StartListen(port, udpdomain.PayloadEncoding(encoding))
}

// StopListen は指定セッションのリスニングを停止する。
func (h *UDPHandler) StopListen(sessionID string) error {
	return h.listenSvc.StopListen(sessionID)
}

// GetListeners はアクティブなリスニングセッション一覧を返す。
func (h *UDPHandler) GetListeners() []udpdomain.UDPListenSession {
	return h.listenSvc.GetListeners()
}

// Shutdown は全リスニングセッションを停止する。
func (h *UDPHandler) Shutdown() {
	h.listenSvc.StopAll()
}
