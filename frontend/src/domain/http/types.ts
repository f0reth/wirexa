// フロントエンド domain 層の型定義（正）。
// Wails 自動生成型（wailsjs/go/models.ts）との変換は infrastructure/http/client.ts で行う。
// これらの型を変更した場合は internal/domain/http/types.go も必ず合わせて更新すること。

export interface KeyValuePair {
  key: string;
  value: string;
  enabled: boolean;
}

export interface RequestBody {
  type: "none" | "json" | "text" | "form-urlencoded" | "form-data";
  content: string;
}

export type AuthType = "none" | "basic" | "bearer";
export const AUTH_TYPES: AuthType[] = ["none", "basic", "bearer"];

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
  timeoutSec: 0,
  proxyMode: "none",
  proxyURL: "",
  insecureSkipVerify: false,
  disableRedirects: false,
  maxResponseBodyMB: 50,
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

export type HttpMethod =
  | "GET"
  | "POST"
  | "PUT"
  | "PATCH"
  | "DELETE"
  | "HEAD"
  | "OPTIONS";
export const HTTP_METHODS: HttpMethod[] = [
  "GET",
  "POST",
  "PUT",
  "PATCH",
  "DELETE",
  "HEAD",
  "OPTIONS",
];

export type BodyType = RequestBody["type"];
export const BODY_TYPES: BodyType[] = [
  "none",
  "json",
  "text",
  "form-urlencoded",
  "form-data",
];

export function isHttpMethod(v: string): v is HttpMethod {
  return (HTTP_METHODS as string[]).includes(v);
}

export function isBodyType(v: string): v is BodyType {
  return (BODY_TYPES as string[]).includes(v);
}

export function isAuthType(v: string): v is AuthType {
  return (AUTH_TYPES as string[]).includes(v);
}
