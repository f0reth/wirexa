// Package mqttinfra は MQTT インフラ層のアダプターを提供する。
package mqttinfra

import (
	"crypto/tls"
	"errors"
	"time"

	pahomqtt "github.com/eclipse/paho.mqtt.golang"

	domain "github.com/f0reth/Wirexa/internal/domain/mqtt"
)

const (
	pahoConnectTimeout = 10 * time.Second
	pahoTokenTimeout   = 30 * time.Second
)

// pahoClient は domain.BrokerClient の Paho MQTT 実装。
type pahoClient struct {
	client pahomqtt.Client
}

// NewPahoClientFactory は Paho MQTT を用いた domain.BrokerClientFactory を返す。
func NewPahoClientFactory() domain.BrokerClientFactory {
	return func(
		config domain.ConnectionConfig,
		onConnected func(),
		onConnectionLost func(error),
	) domain.BrokerClient {
		opts := pahomqtt.NewClientOptions()
		opts.AddBroker(config.Broker)
		opts.SetClientID(config.ClientID) // 呼び出し元が事前に解決済み

		if config.Username != "" {
			opts.SetUsername(config.Username)
			opts.SetPassword(config.Password)
		}

		if config.UseTLS {
			opts.SetTLSConfig(&tls.Config{MinVersion: tls.VersionTLS12})
		}

		opts.SetAutoReconnect(true)
		opts.SetResumeSubs(true)
		opts.SetConnectTimeout(pahoConnectTimeout)

		opts.SetOnConnectHandler(func(_ pahomqtt.Client) {
			onConnected()
		})
		opts.SetConnectionLostHandler(func(_ pahomqtt.Client, err error) {
			onConnectionLost(err)
		})

		return &pahoClient{client: pahomqtt.NewClient(opts)}
	}
}

func (p *pahoClient) Connect() error {
	token := p.client.Connect()
	if !token.WaitTimeout(pahoTokenTimeout) {
		return errors.New("connection timed out")
	}
	return token.Error()
}

func (p *pahoClient) Disconnect(quiesce uint) {
	p.client.Disconnect(quiesce)
}

func (p *pahoClient) Publish(topic string, qos byte, retained bool, payload string) error {
	token := p.client.Publish(topic, qos, retained, payload)
	if !token.WaitTimeout(pahoTokenTimeout) {
		return errors.New("publish timed out")
	}
	return token.Error()
}

func (p *pahoClient) Subscribe(topic string, qos byte, handler domain.MessageHandler) error {
	pahoHandler := func(_ pahomqtt.Client, msg pahomqtt.Message) {
		handler(msg.Topic(), string(msg.Payload()), msg.Qos(), msg.Retained())
	}
	token := p.client.Subscribe(topic, qos, pahoHandler)
	if !token.WaitTimeout(pahoTokenTimeout) {
		return errors.New("subscribe timed out")
	}
	return token.Error()
}

func (p *pahoClient) Unsubscribe(topics ...string) error {
	token := p.client.Unsubscribe(topics...)
	if !token.WaitTimeout(pahoTokenTimeout) {
		return errors.New("unsubscribe timed out")
	}
	return token.Error()
}

func (p *pahoClient) IsConnected() bool {
	return p.client.IsConnected()
}
