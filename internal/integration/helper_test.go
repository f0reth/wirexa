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
