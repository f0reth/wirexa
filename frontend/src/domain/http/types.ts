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

// form 系ボディの 1 行。Headers/Params の KeyValuePair と分けているのは、
// 値の種別とパートごとの Content-Type がヘッダー行には無意味なため。
export interface FormRow {
  key: string;
  value: string;
  // 未設定は "text"（kind 導入前に保存された行）。
  kind?: FormRowKind;
  // file の送信元パス。value と分けて持つので kind を往復しても入力が消えない。
  filePath?: string;
  // 空なら送信時に kind から自動決定する。
  contentType?: string;
  enabled: boolean;
}

// form 系ボディの行は contents の文字列ではなく専用フィールドで保持する。
// 文字列を正にすると空キー行・無効行を表現できず編集中に行が消えるため、
// ワイヤ形式（urlencoded / multipart）は Go 側が送信時に生成する。
export interface RequestBody {
  type: "none" | "json" | "text" | "form-urlencoded" | "form-data" | "file";
  contents: Partial<
    Record<
      "none" | "json" | "text" | "form-urlencoded" | "form-data" | "file",
      string
    >
  >;
  formData?: FormRow[];
  formUrlEncoded?: FormRow[];
}

// body type と行フィールドの対応。form 系以外は行を持たない。
export const FORM_PAIR_FIELDS = {
  "form-data": "formData",
  "form-urlencoded": "formUrlEncoded",
} as const satisfies Partial<Record<BodyType, keyof RequestBody>>;

export type FormBodyType = keyof typeof FORM_PAIR_FIELDS;

export function isFormBodyType(v: BodyType): v is FormBodyType {
  return v in FORM_PAIR_FIELDS;
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
  // Set-Cookie など複数値ヘッダーを落とさないよう、キーごとに全値を保持する。
  headers: Record<string, string[]>;
  body: string;
  contentType: string;
  // 実際に受信したバイト数。bodyCapped の場合はレスポンス全長と一致しない。
  size: number;
  timingMs: number;
  error: string;
  bodyTruncated: boolean;
  tempFilePath: string;
  bodyBase64: boolean;
  // 絶対上限に達して受信を打ち切った。tempFilePath のファイルも全文ではない。
  bodyCapped: boolean;
}

export interface Collection {
  id: string;
  name: string;
  items: TreeItem[];
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

export type FormRowKind = "text" | "json" | "file";
// FormRowKind ユニオンに値を追加して下のレコードを更新し忘れると satisfies がコンパイルエラーになる。
const FORM_ROW_KIND_SET = {
  text: true,
  json: true,
  file: true,
} satisfies Record<FormRowKind, true>;
export const FORM_ROW_KINDS = Object.keys(FORM_ROW_KIND_SET) as FormRowKind[];

export function isFormRowKind(v: string): v is FormRowKind {
  return (FORM_ROW_KINDS as string[]).includes(v);
}

// file は multipart のパートでしか表現できないため form-data 行にだけ許す。
export const FORM_ROW_KINDS_BY_BODY_TYPE = {
  "form-data": FORM_ROW_KINDS,
  "form-urlencoded": ["text", "json"],
} as const satisfies Record<FormBodyType, readonly FormRowKind[]>;

export function isAuthType(v: string): v is AuthType {
  return (AUTH_TYPES as string[]).includes(v);
}
