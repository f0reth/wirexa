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

// request file の参照。実パスは持たない。token は backend がファイルダイアログの
// 選択結果に発行したもので、現在のセッションでだけ有効（永続化されない）。
export interface FileReference {
  token?: string;
  // 表示用の basename。
  name?: string;
  // 選択時に推定した表示用の Content-Type。
  contentType?: string;
  // 保存済み・移行済みの参照で、送信前にファイルの再選択が必要。
  needsReselect?: boolean;
  // 入力欄に入力・ペーストされた文字列。ダイアログの初期位置にだけ使う frontend 専用の値で、
  // 許可にはならない。Wails の createFrom が未知のフィールドを落とすため RPC にも永続化にも載らない。
  hint?: string;
}

// ファイル参照の状態。selected は現在のセッションでダイアログから確定済み、
// unconfirmed は入力欄に文字列があるだけで未確定、reselect は保存済みで再選択が必要。
export type FileSelectionState =
  | "none"
  | "selected"
  | "unconfirmed"
  | "reselect";

export function fileSelectionState(
  ref: FileReference | undefined,
): FileSelectionState {
  if (!ref) return "none";
  if (ref.token) return "selected";
  if (ref.hint) return "unconfirmed";
  if (ref.needsReselect && ref.name) return "reselect";
  return "none";
}

// 送信前にダイアログでの確定が必要なファイルがあるか。入力しただけのパスや
// 保存済みの参照は許可にならないため、backend に送る前に止める。
export function hasUnconfirmedFile(body: RequestBody): boolean {
  const pending = (ref: FileReference | undefined) => {
    const state = fileSelectionState(ref);
    return state === "unconfirmed" || state === "reselect";
  };
  if (body.type === "file") return pending(body.file);
  if (body.type === "form-data") {
    return (body.formData ?? []).some(
      (row) =>
        row.enabled &&
        row.key !== "" &&
        row.kind === "file" &&
        pending(row.file),
    );
  }
  return false;
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
  // file の送信元ファイルの参照。
  file?: FileReference;
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
  // type が "file" のときの送信元ファイルの参照。
  file?: FileReference;
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
  // 全文は backend が送信時の execution ID で追跡する一時ファイルにある（パスは公開されない）。
  bodyTruncated: boolean;
  bodyBase64: boolean;
  // 絶対上限に達して受信を打ち切った。backend の一時ファイルも全文ではない。
  bodyCapped: boolean;
}

// backend が一時ファイルを回収済み（保存済み・TTL・上限）のときに返すエラー文言。
// internal/domain/http/errors.go の ErrResponseUnavailable と一致させる。
export const RESPONSE_UNAVAILABLE_ERROR = "response body unavailable";

export function isResponseUnavailableError(message: string): boolean {
  return message.includes(RESPONSE_UNAVAILABLE_ERROR);
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
