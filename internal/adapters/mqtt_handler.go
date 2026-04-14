// Package adapters は Wails RPC アダプター層を提供する。
package adapters

import (
	mqttdomain "github.com/f0reth/Wirexa/internal/domain/mqtt"
)

// MqttHandler は Wails RPC アダプターとして MQTT ユースケースを公開する。
type MqttHandler struct {
	svc        mqttdomain.MqttUseCase
	profileSvc mqttdomain.ProfileUseCase
}

// SetupMqttHandler は既存の MqttHandler インスタンスにサービスを注入する。
// Wails の Bind に渡す前に事前確保した空ハンドラーを startup() で初期化する際に使用する。
func SetupMqttHandler(h *MqttHandler, svc mqttdomain.MqttUseCase, profileSvc mqttdomain.ProfileUseCase) {
	h.svc = svc
	h.profileSvc = profileSvc
}

// Connect は MQTT ブローカーへ接続し、接続 ID を返す。
func (h *MqttHandler) Connect(config ConnectionConfig) (string, error) {
	return h.svc.Connect(fromConnectionConfigDTO(config))
}

// Disconnect は指定した接続を切断する。
func (h *MqttHandler) Disconnect(connectionID string) error {
	return h.svc.Disconnect(connectionID)
}

// Publish は指定トピックへメッセージを送信する。
func (h *MqttHandler) Publish(connectionID, topic, payload string, qos byte, retain bool) error {
	return h.svc.Publish(connectionID, topic, payload, qos, retain)
}

// Subscribe は指定トピックの購読を開始する。
func (h *MqttHandler) Subscribe(connectionID, topic string, qos byte) error {
	return h.svc.Subscribe(connectionID, topic, qos)
}

// Unsubscribe は指定トピックの購読を解除する。
func (h *MqttHandler) Unsubscribe(connectionID, topic string) error {
	return h.svc.Unsubscribe(connectionID, topic)
}

// GetConnections は全接続の現在状態を返す。
func (h *MqttHandler) GetConnections() []ConnectionStatus {
	statuses := h.svc.GetConnections()
	result := make([]ConnectionStatus, len(statuses))
	for i, s := range statuses {
		result[i] = toConnectionStatusDTO(s)
	}
	return result
}

// Shutdown は全接続を切断してサービスを終了する。
func (h *MqttHandler) Shutdown() {
	h.svc.Shutdown()
}

// GetProfiles は全 MQTT ブローカープロファイルを返す。
func (h *MqttHandler) GetProfiles() []BrokerProfile {
	profiles := h.profileSvc.GetProfiles()
	result := make([]BrokerProfile, len(profiles))
	for i, p := range profiles {
		result[i] = toBrokerProfileDTO(p)
	}
	return result
}

// SaveProfile は MQTT ブローカープロファイルを保存する。
func (h *MqttHandler) SaveProfile(profile BrokerProfile) error {
	return h.profileSvc.SaveProfile(fromBrokerProfileDTO(profile))
}

// DeleteProfile は MQTT ブローカープロファイルを削除する。
func (h *MqttHandler) DeleteProfile(id string) error {
	return h.profileSvc.DeleteProfile(id)
}
