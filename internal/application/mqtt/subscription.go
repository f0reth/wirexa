package mqttapp

import (
	"errors"
	"fmt"
	"slices"
	"time"

	cmn "github.com/f0reth/Wirexa/internal/domain"
	domain "github.com/f0reth/Wirexa/internal/domain/mqtt"
)

// setSub は購読を登録する (stateMu 保持中に呼ぶ)。新しいトピックは購読順の末尾に足し、
// 購読中のトピックは QoS だけを変えて位置を保つ。
func (c *connection) setSub(topic string, qos byte) {
	if _, ok := c.subs[topic]; !ok {
		c.subOrder = append(c.subOrder, topic)
	}
	c.subs[topic] = qos
}

// deleteSub は購読を外す (stateMu 保持中に呼ぶ)。
func (c *connection) deleteSub(topic string) {
	if _, ok := c.subs[topic]; !ok {
		return
	}
	delete(c.subs, topic)
	c.subOrder = slices.DeleteFunc(c.subOrder, func(t string) bool { return t == topic })
}

// orderedSubs は購読を購読した順に複製して返す (stateMu 保持中に呼ぶ)。
func (c *connection) orderedSubs() []domain.SubscriptionInfo {
	subs := make([]domain.SubscriptionInfo, 0, len(c.subOrder))
	for _, topic := range c.subOrder {
		subs = append(subs, domain.SubscriptionInfo{Topic: topic, QoS: c.subs[topic]})
	}
	return subs
}

// updatePendingSubs は接続の確立前 (stateConnecting) なら client を呼ばずに update で subs だけを変え、
// true を返す。確立前の client は購読を受け付けない (paho は ErrNotConnected を返す) ので、
// 実際の購読・解除は onConnected の張り直しに任せる。確立済みなら何もせず false を返す。
// opMu の保持中 (withConn の中) に呼ぶ。
func (c *connection) updatePendingSubs(update func()) bool {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()
	if c.state != stateConnecting {
		return false
	}
	update()
	return true
}

// Publish は指定トピックへメッセージを送信する。
func (s *MQTTService) Publish(connectionID, topic, payload string, qos byte, retain bool) error {
	if err := domain.ValidateTopicName(topic); err != nil {
		return err
	}
	if err := domain.ValidateQoS(qos); err != nil {
		return err
	}
	return s.withConn(connectionID, func(conn *connection) error {
		if err := conn.client.Publish(topic, qos, retain, payload); err != nil {
			return fmt.Errorf("failed to publish: %w", err)
		}
		return nil
	})
}

// Subscribe は指定トピックの購読を開始する。接続の確立前なら登録だけ行い、確立時に購読する。
func (s *MQTTService) Subscribe(connectionID, topic string, qos byte) error {
	if err := domain.ValidateTopicFilter(topic); err != nil {
		return err
	}
	if err := domain.ValidateQoS(qos); err != nil {
		return err
	}
	return s.withConn(connectionID, func(conn *connection) error {
		if conn.updatePendingSubs(func() { conn.setSub(topic, qos) }) {
			return nil
		}
		if err := conn.client.Subscribe(topic, qos, s.messageHandler(connectionID, conn)); err != nil {
			return fmt.Errorf("failed to subscribe: %w", err)
		}
		conn.stateMu.Lock()
		conn.setSub(topic, qos)
		conn.stateMu.Unlock()
		return nil
	})
}

// messageHandler は受信したメッセージを mqtt:message イベントとして発行するハンドラを返す。
func (s *MQTTService) messageHandler(connectionID string, conn *connection) domain.MessageHandler {
	return func(msgTopic string, msgPayload []byte, msgQoS byte, retained bool) {
		// RLock なのでメッセージ同士は直列化しない。状態遷移 (Lock) は実行中の発行の完了を待つ。
		conn.stateMu.RLock()
		defer conn.stateMu.RUnlock()
		if conn.terminal() {
			return // 切断・shutdown 済みの接続のメッセージは捨てる
		}
		s.logger.Info("MQTT message received", "source", "mqtt", "connection_id", connectionID, "topic", msgTopic, "payload_bytes", len(msgPayload))
		// 非 UTF-8 のバイナリペイロードは string 変換で壊れるため base64 で渡す。
		payloadStr, payloadBase64 := cmn.EncodeMaybeBase64(msgPayload)
		s.emitter.Emit(cmn.EventMQTTMessage, domain.MQTTMessage{
			ConnectionID:  connectionID,
			Topic:         msgTopic,
			Payload:       payloadStr,
			PayloadBase64: payloadBase64,
			QoS:           msgQoS,
			Retained:      retained,
			Timestamp:     time.Now().UnixMilli(),
		})
	}
}

// resubscribe は接続の確立後に subs の購読を購読した順に張り直す。client は CleanSession で接続するので、
// 再接続したブローカー側には前回の購読が残っていない。張り直さないと、GetConnections は
// 購読中と返し続けるのにメッセージが届かなくなる。確立前に受け付けた購読もここで購読する。
// onConnected (client のコールバック用 goroutine) から、opMu を保持したまま呼ぶ。
func (s *MQTTService) resubscribe(connID string, conn *connection, subs []domain.SubscriptionInfo) {
	for _, sub := range subs {
		topic, qos := sub.Topic, sub.QoS
		// 途中で切断された接続 (Disconnect の detach は opMu を取らずに遷移させる) と、
		// Unsubscribe で外された購読は張り直さない。
		conn.stateMu.RLock()
		_, still := conn.subs[topic]
		closed := conn.terminal()
		conn.stateMu.RUnlock()
		if closed {
			return
		}
		if !still {
			continue
		}
		err := conn.client.Subscribe(topic, qos, s.messageHandler(connID, conn))
		if err == nil {
			continue
		}
		s.logger.Error("MQTT resubscribe failed", "source", "mqtt", "connection_id", connID, "topic", topic, "error", err)
		// ブローカーが拒否した購読と、接続が開いたまま失敗した購読 (SUBACK を時間内に確認できなかった等) は
		// 表示から外す。後者は次の再接続が来ないので、残すと購読中と表示されたままメッセージが届かない。
		// 接続が切れて失敗した購読は残し、次の再接続で張り直す。
		if errors.Is(err, domain.ErrSubscriptionRejected) || conn.client.IsConnected() {
			conn.stateMu.Lock()
			conn.deleteSub(topic)
			conn.stateMu.Unlock()
			// client は張り直しの失敗では振り分け先を残すので、解除して片付ける。ブローカーが購読を
			// 受理していた場合も、これでブローカー側の購読が消える。
			// 失敗しても (直後にまた切断した等) 表示からは外したままにする。振り分け先が無ければ
			// 届いたメッセージは捨てられるので、表示とは食い違わない。
			if err := conn.client.Unsubscribe(topic); err != nil {
				s.logger.Error("MQTT unsubscribe of a failed subscription failed", "source", "mqtt", "connection_id", connID, "topic", topic, "error", err)
			}
		}
	}
}

// Unsubscribe は指定トピックの購読を解除する。接続の確立前なら登録を外すだけで client は呼ばない。
// ブローカーの応答を確認できなかった場合 (domain.ErrAckTimeout) はエラーを返すが、購読は外す。
func (s *MQTTService) Unsubscribe(connectionID, topic string) error {
	if err := domain.ValidateTopicFilter(topic); err != nil {
		return err
	}
	return s.withConn(connectionID, func(conn *connection) error {
		if conn.updatePendingSubs(func() { conn.deleteSub(topic) }) {
			return nil
		}
		err := conn.client.Unsubscribe(topic)
		// 応答を確認できなかった解除は、client が振り分け先を外しているので購読も外す
		// (残すと購読中と表示されたままメッセージが届かない)。それ以外の失敗では購読を残す。
		if err == nil || errors.Is(err, domain.ErrAckTimeout) {
			conn.stateMu.Lock()
			conn.deleteSub(topic)
			conn.stateMu.Unlock()
		}
		if err != nil {
			return fmt.Errorf("failed to unsubscribe: %w", err)
		}
		return nil
	})
}
