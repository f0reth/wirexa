// Package mqttdomain は MQTT ドメイン層のポートインターフェースを定義する。
package mqttdomain

// MQTTUseCase は MQTT 接続管理のユースケース入力ポート。
// Wails RPC アダプター (ports 層) はこのインターフェースのみに依存する。
type MQTTUseCase interface {
	Connect(config ConnectionConfig) (string, error)
	Disconnect(connectionID string) error
	Publish(connectionID, topic, payload string, qos byte, retain bool) error
	Subscribe(connectionID, topic string, qos byte) error
	Unsubscribe(connectionID, topic string) error
	GetConnections() []ConnectionStatus
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
	// SaveProfile は保存済みプロファイルを返す。新規作成 (空 ID) では
	// サーバ側で採番した ID が入るため、呼び出し元は戻り値を使う。
	SaveProfile(profile BrokerProfile) (BrokerProfile, error)
	DeleteProfile(id string) error
}
