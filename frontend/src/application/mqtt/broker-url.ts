// ブローカー URL のスキーム。入力欄の選択肢、既定のポート、parseBrokerUrl の正規表現はここから作る。
export const BROKER_SCHEMES = ["mqtt", "mqtts", "tcp", "ws", "wss"] as const;

type BrokerScheme = (typeof BROKER_SCHEMES)[number];

const DEFAULT_PORT_MAP: Record<BrokerScheme, string> = {
  mqtt: "1883",
  mqtts: "8883",
  tcp: "1883",
  ws: "9001",
  wss: "8884",
};

function isBrokerScheme(scheme: string): scheme is BrokerScheme {
  return (BROKER_SCHEMES as readonly string[]).includes(scheme);
}

export function defaultPort(scheme: string): string {
  return isBrokerScheme(scheme) ? DEFAULT_PORT_MAP[scheme] : "1883";
}

interface BrokerUrlParts {
  scheme: string;
  host: string;
  port: string;
}

// 新規プロファイルの初期値。読めない URL もこの値として扱う。
const DEFAULT_BROKER: Readonly<BrokerUrlParts> = {
  scheme: "mqtt",
  host: "localhost",
  port: "1883",
};

export const DEFAULT_BROKER_URL = composeBrokerUrl(
  DEFAULT_BROKER.scheme,
  DEFAULT_BROKER.host,
  DEFAULT_BROKER.port,
);

const BROKER_URL_PATTERN = new RegExp(
  `^(${BROKER_SCHEMES.join("|")})://([^:]+)(?::(\\d+))?$`,
);

// 読めない URL は null。保存する前に、読み戻せる URL かを確かめるのに使う。
export function tryParseBrokerUrl(url: string): BrokerUrlParts | null {
  const match = url.match(BROKER_URL_PATTERN);
  if (!match) return null;
  return {
    scheme: match[1],
    host: match[2],
    port: match[3] ?? defaultPort(match[1]),
  };
}

// 表示用。読めない URL は既定値で埋める。
export function parseBrokerUrl(url: string): BrokerUrlParts {
  return tryParseBrokerUrl(url) ?? { ...DEFAULT_BROKER };
}

export function composeBrokerUrl(
  scheme: string,
  host: string,
  port: string,
): string {
  return `${scheme}://${host}:${port}`;
}
