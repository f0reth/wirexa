// Package mqttinfra は MQTT インフラ層のアダプターを提供する。
package mqttinfra

import (
	"context"
	"crypto/tls"
	"errors"
	"strings"
	"sync/atomic"
	"time"

	pahomqtt "github.com/eclipse/paho.mqtt.golang"

	domain "github.com/f0reth/Wirexa/internal/domain/mqtt"
)

const (
	defaultConnectTimeout = 10 * time.Second
	defaultTokenTimeout   = 30 * time.Second
)

// abortWait は、打ち切り後に確立してしまった接続の後片付けを待つ上限 (ms)。
// 2 回目の Disconnect は 1 回目の後片付けが終わった時点で復帰するため、通常はこれより早く返る。
const abortWait = 1000

// MQTTClientConfig は Paho MQTT クライアントの設定。
type MQTTClientConfig struct {
	ConnectTimeout time.Duration
	TokenTimeout   time.Duration
}

func (c MQTTClientConfig) connectTimeout() time.Duration {
	if c.ConnectTimeout > 0 {
		return c.ConnectTimeout
	}
	return defaultConnectTimeout
}

func (c MQTTClientConfig) tokenTimeout() time.Duration {
	if c.TokenTimeout > 0 {
		return c.TokenTimeout
	}
	return defaultTokenTimeout
}

// pahoClient は domain.BrokerClient の Paho MQTT 実装。
type pahoClient struct {
	client       pahomqtt.Client
	tokenTimeout time.Duration
	// aborting は Connect の打ち切りを開始したことを示す。
	// 打ち切り後に確立した接続でも paho は OnConnect を起動するため、これを見て onConnected を呼ばない。
	aborting atomic.Bool
}

// applyTLSScheme は UseTLS=true の場合、Broker URL のスキームを TLS 対応のものに変換する。
func applyTLSScheme(broker string) string {
	switch {
	case strings.HasPrefix(broker, "tcp://"):
		return "ssl://" + broker[len("tcp://"):]
	case strings.HasPrefix(broker, "mqtt://"):
		return "mqtts://" + broker[len("mqtt://"):]
	case strings.HasPrefix(broker, "ws://"):
		return "wss://" + broker[len("ws://"):]
	}
	return broker
}

// NewPahoClientFactory は Paho MQTT を用いた domain.BrokerClientFactory を返す。
func NewPahoClientFactory(cfg MQTTClientConfig) domain.BrokerClientFactory {
	return func(
		config domain.ConnectionConfig,
		onConnected func(),
		onConnectionLost func(error),
	) domain.BrokerClient {
		opts := pahomqtt.NewClientOptions()
		opts.SetClientID(config.ClientID) // 呼び出し元が事前に解決済み

		if config.Username != "" {
			opts.SetUsername(config.Username)
			opts.SetPassword(config.Password)
		}

		broker := config.Broker
		if config.UseTLS {
			broker = applyTLSScheme(broker)
			opts.SetTLSConfig(&tls.Config{MinVersion: tls.VersionTLS12})
		}
		opts.AddBroker(broker)

		opts.SetAutoReconnect(true)
		opts.SetResumeSubs(true)
		opts.SetConnectTimeout(cfg.connectTimeout())

		// OnConnect ハンドラが p を参照できるよう、pahoClient を先に作ってから client を生成する。
		p := &pahoClient{tokenTimeout: cfg.tokenTimeout()}
		opts.SetOnConnectHandler(func(_ pahomqtt.Client) {
			if p.aborting.Load() {
				return
			}
			onConnected()
		})
		opts.SetConnectionLostHandler(func(_ pahomqtt.Client, err error) {
			onConnectionLost(err)
		})

		p.client = pahomqtt.NewClient(opts)
		return p
	}
}

// Connect は ctx の打ち切りと token timeout を同じ経路で扱い、どちらでも接続試行の完了まで
// client を手放さない。paho は Disconnect(0) で進行中の dial を中断せず、試行が CONNACK まで
// 進めば接続を一旦確立して OnConnect を起動し token をエラー無しで完了させる (issue 675) ため、
// 確立してしまった接続は paho の後片付けの完了を待ってから返す。
func (p *pahoClient) Connect(ctx context.Context) error {
	token := p.client.Connect()
	timer := time.NewTimer(p.tokenTimeout)
	defer timer.Stop()

	var abortErr error
	select {
	case <-token.Done():
		return token.Error()
	case <-ctx.Done():
		abortErr = ctx.Err()
	case <-timer.C:
		abortErr = errors.New("connection timed out")
	}

	// Disconnect より先に立てる。paho の status mutex を介して、打ち切り後に起動される OnConnect から必ず見える。
	p.aborting.Store(true)
	// status を disconnecting に落とす。試行が確立まで進んだ場合も、この Disconnect の内部 goroutine が切断する。
	// paho の後片付けは非同期で、進行中の dial は ConnectTimeout まで続く。
	p.client.Disconnect(0)
	// 試行の完了まで client を手放さない (上限は概ね ConnectTimeout × 2)。
	<-token.Done()
	if token.Error() == nil {
		// 打ち切り後に CONNACK が届いた (paho は接続を一旦確立する)。後片付けの完了を待ってから返す。
		p.client.Disconnect(abortWait)
	}
	return abortErr
}

func (p *pahoClient) Disconnect(quiesce uint) {
	p.client.Disconnect(quiesce)
}

func (p *pahoClient) Publish(topic string, qos byte, retained bool, payload string) error {
	token := p.client.Publish(topic, qos, retained, payload)
	if !token.WaitTimeout(p.tokenTimeout) {
		return errors.New("publish timed out")
	}
	return token.Error()
}

func (p *pahoClient) Subscribe(topic string, qos byte, handler domain.MessageHandler) error {
	pahoHandler := func(_ pahomqtt.Client, msg pahomqtt.Message) {
		handler(msg.Topic(), msg.Payload(), msg.Qos(), msg.Retained())
	}
	token := p.client.Subscribe(topic, qos, pahoHandler)
	if !token.WaitTimeout(p.tokenTimeout) {
		return errors.New("subscribe timed out")
	}
	return token.Error()
}

func (p *pahoClient) Unsubscribe(topics ...string) error {
	token := p.client.Unsubscribe(topics...)
	if !token.WaitTimeout(p.tokenTimeout) {
		return errors.New("unsubscribe timed out")
	}
	return token.Error()
}

func (p *pahoClient) IsConnected() bool {
	return p.client.IsConnected()
}
