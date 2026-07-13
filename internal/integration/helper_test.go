//go:build integration

// Package integration はバックエンド統合テストを提供する。
// 各テストは DI を手動で組み立て、テスト後にクリーンアップする。
package integration

import (
	"net"
	"testing"
)

// freePort はポート 0 でリッスンして空きポート番号を返す。
//
// 返した時点でリスナーは閉じているため、番号を使うまでの隙間に他プロセスがポートを
// 奪える。ただしこの helper の用途は「誰もリッスンしていないポート」(接続失敗テスト) と
// 「テスト対象自身に bind させるポート」であり、どちらもこちらが握り続けることはできない。
// 隙間を無くしたい呼び出し側は、リスナーを開いたまま実アドレスを読むこと (mqtt_test.go の
// TestMain が埋め込みブローカーでそうしている)。
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
// freePort と同じ制約 (返却時点で解放済み) がある。リスナーサービス自身に bind させるため。
func freeUDPPort(t *testing.T) int {
	t.Helper()
	conn, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("freeUDPPort: %v", err)
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).Port
}
