export interface KeyValuePair {
  key: string;
  value: string;
  enabled: boolean;
}

export interface RequestBody {
  type: "none" | "json" | "text" | "form-urlencoded" | "form-data";
  content: string;
}

export interface HttpRequest {
  id: string;
  name: string;
  method: HttpMethod;
  url: string;
  headers: KeyValuePair[];
  params: KeyValuePair[];
  body: RequestBody;
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
