const DEFAULT_PORT_MAP: Record<string, string> = {
  mqtt: "1883",
  mqtts: "8883",
  tcp: "1883",
  ws: "9001",
  wss: "8884",
};

export function defaultPort(scheme: string): string {
  return DEFAULT_PORT_MAP[scheme] ?? "1883";
}

export function parseBrokerUrl(url: string): {
  scheme: string;
  host: string;
  port: string;
} {
  const match = url.match(/^(mqtt|mqtts|tcp|ws|wss):\/\/([^:]+)(?::(\d+))?$/);
  if (!match) return { scheme: "mqtt", host: "localhost", port: "1883" };
  return {
    scheme: match[1],
    host: match[2],
    port: match[3] ?? defaultPort(match[1]),
  };
}

export function composeBrokerUrl(
  scheme: string,
  host: string,
  port: string,
): string {
  return `${scheme}://${host}:${port}`;
}
