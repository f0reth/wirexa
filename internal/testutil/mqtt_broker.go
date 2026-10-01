package testutil

import (
	"net"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/eclipse/paho.mqtt.golang/packets"
)

// DuplicatingBroker は、1 つの接続に重なる購読があると、一致する購読ごとに PUBLISH を 1 件ずつ送る
// 偽の MQTT ブローカー (MQTT 3.1.1 3.3.5 が認める動作)。mochi ブローカーは接続ごとに 1 件しか
// 送らないので、重複して届く環境の再現にはこれを使う。
// 複数の接続を受け付ける。配信は QoS 0 だけで、retained メッセージとセッションは持たない。
type DuplicatingBroker struct {
	ln    net.Listener
	conns map[*duplicatingConn]struct{}
	mu    sync.Mutex
}

// duplicatingConn はブローカーが受け付けた接続 1 本と、その購読。
type duplicatingConn struct {
	conn net.Conn
	// filters は購読中のフィルター (DuplicatingBroker.mu で保護)。
	filters []string
	// writeMu は応答 (この接続の goroutine) と配信 (publish した接続の goroutine) の書き込みを直列化する。
	writeMu sync.Mutex
}

func (c *duplicatingConn) write(pkt packets.ControlPacket) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return pkt.Write(c.conn)
}

// StartDuplicatingBroker は空きポートで DuplicatingBroker を起動する。テストの終了時に止まる。
func StartDuplicatingBroker(tb testing.TB) *DuplicatingBroker {
	tb.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		tb.Fatalf("listen: %v", err)
	}
	b := &DuplicatingBroker{ln: ln, conns: make(map[*duplicatingConn]struct{})}
	tb.Cleanup(b.close)
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go b.serve(conn)
		}
	}()
	return b
}

// URL は接続先のブローカー URL (tcp://host:port) を返す。
func (b *DuplicatingBroker) URL() string {
	return "tcp://" + b.ln.Addr().String()
}

// Subscriptions は全接続の購読数の合計を返す。SUBACK を返す前に数えるので、購読の完了待ちに使える。
func (b *DuplicatingBroker) Subscriptions() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	n := 0
	for c := range b.conns {
		n += len(c.filters)
	}
	return n
}

func (b *DuplicatingBroker) close() {
	_ = b.ln.Close() //nolint:errcheck // テスト用ブローカーの後始末。閉じる失敗は無視してよい
	b.mu.Lock()
	defer b.mu.Unlock()
	for c := range b.conns {
		_ = c.conn.Close() //nolint:errcheck // テスト用ブローカーの後始末。閉じる失敗は無視してよい
	}
}

// serve は接続 1 本のパケットを処理する。CONNECT を受理し、切断されるまで読み続ける。
func (b *DuplicatingBroker) serve(conn net.Conn) {
	defer conn.Close() //nolint:errcheck // テスト用ブローカーの後始末。閉じる失敗は無視してよい
	first, err := packets.ReadPacket(conn)
	if err != nil {
		return
	}
	if _, ok := first.(*packets.ConnectPacket); !ok {
		return
	}
	c := &duplicatingConn{conn: conn}
	connack, ok := packets.NewControlPacket(packets.Connack).(*packets.ConnackPacket)
	if !ok {
		return
	}
	connack.ReturnCode = packets.Accepted
	if c.write(connack) != nil {
		return
	}

	b.mu.Lock()
	b.conns[c] = struct{}{}
	b.mu.Unlock()
	defer func() {
		b.mu.Lock()
		delete(b.conns, c)
		b.mu.Unlock()
	}()

	for {
		pkt, err := packets.ReadPacket(conn)
		if err != nil {
			return
		}
		if !b.handle(c, pkt) {
			return
		}
	}
}

// handle はパケット 1 つを処理する。接続を閉じるべきなら false を返す。
func (b *DuplicatingBroker) handle(c *duplicatingConn, pkt packets.ControlPacket) bool {
	switch req := pkt.(type) {
	case *packets.SubscribePacket:
		b.mu.Lock()
		for _, filter := range req.Topics {
			if !slices.Contains(c.filters, filter) {
				c.filters = append(c.filters, filter)
			}
		}
		b.mu.Unlock()
		suback, ok := packets.NewControlPacket(packets.Suback).(*packets.SubackPacket)
		if !ok {
			return false
		}
		suback.MessageID = req.MessageID
		// 配信は QoS 0 だけなので、どの購読も QoS 0 で受理する。
		suback.ReturnCodes = make([]byte, len(req.Topics))
		return c.write(suback) == nil
	case *packets.UnsubscribePacket:
		b.mu.Lock()
		c.filters = slices.DeleteFunc(c.filters, func(filter string) bool {
			return slices.Contains(req.Topics, filter)
		})
		b.mu.Unlock()
		unsuback, ok := packets.NewControlPacket(packets.Unsuback).(*packets.UnsubackPacket)
		if !ok {
			return false
		}
		unsuback.MessageID = req.MessageID
		return c.write(unsuback) == nil
	case *packets.PublishPacket:
		if req.Qos == 1 {
			puback, ok := packets.NewControlPacket(packets.Puback).(*packets.PubackPacket)
			if !ok {
				return false
			}
			puback.MessageID = req.MessageID
			if c.write(puback) != nil {
				return false
			}
		}
		b.deliver(req.TopicName, req.Payload)
		return true
	case *packets.PingreqPacket:
		return c.write(packets.NewControlPacket(packets.Pingresp)) == nil
	case *packets.DisconnectPacket:
		return false
	default:
		return true
	}
}

// deliver は、全接続の一致する購読ごとに QoS 0 の PUBLISH を 1 件ずつ送る。
func (b *DuplicatingBroker) deliver(topic string, payload []byte) {
	type target struct {
		conn  *duplicatingConn
		count int
	}
	b.mu.Lock()
	targets := make([]target, 0, len(b.conns))
	for c := range b.conns {
		n := 0
		for _, filter := range c.filters {
			if topicFilterMatches(filter, topic) {
				n++
			}
		}
		if n > 0 {
			targets = append(targets, target{conn: c, count: n})
		}
	}
	b.mu.Unlock()

	for _, tg := range targets {
		for range tg.count {
			pub, ok := packets.NewControlPacket(packets.Publish).(*packets.PublishPacket)
			if !ok {
				return
			}
			pub.TopicName = topic
			pub.Payload = payload
			// 切断と行き違いの書き込み失敗は、その接続の読み取りループが検出する。
			_ = tg.conn.write(pub) //nolint:errcheck // 上記のとおり読み取りループに任せる
		}
	}
}

// topicFilterMatches は topic が購読のフィルター (+ と # を含み得る) に一致するかを返す。
func topicFilterMatches(filter, topic string) bool {
	filterLevels := strings.Split(filter, "/")
	topicLevels := strings.Split(topic, "/")
	for i, f := range filterLevels {
		if f == "#" {
			return true
		}
		if i >= len(topicLevels) || (f != "+" && f != topicLevels[i]) {
			return false
		}
	}
	return len(filterLevels) == len(topicLevels)
}
