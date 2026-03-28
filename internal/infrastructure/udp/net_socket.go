package udpinfra

import (
	"fmt"
	"net"
	"strconv"

	domain "github.com/f0reth/Wirexa/internal/domain/udp"
)

var _ domain.UdpSocket = (*NetSocket)(nil)

// NetSocket は net パッケージを使った domain.UdpSocket の実装。
type NetSocket struct{}

// NewNetSocket は NetSocket を生成する。
func NewNetSocket() *NetSocket {
	return &NetSocket{}
}

// Send は UDP パケットを指定ホスト・ポートへ送信する。
func (s *NetSocket) Send(host string, port int, data []byte) (int, error) {
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	conn, err := net.Dial("udp4", addr)
	if err != nil {
		return 0, fmt.Errorf("failed to connect: %w", err)
	}
	defer conn.Close()
	return conn.Write(data)
}

// Listen は指定ポートでリスニングを開始し、UdpConn を返す。
func (s *NetSocket) Listen(port int) (domain.UdpConn, error) {
	conn, err := net.ListenPacket("udp4", fmt.Sprintf(":%d", port))
	if err != nil {
		return nil, err
	}
	return &netConn{conn: conn}, nil
}

// netConn は net.PacketConn を domain.UdpConn にアダプトする。
type netConn struct {
	conn net.PacketConn
}

func (c *netConn) ReadFrom(b []byte) (int, string, error) {
	n, addr, err := c.conn.ReadFrom(b)
	if err != nil {
		return 0, "", err
	}
	return n, addr.String(), nil
}

func (c *netConn) Close() error {
	return c.conn.Close()
}
