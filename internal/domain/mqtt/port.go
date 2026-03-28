// Package mqttdomain は MQTT ドメイン層のポートインターフェースを定義する。
package mqttdomain

// EventEmitter はフロントエンドへのイベント通知の抽象。
// Wails に依存しないため、テスト時はモックに差し替え可能。
type EventEmitter interface {
	Emit(event string, data any)
}

// MqttUseCase は MQTT 接続管理のユースケース入力ポート。
// Wails RPC アダプター (ports 層) はこのインターフェースのみに依存する。
type MqttUseCase interface {
	Connect(config ConnectionConfig) (string, error)
	Disconnect(connectionID string) error
	Publish(connectionID, topic, payload string, qos byte, retain bool) error
	Subscribe(connectionID, topic string, qos byte) error
	Unsubscribe(connectionID, topic string) error
	GetConnections() []ConnectionStatus
	Shutdown()
}

// ProfileRepository は MQTT ブローカープロファイルの永続化ポート。
type ProfileRepository interface {
	Load() ([]BrokerProfile, error)
	Save(profile *BrokerProfile) error
	Delete(id string) error
}

// ProfileUseCase は MQTT プロファイル管理のユースケース入力ポート。
type ProfileUseCase interface {
	GetProfiles() []BrokerProfile
	SaveProfile(profile BrokerProfile) error
	DeleteProfile(id string) error
}
