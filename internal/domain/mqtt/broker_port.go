// Package mqttdomain は MQTT ドメイン層のポートインターフェースを定義する。
package mqttdomain

import "context"

// MessageHandler はサブスクライブしたトピックのメッセージ受信時に呼ばれるコールバック。
// payload は生バイト列で渡す（バイナリペイロードを application 層まで保持するため）。
type MessageHandler func(topic string, payload []byte, qos byte, retained bool)

// BrokerClient は MQTT ブローカー接続のトランスポート抽象。
// 実装は infrastructure 層 (例: Paho) が提供する。
type BrokerClient interface {
	// Connect はブローカーへの接続を試み、成功・失敗・打ち切りのいずれかまでブロックする。
	//   - ctx が切れたとき、または実装固有の上限時間を超えたときはエラーを返すが、
	//     接続試行が終わるまでは返らない。
	//   - エラーを返した時点で、client は接続を維持していない
	//     (未確立なら以後確立せず、試行と競合して確立していた場合は切断済み)。
	//   - 打ち切り (ctx・上限時間) を始めた後は、試行が確立まで進んでも onConnected を呼ばない。
	//   - 成功を返したときだけ、呼び出し元が client を切断する責任を持つ。
	Connect(ctx context.Context) error
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
