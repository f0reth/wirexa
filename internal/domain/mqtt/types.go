package mqttdomain

// このパッケージの型は業務概念と不変条件を表し、Wails RPC とイベントの配線型を兼ねる
// (adapters に RPC 用の DTO は作らない)。json タグは RPC とイベントの配線形式だけを表す。
// 永続化形式は infrastructure のリポジトリにある storedXxx DTO が持つので、タグを変えても
// 保存形式は変わらない。保存対象のフィールドを足すときは stored DTO と変換関数にも足す。
// ドメイン型に載せてよいのは RPC で受け渡す業務上の値だけで、パスや一時的なハンドルのような
// infrastructure 内部の状態は載せない。

// ConnectionConfig は MQTT ブローカーへの接続設定を表す。
type ConnectionConfig struct {
	Name     string `json:"name"`
	Broker   string `json:"broker"`
	ClientID string `json:"clientId"`
	Username string `json:"username"`
	Password string `json:"password"`
	// ProfileID は接続元の BrokerProfile ID。リロード後の状態復元で接続とプロファイルを紐付ける。
	ProfileID string `json:"profileId"`
	UseTLS    bool   `json:"useTls"`
}

// SubscriptionInfo は接続が現在購読しているトピックを表す。
type SubscriptionInfo struct {
	Topic string `json:"topic"`
	QoS   byte   `json:"qos"`
}

// MQTTMessage は受信した MQTT メッセージを表す。
type MQTTMessage struct {
	ConnectionID string `json:"connectionId"`
	Topic        string `json:"topic"`
	Payload      string `json:"payload"`
	// Timestamp を byte/bool 群より前に置くのはパディングを避けるため (72B → 64B)。
	// 受信メッセージは件数が出るので、確保サイズクラスを 80 から 64 に下げておく。
	Timestamp int64 `json:"timestamp"`
	QoS       byte  `json:"qos"`
	Retained  bool  `json:"retained"`
	// PayloadBase64 は Payload が非 UTF-8 バイナリのため base64 エンコードされていることを示す。
	PayloadBase64 bool `json:"payloadBase64"`
}

// ConnectionStatus は接続の現在状態を表す。
type ConnectionStatus struct {
	ID            string             `json:"id"`
	Name          string             `json:"name"`
	Broker        string             `json:"broker"`
	ProfileID     string             `json:"profileId"`
	Subscriptions []SubscriptionInfo `json:"subscriptions"`
	Connected     bool               `json:"connected"`
}

// BrokerProfile は MQTT ブローカーへの接続プロファイルを表す。
type BrokerProfile struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Broker   string `json:"broker"`
	ClientID string `json:"clientId"`
	Username string `json:"username"`
	Password string `json:"password"`
	UseTLS   bool   `json:"useTls"`
}
