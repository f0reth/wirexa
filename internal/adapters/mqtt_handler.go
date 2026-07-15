// Package adapters は Wails RPC アダプター層を提供する。
package adapters

import (
	mqttdomain "github.com/f0reth/Wirexa/internal/domain/mqtt"
)

// MQTTHandler は Wails RPC アダプターとして MQTT ユースケースを公開する。
type MQTTHandler struct {
	svc        mqttdomain.MQTTUseCase
	profileSvc mqttdomain.ProfileUseCase
}

// SetupMQTTHandler は既存の MQTTHandler インスタンスにサービスを注入する。
// Wails の Bind に渡す前に事前確保した空ハンドラーを startup() で初期化する際に使用する。
func SetupMQTTHandler(h *MQTTHandler, svc mqttdomain.MQTTUseCase, profileSvc mqttdomain.ProfileUseCase) {
	h.svc = svc
	h.profileSvc = profileSvc
}

// Connect は MQTT ブローカーへ接続し、接続 ID を返す。
func (h *MQTTHandler) Connect(config mqttdomain.ConnectionConfig) (string, error) {
	return h.svc.Connect(config)
}

// Disconnect は指定した接続を切断する。
func (h *MQTTHandler) Disconnect(connectionID string) error {
	return h.svc.Disconnect(connectionID)
}

// Publish は指定トピックへメッセージを送信する。
func (h *MQTTHandler) Publish(connectionID, topic, payload string, qos byte, retain bool) error {
	return h.svc.Publish(connectionID, topic, payload, qos, retain)
}

// Subscribe は指定トピックの購読を開始する。
func (h *MQTTHandler) Subscribe(connectionID, topic string, qos byte) error {
	return h.svc.Subscribe(connectionID, topic, qos)
}

// Unsubscribe は指定トピックの購読を解除する。
func (h *MQTTHandler) Unsubscribe(connectionID, topic string) error {
	return h.svc.Unsubscribe(connectionID, topic)
}

// GetConnections は全接続の現在状態を返す。
func (h *MQTTHandler) GetConnections() []mqttdomain.ConnectionStatus {
	return h.svc.GetConnections()
}

// Shutdown は全接続を切断してサービスを終了する。
func (h *MQTTHandler) Shutdown() {
	h.svc.Shutdown()
}

// GetProfiles は全 MQTT ブローカープロファイルを返す。
func (h *MQTTHandler) GetProfiles() []mqttdomain.BrokerProfile {
	return h.profileSvc.GetProfiles()
}

// SaveProfile は MQTT ブローカープロファイルを保存する。
func (h *MQTTHandler) SaveProfile(profile mqttdomain.BrokerProfile) error {
	return h.profileSvc.SaveProfile(profile)
}

// DeleteProfile は MQTT ブローカープロファイルを削除する。
func (h *MQTTHandler) DeleteProfile(id string) error {
	return h.profileSvc.DeleteProfile(id)
}
