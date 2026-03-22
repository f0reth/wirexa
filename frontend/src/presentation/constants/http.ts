import type { BodyType, HttpMethod } from "../../domain/http/types";

export const METHOD_COLORS: Record<HttpMethod, string> = {
  GET: "#4CAF50",
  POST: "#FF9800",
  PUT: "#2196F3",
  PATCH: "#9C27B0",
  DELETE: "#F44336",
  HEAD: "#607D8B",
  OPTIONS: "#00BCD4",
};

export const BODY_TYPES: { value: BodyType; label: string }[] = [
  { value: "none", label: "None" },
  { value: "json", label: "JSON" },
  { value: "text", label: "Text" },
  { value: "form-urlencoded", label: "Form URL Encoded" },
  { value: "form-data", label: "Form Data" },
];
