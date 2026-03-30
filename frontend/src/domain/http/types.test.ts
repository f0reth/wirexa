import { describe, expect, it } from "vitest";
import {
  AUTH_TYPES,
  BODY_TYPES,
  HTTP_METHODS,
  isAuthType,
  isBodyType,
  isHttpMethod,
} from "./types";

describe("isHttpMethod", () => {
  it.each(HTTP_METHODS)("returns true for %s", (method) => {
    expect(isHttpMethod(method)).toBe(true);
  });

  it("returns false for lowercase variants", () => {
    expect(isHttpMethod("get")).toBe(false);
    expect(isHttpMethod("post")).toBe(false);
    expect(isHttpMethod("delete")).toBe(false);
  });

  it("returns false for unknown methods", () => {
    expect(isHttpMethod("TRACE")).toBe(false);
    expect(isHttpMethod("CONNECT")).toBe(false);
    expect(isHttpMethod("GETS")).toBe(false);
    expect(isHttpMethod("")).toBe(false);
  });

  it("returns false for partial matches", () => {
    expect(isHttpMethod("GE")).toBe(false);
    expect(isHttpMethod("POSTX")).toBe(false);
  });
});

describe("isBodyType", () => {
  it.each(BODY_TYPES)("returns true for %s", (type) => {
    expect(isBodyType(type)).toBe(true);
  });

  it("returns false for unknown body types", () => {
    expect(isBodyType("binary")).toBe(false);
    expect(isBodyType("xml")).toBe(false);
    expect(isBodyType("form")).toBe(false);
    expect(isBodyType("")).toBe(false);
    expect(isBodyType("NONE")).toBe(false);
  });

  it("returns false for similar but invalid types", () => {
    expect(isBodyType("json5")).toBe(false);
    expect(isBodyType("form-data-encoded")).toBe(false);
    expect(isBodyType("urlencoded")).toBe(false);
  });
});

describe("isAuthType", () => {
  it.each(AUTH_TYPES)("returns true for %s", (type) => {
    expect(isAuthType(type)).toBe(true);
  });

  it("returns false for unknown auth types", () => {
    expect(isAuthType("apikey")).toBe(false);
    expect(isAuthType("oauth2")).toBe(false);
    expect(isAuthType("digest")).toBe(false);
    expect(isAuthType("")).toBe(false);
  });

  it("returns false for capitalized variants", () => {
    expect(isAuthType("Basic")).toBe(false);
    expect(isAuthType("Bearer")).toBe(false);
    expect(isAuthType("NONE")).toBe(false);
  });
});
