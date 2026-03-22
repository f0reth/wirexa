// Package mqttdomain は MQTT ドメイン層のポートインターフェースを定義する。
package mqttdomain

// MessageHandler はサブスクライブしたトピックのメッセージ受信時に呼ばれるコールバック。
type MessageHandler func(topic, payload string, qos byte, retained bool)

// BrokerClient は MQTT ブローカー接続のトランスポート抽象。
// 実装は infrastructure 層 (例: Paho) が提供する。
type BrokerClient interface {
	// Connect はブローカーへの接続を開始し、成功 or 失敗までブロックする。
	Connect() error
	// Disconnect は接続を閉じる。quiesce はミリ秒単位の待機時間。
	Disconnect(quiesce uint)
	// Publish は指定トピックへメッセージを送信する。
	Publish(topic string, qos byte, retained bool, payload string) error
	// Subscribe は指定トピックパターンの購読を開始し、受信時に handler を呼ぶ。
	Subscribe(topic string, qos byte, handler MessageHandler) error
	// Unsubscribe は指定トピックの購読を解除する。
	Unsubscribe(topics ...string) error
	// IsConnected は現在接続中かどうかを返す。
	IsConnected() bool
}

// BrokerClientFactory は ConnectionConfig からブローカークライアントを生成するファクトリ。
// onConnected は接続確立（自動再接続を含む）のたびに呼ばれる。
// onConnectionLost は確立済み接続が予期せず切れたときに呼ばれる。
type BrokerClientFactory func(
	config ConnectionConfig,
	onConnected func(),
	onConnectionLost func(error),
) BrokerClient
