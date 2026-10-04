package mqttdomain

// このパッケージの型は Wails RPC とイベントの配線型を兼ねる。
// 永続化形式との境界は AGENTS.md の「Boundaries between domain types, RPC and persistence」に従う。

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
	// Scanning は Broker Topics のスキャンが稼働中かを表す。スキャンは専用の接続で行うので、
	// Subscriptions には現れない。
	Scanning bool `json:"scanning"`
}

// ScannedTopic は Broker Topics のスキャンで見つかったトピックを表す (mqtt:scan-topic のペイロード)。
type ScannedTopic struct {
	ConnectionID string `json:"connectionId"`
	Topic        string `json:"topic"`
}

// SubscriptionDropped は張り直し (接続確立時・再接続時) に失敗して外した購読を表す
// (mqtt:subscription-dropped のペイロード)。
type SubscriptionDropped struct {
	ConnectionID string `json:"connectionId"`
	Topic        string `json:"topic"`
	Error        string `json:"error"`
}

// ConnectionEvent はエラーを伴わない接続のライフサイクルイベント (mqtt:connected・mqtt:disconnected) の
// ペイロードを表す。
type ConnectionEvent struct {
	ConnectionID string `json:"connectionId"`
}

// ConnectionErrorEvent はエラーを伴う接続のライフサイクルイベント (mqtt:connection-failed・
// mqtt:connection-lost・mqtt:scan-stopped) のペイロードを表す。
// Error に omitempty を付けない。文言が空でも error キーを残し、配線形式を変えないため。
type ConnectionErrorEvent struct {
	ConnectionID string `json:"connectionId"`
	Error        string `json:"error"`
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
