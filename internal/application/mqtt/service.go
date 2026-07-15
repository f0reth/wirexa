// Package mqttapp は MQTT 接続管理ユースケースを提供する。
package mqttapp

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	cmn "github.com/f0reth/Wirexa/internal/domain"
	domain "github.com/f0reth/Wirexa/internal/domain/mqtt"
)

const shutdownTimeout = 5 * time.Second

const (
	keyConnectionID = "connectionId"
	fieldTopic      = "topic"
)

var _ domain.MQTTUseCase = (*MQTTService)(nil)

type connection struct {
	client domain.BrokerClient
	config domain.ConnectionConfig
	// subs は現在購読中のトピック→QoS。リロード後の状態復元のためサーバー側で保持する。
	// s.mu は接続 map の保護用で、購読 map の変更は withConn(RLock 保持) 中に起きるため
	// per-connection の subMu で保護する。
	subMu sync.Mutex
	subs  map[string]byte
}

// MQTTService は複数の MQTT 接続を管理するアプリケーションサービス。
type MQTTService struct {
	emitter       cmn.Emitter
	logger        cmn.Logger
	clientFactory domain.BrokerClientFactory
	conns         map[string]*connection
	connWg        sync.WaitGroup
	mu            sync.RWMutex
}

// NewMQTTService は MQTTService を生成する。
func NewMQTTService(emitter cmn.Emitter, clientFactory domain.BrokerClientFactory, logger cmn.Logger) *MQTTService {
	return &MQTTService{
		emitter:       emitter,
		clientFactory: clientFactory,
		logger:        logger,
		conns:         make(map[string]*connection),
	}
}

// Connect は MQTT ブローカーへ接続し、接続 ID を返す。
func (s *MQTTService) Connect(config domain.ConnectionConfig) (string, error) {
	if config.Broker == "" {
		return "", &cmn.ValidationError{Field: "broker URL", Message: cmn.MsgRequired}
	}

	connID := uuid.NewString()

	// ClientID が未指定の場合は自動生成
	if config.ClientID == "" {
		config.ClientID = "wirexa-" + connID[:8]
	}

	s.logger.Info("MQTT connecting", "source", "mqtt", "broker", config.Broker, "client_id", config.ClientID)

	client := s.clientFactory(
		config,
		func() {
			s.logger.Info("MQTT connected", "source", "mqtt", "connection_id", connID, "broker", config.Broker)
			s.emitter.Emit(cmn.EventMQTTConnected, map[string]any{
				keyConnectionID: connID,
			})
		},
		func(err error) {
			s.logger.Error("MQTT connection lost", "source", "mqtt", "connection_id", connID, "error", err)
			s.emitter.Emit(cmn.EventMQTTConnectionLost, map[string]any{
				keyConnectionID: connID,
				"error":         err.Error(),
			})
		},
	)

	s.mu.Lock()
	s.conns[connID] = &connection{client: client, config: config, subs: make(map[string]byte)}
	s.mu.Unlock()

	s.connWg.Go(func() {
		if err := client.Connect(); err != nil {
			s.mu.Lock()
			delete(s.conns, connID)
			s.mu.Unlock()
			s.logger.Error("MQTT connection failed", "source", "mqtt", "connection_id", connID, "error", err)
			s.emitter.Emit(cmn.EventMQTTConnectionFailed, map[string]any{
				keyConnectionID: connID,
				"error":         err.Error(),
			})
		}
	})

	return connID, nil
}

// Disconnect は指定した接続を切断する。
func (s *MQTTService) Disconnect(connectionID string) error {
	s.mu.Lock()
	conn, ok := s.conns[connectionID]
	if !ok {
		s.mu.Unlock()
		return &cmn.NotFoundError{Resource: cmn.ResourceConnection, ID: connectionID}
	}
	delete(s.conns, connectionID)
	s.mu.Unlock()

	conn.client.Disconnect(1000)

	s.logger.Info("MQTT disconnected", "source", "mqtt", "connection_id", connectionID)
	s.emitter.Emit(cmn.EventMQTTDisconnected, map[string]any{
		keyConnectionID: connectionID,
	})
	return nil
}

// withConn はロックを保持したまま接続を取得し、fn を呼び出す。
// ロック解放後に接続が削除される TOCTOU 競合を防ぐ。
func (s *MQTTService) withConn(id string, fn func(conn *connection) error) error {
	s.mu.RLock()
	conn, ok := s.conns[id]
	s.mu.RUnlock()
	if !ok {
		return &cmn.NotFoundError{Resource: cmn.ResourceConnection, ID: id}
	}
	return fn(conn)
}

// Publish は指定トピックへメッセージを送信する。
func (s *MQTTService) Publish(connectionID, topic, payload string, qos byte, retain bool) error {
	if topic == "" {
		return &cmn.ValidationError{Field: fieldTopic, Message: cmn.MsgRequired}
	}
	if qos > 2 {
		return &cmn.ValidationError{Field: "qos", Message: "must be 0, 1, or 2"}
	}
	return s.withConn(connectionID, func(conn *connection) error {
		if err := conn.client.Publish(topic, qos, retain, payload); err != nil {
			return fmt.Errorf("failed to publish: %w", err)
		}
		return nil
	})
}

// Subscribe は指定トピックの購読を開始する。
func (s *MQTTService) Subscribe(connectionID, topic string, qos byte) error {
	if topic == "" {
		return &cmn.ValidationError{Field: fieldTopic, Message: cmn.MsgRequired}
	}
	if qos > 2 {
		return &cmn.ValidationError{Field: "qos", Message: "must be 0, 1, or 2"}
	}
	return s.withConn(connectionID, func(conn *connection) error {
		handler := func(msgTopic string, msgPayload []byte, msgQoS byte, retained bool) {
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
		if err := conn.client.Subscribe(topic, qos, handler); err != nil {
			return fmt.Errorf("failed to subscribe: %w", err)
		}
		conn.subMu.Lock()
		conn.subs[topic] = qos
		conn.subMu.Unlock()
		return nil
	})
}

// Unsubscribe は指定トピックの購読を解除する。
func (s *MQTTService) Unsubscribe(connectionID, topic string) error {
	if topic == "" {
		return &cmn.ValidationError{Field: fieldTopic, Message: cmn.MsgRequired}
	}
	return s.withConn(connectionID, func(conn *connection) error {
		if err := conn.client.Unsubscribe(topic); err != nil {
			return fmt.Errorf("failed to unsubscribe: %w", err)
		}
		conn.subMu.Lock()
		delete(conn.subs, topic)
		conn.subMu.Unlock()
		return nil
	})
}

// GetConnections は全接続の現在状態を返す。
func (s *MQTTService) GetConnections() []domain.ConnectionStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	statuses := make([]domain.ConnectionStatus, 0, len(s.conns))
	for id, conn := range s.conns {
		conn.subMu.Lock()
		subs := make([]domain.SubscriptionInfo, 0, len(conn.subs))
		for topic, qos := range conn.subs {
			subs = append(subs, domain.SubscriptionInfo{Topic: topic, QoS: qos})
		}
		conn.subMu.Unlock()
		statuses = append(statuses, domain.ConnectionStatus{
			ID:            id,
			Name:          conn.config.Name,
			Broker:        conn.config.Broker,
			Connected:     conn.client.IsConnected(),
			ProfileID:     conn.config.ProfileID,
			Subscriptions: subs,
		})
	}
	return statuses
}

// Shutdown は全接続を切断してサービスを終了する。
// 接続goroutineの完了を最大 shutdownTimeout 待つ。
func (s *MQTTService) Shutdown() {
	done := make(chan struct{})
	go func() {
		s.connWg.Wait()
		close(done)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	select {
	case <-done:
	case <-ctx.Done():
	}

	s.mu.Lock()
	conns := make([]*connection, 0, len(s.conns))
	for _, conn := range s.conns {
		conns = append(conns, conn)
	}
	s.conns = make(map[string]*connection)
	s.mu.Unlock()

	for _, conn := range conns {
		conn.client.Disconnect(1000)
	}
}
