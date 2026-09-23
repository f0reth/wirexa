package mqttinfra

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/eclipse/paho.mqtt.golang/packets"

	domain "github.com/f0reth/Wirexa/internal/domain/mqtt"
)

// fakeBroker は素の TCP リスナーで MQTT ブローカーの応答を制御する。
// mochi ブローカーでは CONNACK の遅延を制御できないため、接続ごとの振る舞いを onConn で与える。
type fakeBroker struct {
	ln net.Listener
}

// newFakeBroker は接続を受け付けるたびに onConn を別 goroutine で呼ぶ偽ブローカーを起動する。
func newFakeBroker(t *testing.T, onConn func(conn net.Conn)) *fakeBroker {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go onConn(conn)
		}
	}()
	return &fakeBroker{ln: ln}
}

func (b *fakeBroker) url() string {
	return "tcp://" + b.ln.Addr().String()
}

// readConnect は CONNECT パケットを 1 つ読む。
func readConnect(conn net.Conn) error {
	pkt, err := packets.ReadPacket(conn)
	if err != nil {
		return err
	}
	if _, ok := pkt.(*packets.ConnectPacket); !ok {
		return errors.New("first packet is not CONNECT")
	}
	return nil
}

// writeConnack は接続受理の CONNACK を送る。
func writeConnack(conn net.Conn) error {
	ack, ok := packets.NewControlPacket(packets.Connack).(*packets.ConnackPacket)
	if !ok {
		return errors.New("unexpected packet type")
	}
	ack.ReturnCode = packets.Accepted
	return ack.Write(conn)
}

// waitClosed は相手 (paho) が接続を閉じるまで読み捨てる。DISCONNECT パケットは読み飛ばす。
// タイムアウトした場合は接続が閉じられていないとみなしてテストを失敗させる。
func waitClosed(t *testing.T, conn net.Conn, timeout time.Duration) {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(timeout))
	for {
		if _, err := packets.ReadPacket(conn); err != nil {
			if errors.Is(err, os.ErrDeadlineExceeded) {
				t.Fatal("client did not close the connection")
			}
			return
		}
	}
}

// newTestClient は偽ブローカー向けの pahoClient と onConnected の呼び出し回数を返す。
func newTestClient(broker string, cfg MQTTClientConfig) (*pahoClient, *atomic.Int32) {
	var connectedCalls atomic.Int32
	c := NewPahoClientFactory(cfg)(
		domain.ConnectionConfig{Broker: broker, ClientID: "wirexa-test"},
		func() { connectedCalls.Add(1) },
		func(error) {},
	)
	p, ok := c.(*pahoClient)
	if !ok {
		panic("factory did not return *pahoClient")
	}
	return p, &connectedCalls
}

// assertNoOnConnected は OnConnect の goroutine が走り切る猶予を置いてから、onConnected が呼ばれていないことを確認する。
func assertNoOnConnected(t *testing.T, calls *atomic.Int32) {
	t.Helper()
	time.Sleep(100 * time.Millisecond)
	if n := calls.Load(); n != 0 {
		t.Errorf("onConnected called %d times, want 0", n)
	}
}

func TestPahoClient_Connect_Success(t *testing.T) {
	broker := newFakeBroker(t, func(conn net.Conn) {
		defer conn.Close()
		if readConnect(conn) != nil || writeConnack(conn) != nil {
			return
		}
		_, _ = io.Copy(io.Discard, conn)
	})
	p, calls := newTestClient(broker.url(), MQTTClientConfig{ConnectTimeout: time.Second, TokenTimeout: time.Second})

	if err := p.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer p.Disconnect(0)
	if !p.IsConnected() {
		t.Error("expected IsConnected() = true")
	}
	deadline := time.Now().Add(time.Second)
	for calls.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if calls.Load() != 1 {
		t.Errorf("onConnected called %d times, want 1", calls.Load())
	}
}

// TokenTimeout < ConnectTimeout の構成で、TokenTimeout 後に CONNACK が届いても接続が残らないこと。
func TestPahoClient_Connect_TokenTimeout_NoLateConnection(t *testing.T) {
	serverConns := make(chan net.Conn, 1)
	broker := newFakeBroker(t, func(conn net.Conn) {
		if readConnect(conn) != nil {
			_ = conn.Close()
			return
		}
		time.Sleep(200 * time.Millisecond)
		_ = writeConnack(conn)
		serverConns <- conn
	})
	p, calls := newTestClient(broker.url(), MQTTClientConfig{
		ConnectTimeout: time.Second,
		TokenTimeout:   50 * time.Millisecond,
	})

	err := p.Connect(context.Background())
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if p.IsConnected() {
		t.Error("expected IsConnected() = false after timeout")
	}
	select {
	case conn := <-serverConns:
		defer conn.Close()
		waitClosed(t, conn, 2*time.Second)
	case <-time.After(time.Second):
		t.Fatal("broker did not send CONNACK")
	}
	assertNoOnConnected(t, calls)
}

// ctx を cancel しても、paho の接続試行が終わるまで Connect が復帰しないこと。
func TestPahoClient_Connect_Cancel_WaitsForAttempt(t *testing.T) {
	accepted := make(chan net.Conn, 2)
	// accept だけして応答しない。MQTT 3.1 へのフォールバックで 2 本目の接続が来るので、それも黙って受ける。
	broker := newFakeBroker(t, func(conn net.Conn) { accepted <- conn })
	// 試行 (2 × ConnectTimeout) が abortWait より長く続く構成にし、後片付け待ちではなく
	// 試行の完了待ちで復帰していることを区別する。
	p, calls := newTestClient(broker.url(), MQTTClientConfig{
		ConnectTimeout: 700 * time.Millisecond,
		TokenTimeout:   10 * time.Second,
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- p.Connect(ctx) }()

	var first net.Conn
	select {
	case first = <-accepted:
		defer first.Close()
	case <-time.After(time.Second):
		t.Fatal("broker did not accept")
	}
	cancel()

	var err error
	select {
	case err = <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Connect did not return after cancel")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
	if p.IsConnected() {
		t.Error("expected IsConnected() = false after cancel")
	}
	// 復帰時点で、試行に使われた接続はすべて paho 側で閉じられている。
	waitClosed(t, first, 100*time.Millisecond)
	select {
	case second := <-accepted:
		defer second.Close()
		waitClosed(t, second, 100*time.Millisecond)
	case <-time.After(time.Second):
		t.Fatal("broker did not accept the MQTT 3.1 fallback connection")
	}
	assertNoOnConnected(t, calls)
}

// ctx の打ち切り後に CONNACK が届いた場合も、復帰時点で接続が閉じられていること。
func TestPahoClient_Connect_CancelThenLateConnack_Disconnects(t *testing.T) {
	gotConnect := make(chan net.Conn, 1)
	sendConnack := make(chan struct{})
	broker := newFakeBroker(t, func(conn net.Conn) {
		if readConnect(conn) != nil {
			_ = conn.Close()
			return
		}
		gotConnect <- conn
		<-sendConnack
		_ = writeConnack(conn)
	})
	p, calls := newTestClient(broker.url(), MQTTClientConfig{
		ConnectTimeout: 2 * time.Second,
		TokenTimeout:   10 * time.Second,
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- p.Connect(ctx) }()

	var conn net.Conn
	select {
	case conn = <-gotConnect:
		defer conn.Close()
	case <-time.After(time.Second):
		t.Fatal("broker did not receive CONNECT")
	}
	cancel()
	// 打ち切りが始まってから CONNACK を返す。
	deadline := time.Now().Add(time.Second)
	for !p.aborting.Load() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	close(sendConnack)

	var err error
	select {
	case err = <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Connect did not return after cancel")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
	if p.IsConnected() {
		t.Error("expected IsConnected() = false after cancel")
	}
	waitClosed(t, conn, 100*time.Millisecond)
	assertNoOnConnected(t, calls)
}
