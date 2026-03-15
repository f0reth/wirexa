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
  method: string;
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
  error?: string;
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
  children?: TreeItem[];
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

export const METHOD_COLORS: Record<HttpMethod, string> = {
  GET: "#4CAF50",
  POST: "#FF9800",
  PUT: "#2196F3",
  PATCH: "#9C27B0",
  DELETE: "#F44336",
  HEAD: "#607D8B",
  OPTIONS: "#00BCD4",
};

export type BodyType = RequestBody["type"];

export const BODY_TYPES: { value: BodyType; label: string }[] = [
  { value: "none", label: "None" },
  { value: "json", label: "JSON" },
  { value: "text", label: "Text" },
  { value: "form-urlencoded", label: "Form URL Encoded" },
  { value: "form-data", label: "Form Data" },
];
