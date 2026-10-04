package mqttinfra

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"os"
	"slices"
	"sync"
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

func TestApplyTLSScheme(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"tcp://broker:1883", "ssl://broker:1883"},
		{"mqtt://broker:1883", "mqtts://broker:1883"},
		{"ws://broker:8080/mqtt", "wss://broker:8080/mqtt"},
		{"ssl://broker:8883", "ssl://broker:8883"},
		{"mqtts://broker:8883", "mqtts://broker:8883"},
		{"wss://broker:443/mqtt", "wss://broker:443/mqtt"},
		{"tls://broker:8883", "tls://broker:8883"},
		// スキームの大文字小文字は区別しない (paho はスキームを小文字にして平文で接続する)。
		{"TCP://broker:1883", "ssl://broker:1883"},
		{"Mqtt://broker:1883", "mqtts://broker:1883"},
		{"WS://broker:8080/mqtt", "wss://broker:8080/mqtt"},
		// スキーム無しは paho が tcp:// を補うので、TLS のスキームを補う。
		{"broker:1883", "ssl://broker:1883"},
		// TLS にできないスキームはそのまま返す (呼び出し側が先に拒否する)。
		{"http://broker:1883", "http://broker:1883"},
		{"", ""},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			if got := applyTLSScheme(tc.in); got != tc.want {
				t.Errorf("applyTLSScheme(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// newFactoryClient は factory が組み立てた pahoClient を返す。
func newFactoryClient(t *testing.T, config domain.ConnectionConfig, onLost func(error)) *pahoClient {
	t.Helper()
	c := NewPahoClientFactory(MQTTClientConfig{ConnectTimeout: time.Second, TokenTimeout: time.Second})(
		config, func() {}, onLost,
	)
	p, ok := c.(*pahoClient)
	if !ok {
		t.Fatalf("factory returned %T, want *pahoClient", c)
	}
	return p
}

// UseTLS なら Broker のスキームを TLS 用に変え、TLS 1.2 以上の TLSConfig を設定する。
func TestNewPahoClientFactory_UseTLS(t *testing.T) {
	// スキーム無しや大文字のスキームでも、平文の tcp ではなく ssl で接続する。
	for _, broker := range []string{"tcp://broker:1883", "TCP://broker:1883", "broker:1883"} {
		t.Run(broker, func(t *testing.T) {
			p := newFactoryClient(t, domain.ConnectionConfig{Broker: broker, ClientID: "c", UseTLS: true}, func(error) {})

			opts := p.client.OptionsReader()
			servers := opts.Servers()
			if len(servers) != 1 || servers[0].Scheme != "ssl" || servers[0].Host != "broker:1883" {
				t.Fatalf("servers = %v, want ssl://broker:1883", servers)
			}
			if cfg := opts.TLSConfig(); cfg == nil || cfg.MinVersion != tls.VersionTLS12 {
				t.Fatalf("TLSConfig = %+v, want MinVersion TLS1.2", cfg)
			}
		})
	}
}

func TestNewPahoClientFactory_WithoutTLSKeepsScheme(t *testing.T) {
	p := newFactoryClient(t, domain.ConnectionConfig{Broker: "tcp://broker:1883", ClientID: "c"}, func(error) {})

	opts := p.client.OptionsReader()
	servers := opts.Servers()
	if len(servers) != 1 || servers[0].Scheme != "tcp" {
		t.Fatalf("servers = %v, want tcp://broker:1883", servers)
	}
}

// connectAndLose は、1 本目の接続だけ受理して少し後に切り、自動再接続の試行は受け付けない偽ブローカーに
// 接続し、onConnectionLost が受け取った原因を返す。返した client は自動再接続を試み続けている。
func connectAndLose(t *testing.T) (*pahoClient, error) {
	t.Helper()
	var accepted atomic.Int32
	broker := newFakeBroker(t, func(conn net.Conn) {
		defer conn.Close()
		// 自動再接続の試行は受け付けずに閉じる。
		if accepted.Add(1) > 1 {
			return
		}
		if readConnect(conn) != nil || writeConnack(conn) != nil {
			return
		}
		// CONNACK のあと少し置いてから切断する。
		_ = conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		_, _ = io.Copy(io.Discard, conn)
	})
	lost := make(chan error, 1)
	p := newFactoryClient(t, domain.ConnectionConfig{Broker: broker.url(), ClientID: "wirexa-test"}, func(err error) {
		select {
		case lost <- err:
		default:
		}
	})

	if err := p.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	t.Cleanup(func() { p.Disconnect(0) })

	select {
	case err := <-lost:
		return p, err
	case <-time.After(3 * time.Second):
		t.Fatal("onConnectionLost was not called after the broker closed the connection")
		return nil, nil
	}
}

// 確立済みの接続をブローカーが切ると、factory に渡した onConnectionLost がエラー付きで呼ばれる。
func TestPahoClient_ConnectionLost_InvokesCallback(t *testing.T) {
	if _, err := connectAndLose(t); err == nil {
		t.Error("onConnectionLost should receive the cause")
	}
}

// 接続が切れて自動再接続している間の Publish は、送らずにすぐエラーを返す。paho は再接続中の QoS 0 を
// 送らずに成功させ、QoS 1/2 を保存して再接続後に送るので、そのままでは返した結果と食い違う。
func TestPahoClient_Publish_WhileReconnecting_ReturnsError(t *testing.T) {
	p, _ := connectAndLose(t)

	for _, qos := range []byte{0, 1} {
		start := time.Now()
		err := p.Publish("sensors/temp", qos, false, "x")
		if err == nil {
			t.Errorf("Publish(qos %d) while reconnecting should fail", qos)
		}
		// TokenTimeout (1 秒) まで待たない (QoS 1 を保存して応答を待っていない)。
		if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
			t.Errorf("Publish(qos %d) took %v, want an immediate error", qos, elapsed)
		}
	}
}

// PUBACK が時間内に返らなければ、送達を確認できなかったことが分かるエラーを返す。
func TestPahoClient_Publish_AckTimeout(t *testing.T) {
	_, p := newSubscribingBroker(t)

	err := p.Publish("sensors/temp", 1, false, "x")

	if !errors.Is(err, domain.ErrAckTimeout) {
		t.Errorf("err = %v, want ErrAckTimeout", err)
	}
}

// 接続が開いていれば Publish はブローカーに届く。
func TestPahoClient_Publish_Connected(t *testing.T) {
	b, p := newSubscribingBroker(t)

	if err := p.Publish("sensors/temp", 0, false, "x"); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	select {
	case topic := <-b.published:
		if topic != "sensors/temp" {
			t.Errorf("broker received %q, want sensors/temp", topic)
		}
	case <-time.After(time.Second):
		t.Fatal("broker did not receive the PUBLISH")
	}
}

// subscribingBroker は SUBSCRIBE / UNSUBSCRIBE に応答し、テストから PUBLISH を送れる偽ブローカー。
// 購読の内容は見ず、テストが publish を呼んだ分だけ PUBLISH を送る (重なる購読があっても 1 件)。
// 受信した 1 件の PUBLISH で pahoClient が handler を何回呼ぶかを確かめるために使う。
// 重なる購読ごとに PUBLISH を 1 件ずつ送るブローカーの再現には testutil.DuplicatingBroker を使う。
type subscribingBroker struct {
	conn net.Conn
	// published は client から受信した PUBLISH のトピックを届いた順に流す (PUBACK は返さない)。
	published chan string
	// writeMu は応答 (接続の goroutine) と publish (テストの goroutine) の書き込みを直列化する。
	writeMu sync.Mutex
	// silentSubscribe / silentUnsubscribe が true の間は、SUBSCRIBE / UNSUBSCRIBE に応答しない。
	silentSubscribe   atomic.Bool
	silentUnsubscribe atomic.Bool
}

// newSubscribingBroker は偽ブローカーを起動して pahoClient を接続し、ブローカー側の接続を返す。
func newSubscribingBroker(t *testing.T) (*subscribingBroker, *pahoClient) {
	t.Helper()
	b := &subscribingBroker{published: make(chan string, 16)}
	ready := make(chan struct{})
	broker := newFakeBroker(t, func(conn net.Conn) {
		defer conn.Close()
		if readConnect(conn) != nil || writeConnack(conn) != nil {
			return
		}
		b.conn = conn
		close(ready)
		for {
			pkt, err := packets.ReadPacket(conn)
			if err != nil {
				return
			}
			if b.reply(pkt) != nil {
				return
			}
		}
	})
	p, _ := newTestClient(broker.url(), MQTTClientConfig{ConnectTimeout: time.Second, TokenTimeout: time.Second})
	if err := p.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	t.Cleanup(func() { p.Disconnect(0) })
	select {
	case <-ready:
	case <-time.After(time.Second):
		t.Fatal("broker did not accept the connection")
	}
	return b, p
}

// reply は SUBSCRIBE と UNSUBSCRIBE に成功の応答を返す (silentSubscribe / silentUnsubscribe の間は返さない)。
// PUBLISH は記録するだけで応答しない。それ以外のパケットは読み捨てる。
func (b *subscribingBroker) reply(pkt packets.ControlPacket) error {
	var ack packets.ControlPacket
	switch req := pkt.(type) {
	case *packets.PublishPacket:
		select {
		case b.published <- req.TopicName:
		default:
		}
		return nil
	case *packets.SubscribePacket:
		if b.silentSubscribe.Load() {
			return nil
		}
		suback, ok := packets.NewControlPacket(packets.Suback).(*packets.SubackPacket)
		if !ok {
			return errors.New("unexpected packet type")
		}
		suback.MessageID = req.MessageID
		suback.ReturnCodes = req.Qoss
		ack = suback
	case *packets.UnsubscribePacket:
		if b.silentUnsubscribe.Load() {
			return nil
		}
		unsuback, ok := packets.NewControlPacket(packets.Unsuback).(*packets.UnsubackPacket)
		if !ok {
			return errors.New("unexpected packet type")
		}
		unsuback.MessageID = req.MessageID
		ack = unsuback
	default:
		return nil
	}
	b.writeMu.Lock()
	defer b.writeMu.Unlock()
	return ack.Write(b.conn)
}

// publish は QoS 0 の PUBLISH を 1 件送る。
func (b *subscribingBroker) publish(t *testing.T, topic string) {
	t.Helper()
	pub, ok := packets.NewControlPacket(packets.Publish).(*packets.PublishPacket)
	if !ok {
		t.Fatal("unexpected packet type")
	}
	pub.TopicName = topic
	pub.Payload = []byte("x")
	b.writeMu.Lock()
	defer b.writeMu.Unlock()
	if err := pub.Write(b.conn); err != nil {
		t.Fatalf("publish %s: %v", topic, err)
	}
}

// receivedTopics は、topics を順に publish して、handler が受け取ったトピックを届いた順に返す。
// 最後に終端のトピックを送り、それが届くまで待つ (paho は受信した順に 1 件ずつ handler を呼ぶ)。
func receivedTopics(t *testing.T, b *subscribingBroker, p *pahoClient, topics ...string) []string {
	t.Helper()
	const sentinel = "wirexa-test/end"
	var (
		mu   sync.Mutex
		got  []string
		done = make(chan struct{})
		// closeDone は done を 1 回だけ閉じる。
		closeDone sync.Once
	)
	if err := p.Subscribe(sentinel, 0, noopHandler); err != nil {
		t.Fatalf("Subscribe(%s): %v", sentinel, err)
	}
	// 購読済みの全フィルターのハンドラを、受信を記録するものに置き換える。
	// 終端のトピックは # などにも一致するので、どの購読に振り分けられても終端として扱う。
	record := func(topic string, _ []byte, _ byte, _ bool) {
		if topic == sentinel {
			// 重複して届いても panic させず、記録した件数の比較で失敗させる。
			closeDone.Do(func() { close(done) })
			return
		}
		mu.Lock()
		defer mu.Unlock()
		got = append(got, topic)
	}
	p.routesMu.Lock()
	for i := range p.routes {
		p.routes[i].handler = record
	}
	p.routesMu.Unlock()

	for _, topic := range topics {
		b.publish(t, topic)
	}
	b.publish(t, sentinel)
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("the sentinel message was not delivered")
	}
	if err := p.Unsubscribe(sentinel); err != nil {
		t.Fatalf("Unsubscribe(%s): %v", sentinel, err)
	}
	mu.Lock()
	defer mu.Unlock()
	return got
}

func noopHandler(string, []byte, byte, bool) {}

// 重なる購読 (Broker Topics のスキャンが張る # と、個別の購読) があっても、受信した 1 件のメッセージで
// handler を呼ぶのは 1 回だけ。paho のルーターは一致する購読の数だけ呼ぶので、そのままでは
// メッセージが購読の数だけ重複して表示される。
func TestPahoClient_OverlappingSubscriptions_DeliverEachMessageOnce(t *testing.T) {
	b, p := newSubscribingBroker(t)
	for _, filter := range []string{"#", "sensors/#", "sensors/+", "sensors/temp"} {
		if err := p.Subscribe(filter, 0, noopHandler); err != nil {
			t.Fatalf("Subscribe(%s): %v", filter, err)
		}
	}

	got := receivedTopics(t, b, p, "sensors/temp", "other/topic")

	if want := []string{"sensors/temp", "other/topic"}; !slices.Equal(got, want) {
		t.Errorf("received = %v, want %v (one call per message)", got, want)
	}
}

// 同じフィルターを購読し直しても (再接続時の張り直し)、購読は増えず 1 回だけ届く。
func TestPahoClient_Resubscribe_DeliversEachMessageOnce(t *testing.T) {
	b, p := newSubscribingBroker(t)
	for range 3 {
		if err := p.Subscribe("sensors/temp", 1, noopHandler); err != nil {
			t.Fatalf("Subscribe: %v", err)
		}
	}

	got := receivedTopics(t, b, p, "sensors/temp")

	if want := []string{"sensors/temp"}; !slices.Equal(got, want) {
		t.Errorf("received = %v, want %v", got, want)
	}
}

// 重なる購読の片方を解除しても残りの購読で 1 回届き、一致する購読が無くなれば届かない。
func TestPahoClient_Unsubscribe_StopsDeliveryWhenNoFilterMatches(t *testing.T) {
	b, p := newSubscribingBroker(t)
	for _, filter := range []string{"#", "sensors/temp"} {
		if err := p.Subscribe(filter, 0, noopHandler); err != nil {
			t.Fatalf("Subscribe(%s): %v", filter, err)
		}
	}

	if err := p.Unsubscribe("#"); err != nil {
		t.Fatalf("Unsubscribe(#): %v", err)
	}
	if got, want := receivedTopics(t, b, p, "sensors/temp", "other/topic"), []string{"sensors/temp"}; !slices.Equal(got, want) {
		t.Errorf("after Unsubscribe(#): received = %v, want %v", got, want)
	}

	if err := p.Unsubscribe("sensors/temp"); err != nil {
		t.Fatalf("Unsubscribe(sensors/temp): %v", err)
	}
	if got := receivedTopics(t, b, p, "sensors/temp"); len(got) != 0 {
		t.Errorf("after unsubscribing every filter: received = %v, want none", got)
	}
}

// routedFilters は振り分け先に登録されているフィルターを登録順に返す。
func routedFilters(p *pahoClient) []string {
	p.routesMu.RLock()
	defer p.routesMu.RUnlock()
	filters := make([]string, 0, len(p.routes))
	for _, r := range p.routes {
		filters = append(filters, r.filter)
	}
	return filters
}

// UNSUBACK を時間内に確認できなかったら ErrAckTimeout を返すが、振り分け先は外す。
// ブローカーが解除していた場合に、購読中のままメッセージが届かない状態を残さない。
// ブローカー側に購読が残っていても、届いたメッセージは捨てる。
func TestPahoClient_Unsubscribe_AckTimeout_RemovesRoute(t *testing.T) {
	b, p := newSubscribingBroker(t)
	var received atomic.Int32
	if err := p.Subscribe("sensors/temp", 0, func(string, []byte, byte, bool) { received.Add(1) }); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	b.silentUnsubscribe.Store(true)

	err := p.Unsubscribe("sensors/temp")

	if !errors.Is(err, domain.ErrAckTimeout) {
		t.Errorf("err = %v, want ErrAckTimeout", err)
	}
	if got := routedFilters(p); len(got) != 0 {
		t.Errorf("routes = %v, want none after Unsubscribe timed out", got)
	}
	b.silentUnsubscribe.Store(false)
	// 終端のトピックが届くまでに sensors/temp のハンドラが呼ばれないこと。
	if got := receivedTopics(t, b, p, "sensors/temp"); len(got) != 0 {
		t.Errorf("received = %v, want none", got)
	}
	if n := received.Load(); n != 0 {
		t.Errorf("handler called %d times after Unsubscribe timed out, want 0", n)
	}
}

// SUBACK を時間内に確認できなかったら ErrAckTimeout を返し、その購読を振り分け先に残さない。
func TestPahoClient_Subscribe_AckTimeout_IsNotRouted(t *testing.T) {
	b, p := newSubscribingBroker(t)
	b.silentSubscribe.Store(true)

	err := p.Subscribe("sensors/temp", 0, noopHandler)

	if !errors.Is(err, domain.ErrAckTimeout) {
		t.Errorf("err = %v, want ErrAckTimeout", err)
	}
	if got := routedFilters(p); len(got) != 0 {
		t.Errorf("routes = %v, want none after Subscribe timed out", got)
	}
}

// SUBACK 待ち中に接続が切れても、再接続が確立すれば Subscribe は TokenTimeout を待たずに戻る。
// paho は ResumeSubs のとき切断で token を完了させないが、再接続時に保存済みの SUBSCRIBE を
// 同じメッセージ ID の新しい token で送り直し、そのときに元の token をエラー無しで完了させる。
// 待ち続けると、呼び出し元 (MQTTService) が opMu を持ったままになり、再接続後の onConnected
// (mqtt:connected の発行と張り直し) が TokenTimeout まで遅れる。
func TestPahoClient_Subscribe_ConnectionLostWhileWaiting(t *testing.T) {
	const tokenTimeout = 4 * time.Second
	var accepted atomic.Int32
	b := &subscribingBroker{published: make(chan string, 16)}
	broker := newFakeBroker(t, func(conn net.Conn) {
		defer conn.Close()
		first := accepted.Add(1) == 1
		if readConnect(conn) != nil || writeConnack(conn) != nil {
			return
		}
		if !first {
			b.conn = conn
		}
		for {
			pkt, err := packets.ReadPacket(conn)
			if err != nil {
				return
			}
			if first {
				// 1 本目は SUBSCRIBE を読んだら SUBACK を返さずに切る。
				if _, ok := pkt.(*packets.SubscribePacket); ok {
					return
				}
				continue
			}
			// 再接続した 2 本目は通常どおり応答する。
			if b.reply(pkt) != nil {
				return
			}
		}
	})
	connected := make(chan struct{}, 4)
	c := NewPahoClientFactory(MQTTClientConfig{ConnectTimeout: time.Second, TokenTimeout: tokenTimeout})(
		domain.ConnectionConfig{Broker: broker.url(), ClientID: "wirexa-test"},
		func() { connected <- struct{}{} },
		func(error) {},
	)
	p, ok := c.(*pahoClient)
	if !ok {
		t.Fatalf("factory returned %T, want *pahoClient", c)
	}
	if err := p.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	t.Cleanup(func() { p.Disconnect(0) })
	waitForSignal(t, connected, "onConnected was not called")

	start := time.Now()
	err := p.Subscribe("sensors/#", 1, noopHandler)
	elapsed := time.Since(start)

	waitForSignal(t, connected, "the client did not reconnect")
	if elapsed > tokenTimeout/2 {
		t.Errorf("Subscribe took %v, want it to return on reconnect instead of waiting for TokenTimeout (%v)", elapsed, tokenTimeout)
	}
	// 送り直した SUBSCRIBE の結果は確認できていないが、paho は成功として返す。
	// MQTTService は再接続時の張り直しで同じ購読をもう一度送り、その結果を確かめる。
	if err != nil {
		t.Errorf("Subscribe: %v", err)
	}
	if got, want := routedFilters(p), []string{"sensors/#"}; !slices.Equal(got, want) {
		t.Errorf("routes = %v, want %v", got, want)
	}
}

// waitForSignal は ch に値が届くまで待つ。
func waitForSignal(t *testing.T, ch <-chan struct{}, msg string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(3 * time.Second):
		t.Fatal(msg)
	}
}

// 購読が成立しなかったら、その購読を振り分け先に残さない。
func TestPahoClient_FailedSubscription_IsNotRouted(t *testing.T) {
	p, _ := newTestClient("tcp://127.0.0.1:1", MQTTClientConfig{ConnectTimeout: time.Second, TokenTimeout: time.Second})

	// 未接続なので Subscribe は失敗する。
	if err := p.Subscribe("sensors/temp", 0, noopHandler); err == nil {
		t.Fatal("Subscribe on a disconnected client should fail")
	}

	p.routesMu.RLock()
	defer p.routesMu.RUnlock()
	if len(p.routes) != 0 {
		t.Errorf("routes = %d, want none after a failed Subscribe", len(p.routes))
	}
}

func TestFilterMatches(t *testing.T) {
	tests := []struct {
		filter string
		topic  string
		want   bool
	}{
		{"a/b", "a/b", true},
		{"a/b", "a/b/c", false},
		{"a/b/c", "a/b", false},
		{"#", "a/b", true},
		{"a/#", "a/b/c", true},
		{"a/#", "a", true},
		{"a/#", "b/a", false},
		{"a/+", "a/b", true},
		{"a/+", "a", false},
		{"a/+", "a/b/c", false},
		{"+/b", "a/b", true},
		{"a/+/c", "a/b/c", true},
		{"+", "a", true},
		{"/a", "/a", true},
		{"/a", "a", false},
		{"$share/group/a/#", "a/b", true},
		{"$share/group/a/#", "b/b", false},
		{"$queue/a/b", "a/b", true},
	}
	for _, tc := range tests {
		t.Run(tc.filter+" "+tc.topic, func(t *testing.T) {
			if got := filterMatches(tc.filter, tc.topic); got != tc.want {
				t.Errorf("filterMatches(%q, %q) = %v, want %v", tc.filter, tc.topic, got, tc.want)
			}
		})
	}
}
