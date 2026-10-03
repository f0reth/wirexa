// Code generated from internal/domain/events.go by tools/gen-events; DO NOT EDIT.

export const WailsEvents = {
  mqttConnected: "mqtt:connected",
  mqttDisconnected: "mqtt:disconnected",
  mqttConnectionLost: "mqtt:connection-lost",
  mqttConnectionFailed: "mqtt:connection-failed",
  mqttMessage: "mqtt:message",
  mqttScanTopic: "mqtt:scan-topic",
  mqttScanStopped: "mqtt:scan-stopped",
  mqttSubscriptionDropped: "mqtt:subscription-dropped",
  udpMessage: "udp:message",
  appBeforeClose: "app:before-close",
} as const;

export type MqttEventName =
  | "mqtt:connected"
  | "mqtt:disconnected"
  | "mqtt:connection-lost"
  | "mqtt:connection-failed"
  | "mqtt:message"
  | "mqtt:scan-topic"
  | "mqtt:scan-stopped"
  | "mqtt:subscription-dropped";
