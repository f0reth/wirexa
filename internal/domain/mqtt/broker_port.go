package mqttdomain

import (
	"context"
	"errors"
)

// ErrAckTimeout はブローカーの応答 (PUBACK・SUBACK・UNSUBACK など) を時間内に確認できなかったことを表す。
// ブローカーが要求を処理したかどうかは分からない。
var ErrAckTimeout = errors.New("no acknowledgement from broker in time")

// ErrSubscriptionRejected はブローカーが購読を拒否した (SUBACK が失敗コードを返した) ことを表す。
var ErrSubscriptionRejected = errors.New("subscription rejected by broker")

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
	// Publish は指定トピックへメッセージを送信する。接続が開いていなければ (自動再接続中を含む)
	// 送らずにエラーを返す。ブローカーの応答を時間内に確認できなければ ErrAckTimeout を返す
	// (メッセージは再接続後に届き得る)。
	Publish(topic string, qos byte, retained bool, payload string) error
	// Subscribe は指定トピックパターンの購読を開始し、受信時に handler を呼ぶ。
	// ブローカーが購読を拒否した場合は ErrSubscriptionRejected、応答を時間内に確認できなければ
	// ErrAckTimeout を返す。
	// 受信した 1 件のメッセージが複数の購読に一致しても、handler を呼ぶのは一致する購読の
	// うち 1 つだけ (メッセージ 1 件につき 1 回)。
	Subscribe(topic string, qos byte, handler MessageHandler) error
	// Unsubscribe は指定トピックの購読を解除する。ブローカーの応答を時間内に確認できなければ
	// ErrAckTimeout を返すが、そのときも解除したものとして扱い、以後そのトピックの handler は呼ばない。
	Unsubscribe(topic string) error
	// IsConnected は現在接続中かどうかを返す。接続が切れて自動再接続を試みている間は false。
	IsConnected() bool
}

// BrokerClientFactory は ConnectionConfig からブローカークライアントを生成するファクトリ。
// onConnected は接続確立（自動再接続を含む）のたびに呼ばれる。client の操作 (Subscribe など) の
// 完了を待ってよい goroutine から呼び、Connect の呼び出し元の goroutine では呼ばない。
// onConnectionLost は確立済み接続が予期せず切れたときに呼ばれる。onConnectionLost から client の
// Disconnect を呼んでよく、呼ぶと自動再接続は止まる。
type BrokerClientFactory func(
	config ConnectionConfig,
	onConnected func(),
	onConnectionLost func(error),
) BrokerClient
