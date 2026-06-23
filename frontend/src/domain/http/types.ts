// フロントエンド domain 層の型定義（ユニオン型・型ガードで UI に意味付けする層）。
// 配線型の正は Go ドメイン型 internal/domain/http/types.go で、これが RPC 境界に直接公開され
// Wails 生成型 wailsjs/go/models.ts に反映される。Go 側を変更したら `wails generate module` を
// 実行する（再生成忘れは CI のバインディング鮮度チェックが検知する）。
// 生成型 → この型の変換は infrastructure/http/client.ts でスプレッド素通し＋ユニオン検証で行う。

export interface KeyValuePair {
  key: string;
  value: string;
  enabled: boolean;
}

export interface RequestBody {
  type: "none" | "json" | "text" | "form-urlencoded" | "form-data" | "file";
  contents: Partial<
    Record<
      "none" | "json" | "text" | "form-urlencoded" | "form-data" | "file",
      string
    >
  >;
}

export type AuthType = "none" | "basic" | "bearer";
// AuthType ユニオンに値を追加して下のレコードを更新し忘れると satisfies がコンパイルエラーになる。
const AUTH_TYPE_SET = {
  none: true,
  basic: true,
  bearer: true,
} satisfies Record<AuthType, true>;
export const AUTH_TYPES = Object.keys(AUTH_TYPE_SET) as AuthType[];

export interface RequestAuth {
  type: AuthType;
  username: string;
  password: string;
  token: string;
}

export type ProxyMode = "system" | "none" | "custom";

export interface RequestSettings {
  timeoutSec: number; // 0 = default (30s)
  proxyMode: ProxyMode; // default: "none"
  proxyURL: string; // used when proxyMode == "custom"
  insecureSkipVerify: boolean;
  disableRedirects: boolean;
  maxResponseBodyMB: number; // 0 = default (10MB)
}

export const DEFAULT_SETTINGS: RequestSettings = {
  timeoutSec: 120,
  proxyMode: "none",
  proxyURL: "",
  insecureSkipVerify: true,
  disableRedirects: false,
  maxResponseBodyMB: 10,
};

export interface HttpRequest {
  id: string;
  name: string;
  method: HttpMethod;
  url: string;
  headers: KeyValuePair[];
  params: KeyValuePair[];
  body: RequestBody;
  auth: RequestAuth;
  settings: RequestSettings;
  doc: string;
}

export interface HttpResponse {
  statusCode: number;
  statusText: string;
  headers: Record<string, string>;
  body: string;
  contentType: string;
  size: number;
  timingMs: number;
  error: string;
  bodyTruncated: boolean;
  tempFilePath: string;
}

export interface Collection {
  id: string;
  name: string;
  items: TreeItem[];
  order: number;
}

export interface TreeItem {
  type: "folder" | "request";
  id: string;
  name: string;
  children: TreeItem[];
  request?: HttpRequest;
}

// ルートリクエスト置き場として使用する予約済みコレクション ID。
export const ROOT_COLLECTION_ID = "__root__";

export type SidebarEntry = { kind: "collection" | "item"; id: string };

export type HttpMethod =
  | "GET"
  | "POST"
  | "PUT"
  | "PATCH"
  | "DELETE"
  | "HEAD"
  | "OPTIONS";
// HttpMethod ユニオンに値を追加して下のレコードを更新し忘れると satisfies がコンパイルエラーになる。
const HTTP_METHOD_SET = {
  GET: true,
  POST: true,
  PUT: true,
  PATCH: true,
  DELETE: true,
  HEAD: true,
  OPTIONS: true,
} satisfies Record<HttpMethod, true>;
export const HTTP_METHODS = Object.keys(HTTP_METHOD_SET) as HttpMethod[];

export type BodyType = RequestBody["type"];
// BodyType ユニオンに値を追加して下のレコードを更新し忘れると satisfies がコンパイルエラーになる。
const BODY_TYPE_SET = {
  none: true,
  json: true,
  text: true,
  "form-urlencoded": true,
  "form-data": true,
  file: true,
} satisfies Record<BodyType, true>;
export const BODY_TYPES = Object.keys(BODY_TYPE_SET) as BodyType[];

export function isHttpMethod(v: string): v is HttpMethod {
  return (HTTP_METHODS as string[]).includes(v);
}

export function isBodyType(v: string): v is BodyType {
  return (BODY_TYPES as string[]).includes(v);
}

export function isAuthType(v: string): v is AuthType {
  return (AUTH_TYPES as string[]).includes(v);
}
