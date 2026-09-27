import { describe, expect, it } from "vitest";
import {
  AUTH_TYPES,
  BODY_TYPES,
  type BodyType,
  type FileReference,
  type FileSelectionState,
  FORM_ROW_KINDS,
  type FormRow,
  fileSelectionState,
  HTTP_METHODS,
  hasUnconfirmedFile,
  isAuthType,
  isBodyType,
  isFormBodyType,
  isFormRowKind,
  isHttpMethod,
  isResponseUnavailableError,
  RESPONSE_UNAVAILABLE_ERROR,
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

  it("returns false for whitespace-only string", () => {
    expect(isHttpMethod(" ")).toBe(false);
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

  it("returns false for capitalized variants", () => {
    expect(isBodyType("JSON")).toBe(false);
    expect(isBodyType("FILE")).toBe(false);
  });

  it("returns false for whitespace-only string", () => {
    expect(isBodyType(" ")).toBe(false);
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

  it("returns false for whitespace-only string", () => {
    expect(isAuthType(" ")).toBe(false);
  });
});

describe("isFormRowKind", () => {
  it.each(FORM_ROW_KINDS)("returns true for %s", (kind) => {
    expect(isFormRowKind(kind)).toBe(true);
  });

  it("returns false for capitalized, empty and partial values", () => {
    expect(isFormRowKind("FILE")).toBe(false);
    expect(isFormRowKind("")).toBe(false);
    expect(isFormRowKind("fil")).toBe(false);
    expect(isFormRowKind(" ")).toBe(false);
  });
});

describe("isFormBodyType", () => {
  const formBodyTypes: BodyType[] = ["form-data", "form-urlencoded"];

  it.each(
    BODY_TYPES,
  )("returns true for %s only when it is a form body", (type) => {
    expect(isFormBodyType(type)).toBe(formBodyTypes.includes(type));
  });

  it("does not treat prototype keys as form bodies", () => {
    expect(isFormBodyType("toString" as BodyType)).toBe(false);
    expect(isFormBodyType("constructor" as BodyType)).toBe(false);
  });
});

describe("isResponseUnavailableError", () => {
  it("matches the backend message", () => {
    expect(isResponseUnavailableError(RESPONSE_UNAVAILABLE_ERROR)).toBe(true);
  });

  it("matches the backend message even when wrapped", () => {
    expect(
      isResponseUnavailableError("save: response body unavailable (ttl)"),
    ).toBe(true);
  });

  it("is case-sensitive", () => {
    expect(isResponseUnavailableError("Response body unavailable")).toBe(false);
  });

  it("returns false for other errors", () => {
    expect(isResponseUnavailableError("")).toBe(false);
    expect(isResponseUnavailableError("permission denied")).toBe(false);
  });
});

describe("fileSelectionState", () => {
  it.each<[FileReference | undefined, FileSelectionState]>([
    [undefined, "none"],
    [{}, "none"],
    [{ token: "t1", name: "a.bin" }, "selected"],
    // token と hint が両方あれば確定済みを優先する。
    [{ token: "t1", hint: "C:a.bin" }, "selected"],
    [{ hint: "C:a.bin" }, "unconfirmed"],
    [{ needsReselect: true, name: "a.bin" }, "reselect"],
    // name の無い再選択参照は表示できないので none として扱う。
    [{ needsReselect: true }, "none"],
    [{ needsReselect: true, name: "" }, "none"],
    [{ name: "a.bin" }, "none"],
  ])("classifies %j as %s", (ref, expected) => {
    expect(fileSelectionState(ref)).toBe(expected);
  });
});

describe("hasUnconfirmedFile", () => {
  const row = (over: Partial<FormRow>): FormRow => ({
    key: "k",
    value: "",
    enabled: true,
    ...over,
  });

  it("stops a file body whose file is unconfirmed or needs reselecting", () => {
    expect(
      hasUnconfirmedFile({ type: "file", contents: {}, file: { hint: "x" } }),
    ).toBe(true);
    expect(
      hasUnconfirmedFile({
        type: "file",
        contents: {},
        file: { needsReselect: true, name: "a.bin" },
      }),
    ).toBe(true);
    expect(
      hasUnconfirmedFile({ type: "file", contents: {}, file: { token: "t" } }),
    ).toBe(false);
  });

  it("does not stop a file body without a file reference", () => {
    expect(hasUnconfirmedFile({ type: "file", contents: {} })).toBe(false);
  });

  it("stops form-data with an enabled, keyed, unconfirmed file row", () => {
    expect(
      hasUnconfirmedFile({
        type: "form-data",
        contents: {},
        formData: [row({ kind: "file", file: { hint: "x" } })],
      }),
    ).toBe(true);
  });

  it("ignores disabled rows and rows without a key", () => {
    expect(
      hasUnconfirmedFile({
        type: "form-data",
        contents: {},
        formData: [
          row({ kind: "file", file: { hint: "x" }, enabled: false }),
          row({ kind: "file", file: { hint: "x" }, key: "" }),
        ],
      }),
    ).toBe(false);
  });

  it("ignores file references on rows whose kind is not file", () => {
    // kind を file → text に戻しても file 参照は残るが、送信時には使われない。
    expect(
      hasUnconfirmedFile({
        type: "form-data",
        contents: {},
        formData: [
          row({ kind: "text", file: { hint: "x" } }),
          row({ file: { needsReselect: true, name: "a.bin" } }),
        ],
      }),
    ).toBe(false);
  });

  it("only checks form-data rows", () => {
    expect(
      hasUnconfirmedFile({
        type: "form-urlencoded",
        contents: {},
        formUrlEncoded: [row({ kind: "file", file: { hint: "x" } })],
        formData: [row({ kind: "file", file: { hint: "x" } })],
      }),
    ).toBe(false);
  });

  it("does not stop form-data without rows", () => {
    expect(hasUnconfirmedFile({ type: "form-data", contents: {} })).toBe(false);
  });
});
