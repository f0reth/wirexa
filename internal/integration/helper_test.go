//go:build integration

// Package integration はバックエンド統合テストを提供する。
// 各テストは DI を手動で組み立て、テスト後にクリーンアップする。
package integration

import (
	"net"
	"testing"
)

// freePort はポート 0 でリッスンして空きポート番号を返す。
func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("freePort: %v", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

// freeUDPPort は UDP ポート 0 でバインドして空き UDP ポート番号を返す。
func freeUDPPort(t *testing.T) int {
	t.Helper()
	conn, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("freeUDPPort: %v", err)
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).Port
}
