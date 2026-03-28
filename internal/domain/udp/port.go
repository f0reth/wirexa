package udpdomain

// UdpConn は UDP パケットコネクションの抽象。
type UdpConn interface {
	// ReadFrom はパケットを受信し、バイト数・送信元アドレス・エラーを返す。
	ReadFrom(b []byte) (n int, addr string, err error)
	// Close はコネクションを閉じる。
	Close() error
}

// UdpSocket は UDP ソケット操作の抽象。
// 実装は infrastructure/udp に置く。
type UdpSocket interface {
	// Send はデータを指定ホスト・ポートへ送信し、送信バイト数を返す。
	Send(host string, port int, data []byte) (int, error)
	// Listen は指定ポートでリスニングを開始し、UdpConn を返す。
	Listen(port int) (UdpConn, error)
}

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

// ListenUseCase は UDP 受信管理のユースケース入力ポート。
type ListenUseCase interface {
	StartListen(port int, encoding PayloadEncoding) (UdpListenSession, error)
	StopListen(sessionID string) error
	GetListeners() []UdpListenSession
	StopAll()
}
