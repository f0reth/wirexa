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

export interface HttpRequest {
  id: string;
  name: string;
  method: HttpMethod;
  url: string;
  headers: KeyValuePair[];
  params: KeyValuePair[];
  body: RequestBody;
  auth: RequestAuth;
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
