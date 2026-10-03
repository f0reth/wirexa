// Package mqttinfra は MQTT インフラ層のアダプターを提供する。
package mqttinfra

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	pahomqtt "github.com/eclipse/paho.mqtt.golang"

	domain "github.com/f0reth/Wirexa/internal/domain/mqtt"
)

const (
	defaultConnectTimeout = 10 * time.Second
	defaultTokenTimeout   = 30 * time.Second
)

// subackFailure は SUBACK の失敗を表す戻りコード (MQTT 3.1.1 3.9.3)。MQTT 5 の理由コードも 0x80 以上が失敗。
const subackFailure = 0x80

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
	// routes は購読中のフィルターとハンドラ (購読した順)。paho のルーターには登録せず、ここで振り分ける。
	// paho は一致する全ての購読のハンドラを呼ぶので、重なる購読 (例: # と a/b) があると
	// 1 件の受信が購読の数だけ届いてしまう。
	routes   []route
	routesMu sync.RWMutex
}

// route は購読中のフィルターとそのハンドラ。
type route struct {
	handler domain.MessageHandler
	filter  string
}

// filterMatches は受信したトピックが購読のフィルターに一致するかを返す。
// 共有購読の接頭辞 ($share/<group>/ と $queue/) は paho のルーターと同じく外して照合する。
func filterMatches(filter, topic string) bool {
	if strings.HasPrefix(filter, "$share/") {
		if parts := strings.SplitN(filter, "/", 3); len(parts) == 3 {
			filter = parts[2]
		}
	} else {
		filter = strings.TrimPrefix(filter, "$queue/")
	}
	filterLevels := strings.Split(filter, "/")
	topicLevels := strings.Split(topic, "/")
	for i, f := range filterLevels {
		// # は残りの階層すべてに一致する (a/# は a にも一致する)。
		if f == "#" {
			return true
		}
		if i >= len(topicLevels) || (f != "+" && f != topicLevels[i]) {
			return false
		}
	}
	return len(filterLevels) == len(topicLevels)
}

// setRoute は filter のハンドラを登録する。登録済みなら置き換えて true を返す。
func (p *pahoClient) setRoute(filter string, handler domain.MessageHandler) (replaced bool) {
	p.routesMu.Lock()
	defer p.routesMu.Unlock()
	for i := range p.routes {
		if p.routes[i].filter == filter {
			p.routes[i].handler = handler
			return true
		}
	}
	p.routes = append(p.routes, route{filter: filter, handler: handler})
	return false
}

// removeRoute は filter のハンドラの登録を外す。
func (p *pahoClient) removeRoute(filter string) {
	p.routesMu.Lock()
	defer p.routesMu.Unlock()
	p.routes = slices.DeleteFunc(p.routes, func(r route) bool {
		return r.filter == filter
	})
}

// dispatch は受信した 1 件のメッセージを、一致する購読のうち最初に購読したもののハンドラへ 1 回だけ渡す。
// 一致する購読が無いメッセージ (購読解除と行き違いで届いたもの) は捨てる。
func (p *pahoClient) dispatch(msg pahomqtt.Message) {
	var handler domain.MessageHandler
	p.routesMu.RLock()
	for _, r := range p.routes {
		if filterMatches(r.filter, msg.Topic()) {
			handler = r.handler
			break
		}
	}
	p.routesMu.RUnlock()
	if handler != nil {
		handler(msg.Topic(), msg.Payload(), msg.Qos(), msg.Retained())
	}
}

// applyTLSScheme は UseTLS=true の場合、Broker URL のスキームを TLS 対応のものに変換する。
// paho はスキームを小文字にして解釈し、スキーム無しには tcp:// を補うので、大文字を含むスキームと
// スキーム無しをそのまま渡すと平文で接続する。照合は大文字小文字を区別せず、スキーム無しには ssl:// を補う。
// TLS にできないスキームはそのまま返す (MQTTService.Connect が domain.ValidateBroker で先に拒否する)。
func applyTLSScheme(broker string) string {
	if broker == "" {
		return broker
	}
	scheme, rest, ok := strings.Cut(broker, "://")
	if !ok {
		return "ssl://" + broker
	}
	switch strings.ToLower(scheme) {
	case "tcp":
		return "ssl://" + rest
	case "mqtt":
		return "mqtts://" + rest
	case "ws":
		return "wss://" + rest
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
		// 受信したメッセージは全てここに届く (Subscribe が paho のルーターに登録しないため)。
		opts.SetDefaultPublishHandler(func(_ pahomqtt.Client, msg pahomqtt.Message) {
			p.dispatch(msg)
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
	// 自動再接続中の paho は QoS 0 を送らずに成功させ、QoS 1/2 を保存して再接続後に送る。
	// 呼び出し元に返した結果と食い違うので、接続が開いていなければ送らない。
	// 確認の直後に切断された場合までは防げない (起きる窓を狭めるだけ)。
	if !p.client.IsConnectionOpen() {
		return errors.New("not connected")
	}
	token := p.client.Publish(topic, qos, retained, payload)
	if !token.WaitTimeout(p.tokenTimeout) {
		// paho のストアに保存されたメッセージは取り消せないので、再接続後に送られ得る。
		return fmt.Errorf("publish was not acknowledged in time (it may still be delivered after reconnecting): %w", domain.ErrAckTimeout)
	}
	return token.Error()
}

func (p *pahoClient) Subscribe(topic string, qos byte, handler domain.MessageHandler) error {
	// SUBACK の直後に届くメッセージ (retained など) を取りこぼさないよう、送信前に登録する。
	replaced := p.setRoute(topic, handler)
	err := p.subscribe(topic, qos)
	// 新しく足した購読が成立しなかったら登録を外す。登録済みのフィルターの張り直しが失敗したときは
	// 登録を残す (接続中の QoS 変更が拒否された場合、元の購読は続いているため)。
	// 再接続時の張り直しを拒否された購読の後始末は、呼び出し側が Unsubscribe で行う。
	if err != nil && !replaced {
		p.removeRoute(topic)
	}
	return err
}

// subscribe は SUBSCRIBE を送って SUBACK を待つ。callback を渡さないので paho のルーターには登録されず、
// 受信したメッセージは既定ハンドラ (dispatch) に 1 回だけ届く。
func (p *pahoClient) subscribe(topic string, qos byte) error {
	token := p.client.Subscribe(topic, qos, nil)
	if !token.WaitTimeout(p.tokenTimeout) {
		return fmt.Errorf("subscribe was not acknowledged in time: %w", domain.ErrAckTimeout)
	}
	if err := token.Error(); err != nil {
		return err
	}
	// paho はブローカーが SUBACK で拒否 (0x80) しても token をエラーにしないので、結果コードを見る。
	if st, ok := token.(*pahomqtt.SubscribeToken); ok {
		for _, code := range st.Result() {
			if code >= subackFailure {
				return domain.ErrSubscriptionRejected
			}
		}
	}
	return nil
}

func (p *pahoClient) Unsubscribe(topic string) error {
	token := p.client.Unsubscribe(topic)
	if !token.WaitTimeout(p.tokenTimeout) {
		// ブローカーが解除したかどうかは分からない。解除していた場合に購読中のまま届かない状態を
		// 残さないよう、解除した側に倒して振り分け先を外す。ブローカー側に購読が残っていても、
		// 届いたメッセージは dispatch が捨てる。
		p.removeRoute(topic)
		return fmt.Errorf("unsubscribe was not acknowledged in time: %w", domain.ErrAckTimeout)
	}
	if err := token.Error(); err != nil {
		return err
	}
	p.removeRoute(topic)
	return nil
}

// IsConnected は接続が開いているかを返す。paho の IsConnected は自動再接続中も true を返すので、
// 接続が切れている間も接続中と表示されないよう IsConnectionOpen を使う。
func (p *pahoClient) IsConnected() bool {
	return p.client.IsConnectionOpen()
}
