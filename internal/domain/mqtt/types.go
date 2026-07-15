package mqttdomain

// ConnectionConfig は MQTT ブローカーへの接続設定を表す。
type ConnectionConfig struct {
	Name     string `json:"name"`
	Broker   string `json:"broker"`
	ClientID string `json:"clientId"`
	Username string `json:"username"`
	Password string `json:"password"`
	UseTLS   bool   `json:"useTls"`
	// ProfileID は接続元の BrokerProfile ID。リロード後の状態復元で接続とプロファイルを紐付ける。
	ProfileID string `json:"profileId"`
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
	QoS          byte   `json:"qos"`
	Retained     bool   `json:"retained"`
	Timestamp    int64  `json:"timestamp"`
	// PayloadBase64 は Payload が非 UTF-8 バイナリのため base64 エンコードされていることを示す。
	PayloadBase64 bool `json:"payloadBase64"`
}

// ConnectionStatus は接続の現在状態を表す。
type ConnectionStatus struct {
	ID            string             `json:"id"`
	Name          string             `json:"name"`
	Broker        string             `json:"broker"`
	Connected     bool               `json:"connected"`
	ProfileID     string             `json:"profileId"`
	Subscriptions []SubscriptionInfo `json:"subscriptions"`
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
