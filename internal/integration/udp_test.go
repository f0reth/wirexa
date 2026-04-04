//go:build integration

package integration

import (
	"testing"
	"time"

	"github.com/f0reth/Wirexa/internal/adapters"
	udpapp "github.com/f0reth/Wirexa/internal/application/udp"
	cmndomain "github.com/f0reth/Wirexa/internal/domain"
	udpdomain "github.com/f0reth/Wirexa/internal/domain/udp"
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

// newUDPHandler は統合テスト用に UdpHandler を DI で組み立てる。
func newUDPHandler(t *testing.T, emitter cmndomain.Emitter) *adapters.UdpHandler {
	t.Helper()
	repo, err := udpinfra.NewJSONTargetRepository(t.TempDir())
	if err != nil {
		t.Fatalf("NewJSONTargetRepository: %v", err)
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

	result, err := h.Send(udpdomain.UdpSendRequest{
		Host:     "127.0.0.1",
		Port:     port,
		Encoding: udpdomain.EncodingText,
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

	result, err := h.Send(udpdomain.UdpSendRequest{
		Host:     "127.0.0.1",
		Port:     port,
		Encoding: udpdomain.EncodingFixed,
		FixedLengthPayload: udpdomain.FixedLengthPayload{
			Fields: []udpdomain.FixedLengthField{
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
	target, err := h.SaveTarget(udpdomain.UdpTarget{
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
