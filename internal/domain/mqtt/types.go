package mqttdomain

// ConnectionConfig は MQTT ブローカーへの接続設定を表す。
type ConnectionConfig struct {
	Name     string `json:"name"`
	Broker   string `json:"broker"`
	ClientID string `json:"clientId"`
	Username string `json:"username"`
	Password string `json:"password"`
	UseTLS   bool   `json:"useTls"`
}

// MqttMessage は受信した MQTT メッセージを表す。
type MqttMessage struct {
	ConnectionID string `json:"connectionId"`
	Topic        string `json:"topic"`
	Payload      string `json:"payload"`
	QoS          byte   `json:"qos"`
	Retained     bool   `json:"retained"`
	Timestamp    int64  `json:"timestamp"`
}

// ConnectionStatus は接続の現在状態を表す。
type ConnectionStatus struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Broker    string `json:"broker"`
	Connected bool   `json:"connected"`
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
