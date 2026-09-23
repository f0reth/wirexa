// Package mqttdomain は MQTT ドメイン層のポートインターフェースを定義する。
package mqttdomain

// ProfileRepository は MQTT ブローカープロファイルの永続化ポート。
type ProfileRepository interface {
	Load() ([]BrokerProfile, error)
	Save(profile *BrokerProfile) error
	Delete(id string) error
}
