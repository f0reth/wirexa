// Package mqtt provides MQTT client functionality for Wirexa.
package mqtt

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"sync"
	"time"

	pahomqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// ConnectionConfig holds the configuration for an MQTT connection.
type ConnectionConfig struct {
	Name     string `json:"name"`
	Broker   string `json:"broker"` // e.g. "tcp://localhost:1883"
	ClientID string `json:"clientId"`
	Username string `json:"username"`
	Password string `json:"password"`
	UseTLS   bool   `json:"useTls"`
}

// ConnectionStatus represents the current state of a connection.
type ConnectionStatus struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Broker    string `json:"broker"`
	Connected bool   `json:"connected"`
}

// MqttMessage represents a received MQTT message.
type MqttMessage struct {
	ConnectionID string `json:"connectionId"`
	Topic        string `json:"topic"`
	Payload      string `json:"payload"`
	QoS          byte   `json:"qos"`
	Retained     bool   `json:"retained"`
	Timestamp    int64  `json:"timestamp"`
}

const tokenTimeout = 30 * time.Second

type connection struct {
	client pahomqtt.Client
	config ConnectionConfig
}

// MqttService manages multiple MQTT connections.
type MqttService struct {
	ctx         context.Context
	mu          sync.RWMutex
	connections map[string]*connection
	connectWg   sync.WaitGroup
}

// NewMqttService creates a new MqttService instance.
func NewMqttService() *MqttService {
	return &MqttService{
		connections: make(map[string]*connection),
	}
}

// SetContext stores the Wails runtime context for event emission.
func (s *MqttService) SetContext(ctx context.Context) {
	s.mu.Lock()
	s.ctx = ctx
	s.mu.Unlock()
}

// Connect establishes an MQTT connection and returns a connection ID.
func (s *MqttService) Connect(config ConnectionConfig) (string, error) {
	if config.Broker == "" {
		return "", errors.New("broker URL is required")
	}

	connID := uuid.New().String()

	opts := pahomqtt.NewClientOptions()
	opts.AddBroker(config.Broker)

	if config.ClientID != "" {
		opts.SetClientID(config.ClientID)
	} else {
		opts.SetClientID("wirexa-" + connID[:8])
	}

	if config.Username != "" {
		opts.SetUsername(config.Username)
		opts.SetPassword(config.Password)
	}

	if config.UseTLS {
		opts.SetTLSConfig(&tls.Config{
			MinVersion: tls.VersionTLS12,
		})
	}

	opts.SetAutoReconnect(true)
	opts.SetConnectTimeout(10 * time.Second)

	// Capture ctx once under lock so handlers don't race on s.ctx.
	s.mu.RLock()
	ctx := s.ctx
	s.mu.RUnlock()

	opts.SetConnectionLostHandler(func(_ pahomqtt.Client, err error) {
		runtime.EventsEmit(ctx, "mqtt:connection-lost", map[string]any{
			"connectionId": connID,
			"error":        err.Error(),
		})
	})

	opts.SetOnConnectHandler(func(_ pahomqtt.Client) {
		runtime.EventsEmit(ctx, "mqtt:connected", map[string]any{
			"connectionId": connID,
		})
	})

	client := pahomqtt.NewClient(opts)

	s.mu.Lock()
	s.connections[connID] = &connection{client: client, config: config}
	s.mu.Unlock()

	s.connectWg.Go(func() {
		token := client.Connect()
		if !token.WaitTimeout(tokenTimeout) {
			s.mu.Lock()
			delete(s.connections, connID)
			s.mu.Unlock()
			runtime.EventsEmit(ctx, "mqtt:connection-failed", map[string]any{
				"connectionId": connID,
				"error":        "connection timeout",
			})
			return
		}
		if err := token.Error(); err != nil {
			s.mu.Lock()
			delete(s.connections, connID)
			s.mu.Unlock()
			runtime.EventsEmit(ctx, "mqtt:connection-failed", map[string]any{
				"connectionId": connID,
				"error":        err.Error(),
			})
		}
	})

	return connID, nil
}

// Disconnect closes an MQTT connection.
func (s *MqttService) Disconnect(connectionID string) error {
	s.mu.Lock()
	conn, ok := s.connections[connectionID]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("connection not found: %s", connectionID)
	}
	delete(s.connections, connectionID)
	s.mu.Unlock()

	conn.client.Disconnect(1000)

	s.mu.RLock()
	ctx := s.ctx
	s.mu.RUnlock()

	runtime.EventsEmit(ctx, "mqtt:disconnected", map[string]any{
		"connectionId": connectionID,
	})
	return nil
}

// Publish sends a message to the specified topic.
func (s *MqttService) Publish(connectionID, topic, payload string, qos byte, retain bool) error {
	if topic == "" {
		return errors.New("topic is required")
	}
	if qos > 2 {
		return errors.New("invalid QoS: must be 0, 1, or 2")
	}

	s.mu.RLock()
	conn, ok := s.connections[connectionID]
	s.mu.RUnlock()
	if !ok {
		return fmt.Errorf("connection not found: %s", connectionID)
	}

	token := conn.client.Publish(topic, qos, retain, payload)
	if !token.WaitTimeout(tokenTimeout) {
		return errors.New("publish timed out")
	}
	if err := token.Error(); err != nil {
		return fmt.Errorf("failed to publish: %w", err)
	}

	return nil
}

// Subscribe starts listening on the specified topic.
func (s *MqttService) Subscribe(connectionID, topic string, qos byte) error {
	if topic == "" {
		return errors.New("topic is required")
	}
	if qos > 2 {
		return errors.New("invalid QoS: must be 0, 1, or 2")
	}

	s.mu.RLock()
	conn, ok := s.connections[connectionID]
	ctx := s.ctx
	s.mu.RUnlock()
	if !ok {
		return fmt.Errorf("connection not found: %s", connectionID)
	}

	handler := func(_ pahomqtt.Client, msg pahomqtt.Message) {
		runtime.EventsEmit(ctx, "mqtt:message", MqttMessage{
			ConnectionID: connectionID,
			Topic:        msg.Topic(),
			Payload:      string(msg.Payload()),
			QoS:          msg.Qos(),
			Retained:     msg.Retained(),
			Timestamp:    time.Now().UnixMilli(),
		})
	}

	token := conn.client.Subscribe(topic, qos, handler)
	if !token.WaitTimeout(tokenTimeout) {
		return errors.New("subscribe timed out")
	}
	if err := token.Error(); err != nil {
		return fmt.Errorf("failed to subscribe: %w", err)
	}

	return nil
}

// Unsubscribe stops listening on the specified topic.
func (s *MqttService) Unsubscribe(connectionID, topic string) error {
	if topic == "" {
		return errors.New("topic is required")
	}

	s.mu.RLock()
	conn, ok := s.connections[connectionID]
	s.mu.RUnlock()
	if !ok {
		return fmt.Errorf("connection not found: %s", connectionID)
	}

	token := conn.client.Unsubscribe(topic)
	if !token.WaitTimeout(tokenTimeout) {
		return errors.New("unsubscribe timed out")
	}
	if err := token.Error(); err != nil {
		return fmt.Errorf("failed to unsubscribe: %w", err)
	}

	return nil
}

// GetConnections returns the status of all managed connections.
func (s *MqttService) GetConnections() []ConnectionStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	statuses := make([]ConnectionStatus, 0, len(s.connections))
	for id, conn := range s.connections {
		statuses = append(statuses, ConnectionStatus{
			ID:        id,
			Name:      conn.config.Name,
			Broker:    conn.config.Broker,
			Connected: conn.client.IsConnected(),
		})
	}
	return statuses
}

// Shutdown closes all active MQTT connections gracefully.
func (s *MqttService) Shutdown() {
	// Wait for any in-flight connect goroutines to finish.
	s.connectWg.Wait()

	s.mu.Lock()
	conns := make([]*connection, 0, len(s.connections))
	for _, conn := range s.connections {
		conns = append(conns, conn)
	}
	s.connections = make(map[string]*connection)
	s.mu.Unlock()

	for _, conn := range conns {
		conn.client.Disconnect(1000)
	}
}
