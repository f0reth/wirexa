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
func (h *UdpHandler) Send(req UdpSendRequest) (UdpSendResult, error) {
	res, err := h.sendSvc.Send(fromUdpSendRequestDTO(req))
	if err != nil {
		return UdpSendResult{}, err
	}
	return toUdpSendResultDTO(res), nil
}

// GetTargets は全ターゲットを返す。
func (h *UdpHandler) GetTargets() []UdpTarget {
	targets := h.targetSvc.GetTargets()
	result := make([]UdpTarget, len(targets))
	for i, t := range targets {
		result[i] = toUdpTargetDTO(t)
	}
	return result
}

// SaveTarget はターゲットを保存する。
func (h *UdpHandler) SaveTarget(target UdpTarget) (UdpTarget, error) {
	saved, err := h.targetSvc.SaveTarget(fromUdpTargetDTO(target))
	if err != nil {
		return UdpTarget{}, err
	}
	return toUdpTargetDTO(saved), nil
}

// DeleteTarget は ID でターゲットを削除する。
func (h *UdpHandler) DeleteTarget(id string) error {
	return h.targetSvc.DeleteTarget(id)
}

// StartListen は指定ポートで UDP リスニングを開始する。
func (h *UdpHandler) StartListen(port int, encoding string) (UdpListenSession, error) {
	session, err := h.listenSvc.StartListen(port, udpdomain.PayloadEncoding(encoding))
	if err != nil {
		return UdpListenSession{}, err
	}
	return toUdpListenSessionDTO(session), nil
}

// StopListen は指定セッションのリスニングを停止する。
func (h *UdpHandler) StopListen(sessionID string) error {
	return h.listenSvc.StopListen(sessionID)
}

// GetListeners はアクティブなリスニングセッション一覧を返す。
func (h *UdpHandler) GetListeners() []UdpListenSession {
	sessions := h.listenSvc.GetListeners()
	result := make([]UdpListenSession, len(sessions))
	for i, s := range sessions {
		result[i] = toUdpListenSessionDTO(s)
	}
	return result
}

// Shutdown は全リスニングセッションを停止する。
func (h *UdpHandler) Shutdown() {
	h.listenSvc.StopAll()
}
