package udpdomain

// SendUseCase は UDP パケット送信のユースケース入力ポート。
type SendUseCase interface {
	Send(req UdpSendRequest) (UdpSendResult, error)
}

// TargetUseCase はターゲット管理のユースケース入力ポート。
type TargetUseCase interface {
	GetTargets() []UdpTarget
	SaveTarget(target UdpTarget) (UdpTarget, error)
	DeleteTarget(id string) error
}

// TargetRepository はターゲットの永続化抽象。
type TargetRepository interface {
	Load() ([]UdpTarget, error)
	Save(target *UdpTarget) error
	Delete(id string) error
}

// UdpEmitter は Wails イベント送信の抽象。
type UdpEmitter interface {
	Emit(event string, data any)
}

// ListenUseCase は UDP 受信管理のユースケース入力ポート。
type ListenUseCase interface {
	StartListen(port int, encoding PayloadEncoding) (UdpListenSession, error)
	StopListen(sessionID string) error
	GetListeners() []UdpListenSession
	StopAll()
}
