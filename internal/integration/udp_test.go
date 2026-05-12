//go:build integration

package integration

import (
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/f0reth/Wirexa/internal/adapters"
	udpapp "github.com/f0reth/Wirexa/internal/application/udp"
	cmndomain "github.com/f0reth/Wirexa/internal/domain"
	udpdomain "github.com/f0reth/Wirexa/internal/domain/udp"
	infra "github.com/f0reth/Wirexa/internal/infrastructure"
	udpinfra "github.com/f0reth/Wirexa/internal/infrastructure/udp"
	"github.com/f0reth/Wirexa/internal/testutil"
)

// mockEmitter はテスト用の Emitter 実装。受信メッセージをチャンネルで収集する。
type mockEmitter struct {
	ch chan udpdomain.UdpReceivedMessage
}

func newMockEmitter() *mockEmitter {
	return &mockEmitter{ch: make(chan udpdomain.UdpReceivedMessage, 16)}
}

func (e *mockEmitter) Emit(_ string, data any) {
	if msg, ok := data.(udpdomain.UdpReceivedMessage); ok {
		select {
		case e.ch <- msg:
		default:
		}
	}
}

// receiveMessage はタイムアウト付きでメッセージを待機する。
func (e *mockEmitter) receiveMessage(t *testing.T, timeout time.Duration) udpdomain.UdpReceivedMessage {
	t.Helper()
	select {
	case msg := <-e.ch:
		return msg
	case <-time.After(timeout):
		t.Fatal("timeout waiting for UDP message")
		return udpdomain.UdpReceivedMessage{}
	}
}

// newUDPHandlerWithDir は指定ディレクトリから UdpHandler を組み立てる（永続化テスト用）。
func newUDPHandlerWithDir(t *testing.T, emitter cmndomain.Emitter, dir string) *adapters.UdpHandler {
	t.Helper()
	repo, err := infra.NewJSONStore(dir, func(tgt *udpdomain.UdpTarget) string { return tgt.ID })
	if err != nil {
		t.Fatalf("NewJSONStore: %v", err)
	}
	targetSvc, err := udpapp.NewTargetService(repo)
	if err != nil {
		t.Fatalf("NewTargetService: %v", err)
	}
	socket := udpinfra.NewNetSocket()
	sendSvc := udpapp.NewUdpSendService(socket, testutil.NoopLogger{})
	listenSvc := udpapp.NewUdpListenerService(socket, emitter, testutil.NoopLogger{})
	h := &adapters.UdpHandler{}
	adapters.SetupUdpHandler(h, sendSvc, targetSvc, listenSvc)
	return h
}

// newUDPHandler は統合テスト用に UdpHandler を DI で組み立てる。
func newUDPHandler(t *testing.T, emitter cmndomain.Emitter) *adapters.UdpHandler {
	t.Helper()
	return newUDPHandlerWithDir(t, emitter, t.TempDir())
}

// TestUDP_SendRaw はテキストエンコードで送信したペイロードがリスナーに届くことを確認する。
func TestUDP_SendRaw(t *testing.T) {
	emitter := newMockEmitter()
	h := newUDPHandler(t, emitter)

	port := freeUDPPort(t)
	sess, err := h.StartListen(port, string(udpdomain.EncodingText))
	if err != nil {
		t.Fatalf("StartListen: %v", err)
	}
	t.Cleanup(func() { _ = h.StopListen(sess.ID) })

	result, err := h.Send(adapters.UdpSendRequest{
		Host:     "127.0.0.1",
		Port:     port,
		Encoding: string(udpdomain.EncodingText),
		Payload:  "hello",
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if result.BytesSent != len("hello") {
		t.Errorf("BytesSent = %d, want %d", result.BytesSent, len("hello"))
	}

	msg := emitter.receiveMessage(t, 3*time.Second)
	if msg.Payload != "hello" {
		t.Errorf("Payload = %q, want hello", msg.Payload)
	}
	if msg.SessionID != sess.ID {
		t.Errorf("SessionID = %q, want %q", msg.SessionID, sess.ID)
	}
	if msg.Port != port {
		t.Errorf("Port = %d, want %d", msg.Port, port)
	}
}

// TestUDP_SendFixed は Fixed (hex) エンコードで送受信が正しく動くことを確認する。
func TestUDP_SendFixed(t *testing.T) {
	emitter := newMockEmitter()
	h := newUDPHandler(t, emitter)

	port := freeUDPPort(t)
	// リスナーも fixed エンコードで起動し、受信ペイロードが hex 文字列になることを確認する
	sess, err := h.StartListen(port, string(udpdomain.EncodingFixed))
	if err != nil {
		t.Fatalf("StartListen: %v", err)
	}
	t.Cleanup(func() { _ = h.StopListen(sess.ID) })

	result, err := h.Send(adapters.UdpSendRequest{
		Host:     "127.0.0.1",
		Port:     port,
		Encoding: string(udpdomain.EncodingFixed),
		FixedLengthPayload: adapters.FixedLengthPayload{
			Fields: []adapters.FixedLengthField{
				{Name: "data", Length: 4, Value: "ABCD"},
			},
		},
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if result.BytesSent != 4 {
		t.Errorf("BytesSent = %d, want 4", result.BytesSent)
	}

	msg := emitter.receiveMessage(t, 3*time.Second)
	// EncodePayload([]byte{'A','B','C','D'}, EncodingFixed) = hex.EncodeToString(...) = "41424344"
	if msg.Payload != "41424344" {
		t.Errorf("Payload = %q, want 41424344", msg.Payload)
	}
}

// TestUDP_ListenerStartStop は StartListen/StopListen でセッション管理が正しいことを確認する。
func TestUDP_ListenerStartStop(t *testing.T) {
	emitter := newMockEmitter()
	h := newUDPHandler(t, emitter)

	port := freeUDPPort(t)
	sess, err := h.StartListen(port, string(udpdomain.EncodingText))
	if err != nil {
		t.Fatalf("StartListen: %v", err)
	}
	if sess.Port != port {
		t.Errorf("session Port = %d, want %d", sess.Port, port)
	}
	if sess.ID == "" {
		t.Error("expected non-empty session ID")
	}

	if err := h.StopListen(sess.ID); err != nil {
		t.Fatalf("StopListen: %v", err)
	}

	// 二重停止はエラーになる
	if err := h.StopListen(sess.ID); err == nil {
		t.Error("expected error on double StopListen, got nil")
	}
}

// TestUDP_GetListeners はアクティブセッション一覧が正しく返ることを確認する。
func TestUDP_GetListeners(t *testing.T) {
	emitter := newMockEmitter()
	h := newUDPHandler(t, emitter)

	if listeners := h.GetListeners(); len(listeners) != 0 {
		t.Fatalf("expected 0 listeners initially, got %d", len(listeners))
	}

	port1 := freeUDPPort(t)
	sess1, err := h.StartListen(port1, string(udpdomain.EncodingText))
	if err != nil {
		t.Fatalf("StartListen port1: %v", err)
	}
	t.Cleanup(func() { _ = h.StopListen(sess1.ID) })

	port2 := freeUDPPort(t)
	sess2, err := h.StartListen(port2, string(udpdomain.EncodingText))
	if err != nil {
		t.Fatalf("StartListen port2: %v", err)
	}
	t.Cleanup(func() { _ = h.StopListen(sess2.ID) })

	listeners := h.GetListeners()
	if len(listeners) != 2 {
		t.Fatalf("expected 2 listeners, got %d", len(listeners))
	}

	ids := map[string]bool{listeners[0].ID: true, listeners[1].ID: true}
	if !ids[sess1.ID] || !ids[sess2.ID] {
		t.Error("listener IDs do not match started sessions")
	}
}

// TestUDP_TargetCRUD はターゲットの保存・取得・削除が永続化されることを確認する。
func TestUDP_TargetCRUD(t *testing.T) {
	emitter := newMockEmitter()
	h := newUDPHandler(t, emitter)

	if targets := h.GetTargets(); len(targets) != 0 {
		t.Fatalf("expected 0 targets initially, got %d", len(targets))
	}

	// 新規保存
	target, err := h.SaveTarget(adapters.UdpTarget{
		Name: "TestTarget",
		Host: "192.168.1.1",
		Port: 5000,
	})
	if err != nil {
		t.Fatalf("SaveTarget: %v", err)
	}
	if target.ID == "" {
		t.Error("expected non-empty ID after save")
	}
	if target.Name != "TestTarget" {
		t.Errorf("Name = %q, want TestTarget", target.Name)
	}

	// 取得
	targets := h.GetTargets()
	if len(targets) != 1 || targets[0].ID != target.ID {
		t.Fatalf("GetTargets: got %d items, want 1", len(targets))
	}

	// 削除
	if err := h.DeleteTarget(target.ID); err != nil {
		t.Fatalf("DeleteTarget: %v", err)
	}
	if targets := h.GetTargets(); len(targets) != 0 {
		t.Errorf("expected 0 targets after delete, got %d", len(targets))
	}

	// 存在しない ID は NotFoundError
	if err := h.DeleteTarget("nonexistent"); err == nil {
		t.Error("expected error for nonexistent target, got nil")
	}
}

// TestUDP_Shutdown は全セッションが停止することを確認する。
func TestUDP_Shutdown(t *testing.T) {
	emitter := newMockEmitter()
	h := newUDPHandler(t, emitter)

	port1 := freeUDPPort(t)
	if _, err := h.StartListen(port1, string(udpdomain.EncodingText)); err != nil {
		t.Fatalf("StartListen port1: %v", err)
	}
	port2 := freeUDPPort(t)
	if _, err := h.StartListen(port2, string(udpdomain.EncodingText)); err != nil {
		t.Fatalf("StartListen port2: %v", err)
	}

	if len(h.GetListeners()) != 2 {
		t.Fatalf("expected 2 listeners before shutdown")
	}

	h.Shutdown()

	if len(h.GetListeners()) != 0 {
		t.Errorf("expected 0 listeners after shutdown, got %d", len(h.GetListeners()))
	}
}

// TestUDP_TargetPersistenceRoundTrip は SaveTarget 後に同一 dir で再作成した Handler でデータが復元されることを確認する。
func TestUDP_TargetPersistenceRoundTrip(t *testing.T) {
	dir := t.TempDir()
	emitter := newMockEmitter()

	// 1 回目: ターゲットを保存
	h1 := newUDPHandlerWithDir(t, emitter, dir)
	target, err := h1.SaveTarget(adapters.UdpTarget{
		Name: "PersistTarget",
		Host: "192.168.1.100",
		Port: 9999,
	})
	if err != nil {
		t.Fatalf("SaveTarget: %v", err)
	}

	// 2 回目: 同一 dir から Handler を再作成してデータを確認
	h2 := newUDPHandlerWithDir(t, emitter, dir)
	targets := h2.GetTargets()
	if len(targets) != 1 {
		t.Fatalf("expected 1 target after reload, got %d", len(targets))
	}
	if targets[0].ID != target.ID {
		t.Errorf("target ID = %q, want %q", targets[0].ID, target.ID)
	}
	if targets[0].Name != "PersistTarget" {
		t.Errorf("target Name = %q, want PersistTarget", targets[0].Name)
	}
	if targets[0].Host != "192.168.1.100" {
		t.Errorf("target Host = %q, want 192.168.1.100", targets[0].Host)
	}
}

// TestUDP_StartListen_DuplicatePort は同一ポートで 2 回 StartListen を呼ぶと ValidationError が返ることを確認する。
func TestUDP_StartListen_DuplicatePort(t *testing.T) {
	emitter := newMockEmitter()
	h := newUDPHandler(t, emitter)

	port := freeUDPPort(t)
	sess, err := h.StartListen(port, string(udpdomain.EncodingText))
	if err != nil {
		t.Fatalf("first StartListen: %v", err)
	}
	t.Cleanup(func() { _ = h.StopListen(sess.ID) })

	// 同一ポートで再度 StartListen → ValidationError
	_, err = h.StartListen(port, string(udpdomain.EncodingText))
	if err == nil {
		t.Error("expected error on duplicate port StartListen, got nil")
	}
}

// TestUDP_StartListen_PortInUse は OS が既にバインドしているポートで StartListen を呼ぶと error が返ることを確認する。
func TestUDP_StartListen_PortInUse(t *testing.T) {
	emitter := newMockEmitter()
	h := newUDPHandler(t, emitter)

	// 先にポートを OS レベルで全インターフェースで先占する
	prebound, err := net.ListenPacket("udp4", ":0")
	if err != nil {
		t.Fatalf("ListenPacket (pre-bind): %v", err)
	}
	port := prebound.LocalAddr().(*net.UDPAddr).Port
	defer prebound.Close()

	// 同じポートで StartListen → OS レベルのエラー
	_, err = h.StartListen(port, string(udpdomain.EncodingText))
	if err == nil {
		t.Error("expected error for port already in use, got nil")
	}
}

// TestUDP_Send_EmptyHost は Host が空文字列のとき ValidationError が返ることを確認する。
func TestUDP_Send_EmptyHost(t *testing.T) {
	emitter := newMockEmitter()
	h := newUDPHandler(t, emitter)

	_, err := h.Send(adapters.UdpSendRequest{
		Host:     "",
		Port:     5000,
		Encoding: string(udpdomain.EncodingText),
		Payload:  "test",
	})
	if err == nil {
		t.Error("expected error for empty host, got nil")
	}
}

// TestUDP_Send_InvalidPort は Port が範囲外のとき ValidationError が返ることを確認する。
func TestUDP_Send_InvalidPort(t *testing.T) {
	emitter := newMockEmitter()
	h := newUDPHandler(t, emitter)

	for _, port := range []int{0, 65536, -1} {
		_, err := h.Send(adapters.UdpSendRequest{
			Host:     "127.0.0.1",
			Port:     port,
			Encoding: string(udpdomain.EncodingText),
			Payload:  "test",
		})
		if err == nil {
			t.Errorf("expected error for port=%d, got nil", port)
		}
	}
}

// TestUDP_StartListen_InvalidPort は port が範囲外のとき ValidationError が返ることを確認する。
func TestUDP_StartListen_InvalidPort(t *testing.T) {
	emitter := newMockEmitter()
	h := newUDPHandler(t, emitter)

	for _, port := range []int{0, 65536, -1} {
		_, err := h.StartListen(port, string(udpdomain.EncodingText))
		if err == nil {
			t.Errorf("expected error for port=%d, got nil", port)
		}
	}
}

// TestUDP_StartListen_AfterStop は StopListen 後に同一ポートで再 StartListen が成功することを確認する。
func TestUDP_StartListen_AfterStop(t *testing.T) {
	emitter := newMockEmitter()
	h := newUDPHandler(t, emitter)

	port := freeUDPPort(t)

	sess, err := h.StartListen(port, string(udpdomain.EncodingText))
	if err != nil {
		t.Fatalf("first StartListen: %v", err)
	}
	if err := h.StopListen(sess.ID); err != nil {
		t.Fatalf("StopListen: %v", err)
	}

	// OS がポートを解放したことを確認するため、再 StartListen を試みる
	sess2, err := h.StartListen(port, string(udpdomain.EncodingText))
	if err != nil {
		t.Fatalf("second StartListen after stop: %v", err)
	}
	_ = h.StopListen(sess2.ID)
}

// TestUDP_Send_UnreachableHost は存在しないホストへの Send でエラーが返ることを確認する。
func TestUDP_Send_UnreachableHost(t *testing.T) {
	emitter := newMockEmitter()
	h := newUDPHandler(t, emitter)

	// 解決不能なホスト名を使い DNS 失敗を確実に引き起こす
	_, err := h.Send(adapters.UdpSendRequest{
		Host:     "invalid.hostname.that.does.not.exist.wirexa-test.invalid",
		Port:     5000,
		Encoding: string(udpdomain.EncodingText),
		Payload:  "test",
	})
	if err == nil {
		t.Error("expected error for unresolvable host, got nil")
	}
}

// TestUDP_SendJSON は JSON エンコードで送受信が正しく動くことを確認する。
func TestUDP_SendJSON(t *testing.T) {
	emitter := newMockEmitter()
	h := newUDPHandler(t, emitter)

	port := freeUDPPort(t)
	sess, err := h.StartListen(port, string(udpdomain.EncodingJSON))
	if err != nil {
		t.Fatalf("StartListen: %v", err)
	}
	t.Cleanup(func() { _ = h.StopListen(sess.ID) })

	payload := `{"key":"value"}`
	result, err := h.Send(adapters.UdpSendRequest{
		Host:     "127.0.0.1",
		Port:     port,
		Encoding: string(udpdomain.EncodingText),
		Payload:  payload,
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if result.BytesSent != len(payload) {
		t.Errorf("BytesSent = %d, want %d", result.BytesSent, len(payload))
	}

	msg := emitter.receiveMessage(t, 3*time.Second)
	// リスナーが JSON エンコードなので受信ペイロードはそのまま（または JSON として表現される）
	if msg.SessionID != sess.ID {
		t.Errorf("SessionID = %q, want %q", msg.SessionID, sess.ID)
	}
}

// TestUDP_EncodingMismatch はリスナーの Encoding と送信 Encoding が異なる場合、受信側は自身の Encoding で処理することを確認する。
func TestUDP_EncodingMismatch(t *testing.T) {
	emitter := newMockEmitter()
	h := newUDPHandler(t, emitter)

	port := freeUDPPort(t)
	// リスナーは Text エンコード
	sess, err := h.StartListen(port, string(udpdomain.EncodingText))
	if err != nil {
		t.Fatalf("StartListen: %v", err)
	}
	t.Cleanup(func() { _ = h.StopListen(sess.ID) })

	// 送信は Fixed エンコード
	_, err = h.Send(adapters.UdpSendRequest{
		Host:     "127.0.0.1",
		Port:     port,
		Encoding: string(udpdomain.EncodingFixed),
		FixedLengthPayload: adapters.FixedLengthPayload{
			Fields: []adapters.FixedLengthField{
				{Name: "data", Length: 4, Value: "ABCD"},
			},
		},
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	// リスナーは Text エンコードで受信するため、raw バイトをテキストとして表現する
	msg := emitter.receiveMessage(t, 3*time.Second)
	if msg.SessionID != sess.ID {
		t.Errorf("SessionID = %q, want %q", msg.SessionID, sess.ID)
	}
	// エンコーディングが Text なのでペイロードは受信バイトをそのまま文字列化
	if msg.Payload != "ABCD" {
		t.Errorf("Payload = %q, want ABCD (text encoding of raw bytes)", msg.Payload)
	}
}

// TestUDP_StopListen_RaceWithSend は Send 直後に StopListen を呼んでも goroutine が安全に終了することを確認する。
func TestUDP_StopListen_RaceWithSend(t *testing.T) {
	emitter := newMockEmitter()
	h := newUDPHandler(t, emitter)

	port := freeUDPPort(t)
	sess, err := h.StartListen(port, string(udpdomain.EncodingText))
	if err != nil {
		t.Fatalf("StartListen: %v", err)
	}

	// Send と StopListen を並行して実行し race condition を発生させる
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		_, _ = h.Send(adapters.UdpSendRequest{
			Host:     "127.0.0.1",
			Port:     port,
			Encoding: string(udpdomain.EncodingText),
			Payload:  "race-test",
		})
	}()

	go func() {
		defer wg.Done()
		_ = h.StopListen(sess.ID)
	}()

	wg.Wait()
	// goroutine leak がなく正常終了することを確認（race detector で検出）
}

// TestUDP_Concurrent_StartStopListen は複数 goroutine から StartListen / StopListen を並行実行しても安全であることを確認する。
func TestUDP_Concurrent_StartStopListen(t *testing.T) {
	emitter := newMockEmitter()
	h := newUDPHandler(t, emitter)
	t.Cleanup(func() { h.Shutdown() })

	const n = 4
	ports := make([]int, n)
	for i := range n {
		ports[i] = freeUDPPort(t)
	}

	var wg sync.WaitGroup
	sessIDs := make(chan string, n)

	for _, port := range ports {
		p := port
		wg.Add(1)
		go func() {
			defer wg.Done()
			sess, err := h.StartListen(p, string(udpdomain.EncodingText))
			if err == nil {
				sessIDs <- sess.ID
			}
		}()
	}
	wg.Wait()
	close(sessIDs)

	for id := range sessIDs {
		_ = h.StopListen(id)
	}
}

// TestUDP_CorruptStorage はストレージの JSON ファイルが不正な状態で NewTargetService がエラーを返すことを確認する。
func TestUDP_CorruptStorage(t *testing.T) {
	dir := t.TempDir()

	// 壊れた JSON ファイルをストレージディレクトリに配置する
	corruptFile := filepath.Join(dir, "corrupt.json")
	if err := os.WriteFile(corruptFile, []byte("{invalid json}"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// NewTargetService の Load() が失敗することを確認する
	repo, err := infra.NewJSONStore(dir, func(tgt *udpdomain.UdpTarget) string { return tgt.ID })
	if err != nil {
		t.Fatalf("NewJSONStore: %v", err)
	}
	_, err = udpapp.NewTargetService(repo)
	if err == nil {
		t.Error("expected error for corrupt storage, got nil")
	}
}
