package udpdomain

// UDPConn は UDP パケットコネクションの抽象。
type UDPConn interface {
	ReadFrom(b []byte) (n int, addr string, err error)
	Close() error
}

// UDPSocket は UDP ソケット操作の抽象。
type UDPSocket interface {
	Send(host string, port int, data []byte) (int, error)
	Listen(port int) (UDPConn, error)
}

// TargetRepository はターゲットの永続化抽象。
type TargetRepository interface {
	Load() ([]UDPTarget, error)
	Save(target *UDPTarget) error
	Delete(id string) error
}
