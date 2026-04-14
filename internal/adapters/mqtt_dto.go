package adapters

import mqttdomain "github.com/f0reth/Wirexa/internal/domain/mqtt"

// ConnectionConfig は MQTT 接続設定の RPC 転送型。
type ConnectionConfig struct {
	Name     string `json:"name"`
	Broker   string `json:"broker"`
	ClientID string `json:"clientId"`
	Username string `json:"username"`
	Password string `json:"password"`
	UseTLS   bool   `json:"useTls"`
}

// ConnectionStatus は MQTT 接続状態の RPC 転送型。
type ConnectionStatus struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Broker    string `json:"broker"`
	Connected bool   `json:"connected"`
}

// BrokerProfile は MQTT ブローカープロファイルの RPC 転送型。
type BrokerProfile struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Broker   string `json:"broker"`
	ClientID string `json:"clientId"`
	Username string `json:"username"`
	Password string `json:"password"`
	UseTLS   bool   `json:"useTls"`
}

func fromConnectionConfigDTO(cfg ConnectionConfig) mqttdomain.ConnectionConfig {
	return mqttdomain.ConnectionConfig{
		Name:     cfg.Name,
		Broker:   cfg.Broker,
		ClientID: cfg.ClientID,
		Username: cfg.Username,
		Password: cfg.Password,
		UseTLS:   cfg.UseTLS,
	}
}

func toConnectionStatusDTO(s mqttdomain.ConnectionStatus) ConnectionStatus {
	return ConnectionStatus{ID: s.ID, Name: s.Name, Broker: s.Broker, Connected: s.Connected}
}

func fromBrokerProfileDTO(p BrokerProfile) mqttdomain.BrokerProfile {
	return mqttdomain.BrokerProfile{
		ID:       p.ID,
		Name:     p.Name,
		Broker:   p.Broker,
		ClientID: p.ClientID,
		Username: p.Username,
		Password: p.Password,
		UseTLS:   p.UseTLS,
	}
}

func toBrokerProfileDTO(p mqttdomain.BrokerProfile) BrokerProfile {
	return BrokerProfile{
		ID:       p.ID,
		Name:     p.Name,
		Broker:   p.Broker,
		ClientID: p.ClientID,
		Username: p.Username,
		Password: p.Password,
		UseTLS:   p.UseTLS,
	}
}
