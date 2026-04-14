// Package mqttapp は MQTT 接続管理ユースケースを提供する。
package mqttapp

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"

	cmn "github.com/f0reth/Wirexa/internal/domain"
	domain "github.com/f0reth/Wirexa/internal/domain/mqtt"
)

const shutdownTimeout = 5 * time.Second

const (
	eventConnected        = "mqtt:connected"
	eventDisconnected     = "mqtt:disconnected"
	eventConnectionLost   = "mqtt:connection-lost"
	eventConnectionFailed = "mqtt:connection-failed"
	eventMessage          = "mqtt:message"
)

var _ domain.MqttUseCase = (*MqttService)(nil)

type connection struct {
	id     string
	config domain.ConnectionConfig
	client domain.BrokerClient
}

// MqttService は複数の MQTT 接続を管理するアプリケーションサービス。
type MqttService struct {
	emitter       cmn.Emitter
	clientFactory domain.BrokerClientFactory
	logger        cmn.Logger
	mu            sync.RWMutex
	conns         map[string]*connection
	connWg        sync.WaitGroup
}

// NewMqttService は MqttService を生成する。
func NewMqttService(emitter cmn.Emitter, clientFactory domain.BrokerClientFactory, logger cmn.Logger) *MqttService {
	return &MqttService{
		emitter:       emitter,
		clientFactory: clientFactory,
		logger:        logger,
		conns:         make(map[string]*connection),
	}
}

// Connect は MQTT ブローカーへ接続し、接続 ID を返す。
func (s *MqttService) Connect(config domain.ConnectionConfig) (string, error) {
	if config.Broker == "" {
		return "", &cmn.ValidationError{Field: "broker URL", Message: "is required"}
	}

	connID := uuid.New().String()

	// ClientID が未指定の場合は自動生成
	if config.ClientID == "" {
		config.ClientID = "wirexa-" + connID[:8]
	}

	s.logger.Info("MQTT connecting", "source", "mqtt", "broker", config.Broker, "client_id", config.ClientID)

	client := s.clientFactory(
		config,
		func() {
			s.logger.Info("MQTT connected", "source", "mqtt", "connection_id", connID, "broker", config.Broker)
			s.emitter.Emit(eventConnected, map[string]any{
				"connectionId": connID,
			})
		},
		func(err error) {
			s.logger.Error("MQTT connection lost", "source", "mqtt", "connection_id", connID, "error", err)
			s.emitter.Emit(eventConnectionLost, map[string]any{
				"connectionId": connID,
				"error":        err.Error(),
			})
		},
	)

	s.mu.Lock()
	s.conns[connID] = &connection{id: connID, client: client, config: config}
	s.mu.Unlock()

	s.connWg.Go(func() {
		if err := client.Connect(); err != nil {
			s.mu.Lock()
			delete(s.conns, connID)
			s.mu.Unlock()
			s.logger.Error("MQTT connection failed", "source", "mqtt", "connection_id", connID, "error", err)
			s.emitter.Emit(eventConnectionFailed, map[string]any{
				"connectionId": connID,
				"error":        err.Error(),
			})
		}
	})

	return connID, nil
}

// Disconnect は指定した接続を切断する。
func (s *MqttService) Disconnect(connectionID string) error {
	s.mu.Lock()
	conn, ok := s.conns[connectionID]
	if !ok {
		s.mu.Unlock()
		return &cmn.NotFoundError{Resource: "connection", ID: connectionID}
	}
	delete(s.conns, connectionID)
	s.mu.Unlock()

	conn.client.Disconnect(1000)

	s.logger.Info("MQTT disconnected", "source", "mqtt", "connection_id", connectionID)
	s.emitter.Emit(eventDisconnected, map[string]any{
		"connectionId": connectionID,
	})
	return nil
}

// withConn はロックを保持したまま接続を取得し、fn を呼び出す。
// ロック解放後に接続が削除される TOCTOU 競合を防ぐ。
func (s *MqttService) withConn(id string, fn func(conn *connection) error) error {
	s.mu.RLock()
	conn, ok := s.conns[id]
	s.mu.RUnlock()
	if !ok {
		return &cmn.NotFoundError{Resource: "connection", ID: id}
	}
	return fn(conn)
}

// Publish は指定トピックへメッセージを送信する。
func (s *MqttService) Publish(connectionID, topic, payload string, qos byte, retain bool) error {
	if topic == "" {
		return &cmn.ValidationError{Field: "topic", Message: "is required"}
	}
	if qos > 2 {
		return &cmn.ValidationError{Field: "qos", Message: "must be 0, 1, or 2"}
	}
	return s.withConn(connectionID, func(conn *connection) error {
		return conn.client.Publish(topic, qos, retain, payload)
	})
}

// Subscribe は指定トピックの購読を開始する。
func (s *MqttService) Subscribe(connectionID, topic string, qos byte) error {
	if topic == "" {
		return &cmn.ValidationError{Field: "topic", Message: "is required"}
	}
	if qos > 2 {
		return &cmn.ValidationError{Field: "qos", Message: "must be 0, 1, or 2"}
	}
	return s.withConn(connectionID, func(conn *connection) error {
		handler := func(msgTopic, msgPayload string, msgQoS byte, retained bool) {
			s.logger.Info("MQTT message received", "source", "mqtt", "connection_id", connectionID, "topic", msgTopic, "payload_bytes", len(msgPayload))
			s.emitter.Emit(eventMessage, domain.MqttMessage{
				ConnectionID: connectionID,
				Topic:        msgTopic,
				Payload:      msgPayload,
				QoS:          msgQoS,
				Retained:     retained,
				Timestamp:    time.Now().UnixMilli(),
			})
		}
		return conn.client.Subscribe(topic, qos, handler)
	})
}

// Unsubscribe は指定トピックの購読を解除する。
func (s *MqttService) Unsubscribe(connectionID, topic string) error {
	if topic == "" {
		return &cmn.ValidationError{Field: "topic", Message: "is required"}
	}
	return s.withConn(connectionID, func(conn *connection) error {
		return conn.client.Unsubscribe(topic)
	})
}

// GetConnections は全接続の現在状態を返す。
func (s *MqttService) GetConnections() []domain.ConnectionStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	statuses := make([]domain.ConnectionStatus, 0, len(s.conns))
	for id, conn := range s.conns {
		statuses = append(statuses, domain.ConnectionStatus{
			ID:        id,
			Name:      conn.config.Name,
			Broker:    conn.config.Broker,
			Connected: conn.client.IsConnected(),
		})
	}
	return statuses
}

// Shutdown は全接続を切断してサービスを終了する。
// 接続goroutineの完了を最大 shutdownTimeout 待つ。
func (s *MqttService) Shutdown() {
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
