// Code generated from internal/domain/events.go by tools/gen-events; DO NOT EDIT.

export const WailsEvents = {
  mqttConnected: "mqtt:connected",
  mqttDisconnected: "mqtt:disconnected",
  mqttConnectionLost: "mqtt:connection-lost",
  mqttConnectionFailed: "mqtt:connection-failed",
  mqttMessage: "mqtt:message",
  udpMessage: "udp:message",
  appBeforeClose: "app:before-close",
} as const;

export type MqttEventName =
  | "mqtt:connected"
  | "mqtt:disconnected"
  | "mqtt:connection-lost"
  | "mqtt:connection-failed"
  | "mqtt:message";
