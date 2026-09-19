import { describe, expect, it } from "vitest";
import { isValidBrokerPort, isValidProfileDraft } from "./profile-validation";

describe("isValidBrokerPort", () => {
  it("accepts ports within 1-65535", () => {
    expect(isValidBrokerPort("1")).toBe(true);
    expect(isValidBrokerPort("1883")).toBe(true);
    expect(isValidBrokerPort("65535")).toBe(true);
  });

  it("rejects out-of-range ports", () => {
    expect(isValidBrokerPort("0")).toBe(false);
    expect(isValidBrokerPort("65536")).toBe(false);
    expect(isValidBrokerPort("-1")).toBe(false);
  });

  it("rejects empty, whitespace-only and non-numeric input", () => {
    expect(isValidBrokerPort("")).toBe(false);
    expect(isValidBrokerPort("  ")).toBe(false);
    expect(isValidBrokerPort("abc")).toBe(false);
  });

  it("rejects non-integer numbers", () => {
    expect(isValidBrokerPort("1883.5")).toBe(false);
  });
});

describe("isValidProfileDraft", () => {
  const valid = { name: "My Broker", host: "localhost", port: "1883" };

  it("accepts a fully filled draft", () => {
    expect(isValidProfileDraft(valid)).toBe(true);
  });

  it("rejects a blank name", () => {
    expect(isValidProfileDraft({ ...valid, name: "" })).toBe(false);
    expect(isValidProfileDraft({ ...valid, name: "   " })).toBe(false);
  });

  it("rejects a blank host", () => {
    expect(isValidProfileDraft({ ...valid, host: "" })).toBe(false);
    expect(isValidProfileDraft({ ...valid, host: "   " })).toBe(false);
  });

  it("rejects an invalid port", () => {
    expect(isValidProfileDraft({ ...valid, port: "0" })).toBe(false);
    expect(isValidProfileDraft({ ...valid, port: "65536" })).toBe(false);
  });
});
