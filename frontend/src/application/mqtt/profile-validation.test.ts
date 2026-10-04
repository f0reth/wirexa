import { describe, expect, it } from "vitest";
import { composeBrokerUrl, parseBrokerUrl } from "./broker-url";
import {
  isValidBrokerAddress,
  isValidBrokerPort,
  isValidProfileDraft,
} from "./profile-validation";

describe("isValidBrokerAddress", () => {
  it("accepts a host with a port up to 65535", () => {
    expect(isValidBrokerAddress("alpha.local", "1883")).toBe(true);
    expect(isValidBrokerAddress("10.0.0.1", "65535")).toBe(true);
  });

  it.each([
    "",
    "   ",
    "al pha",
    "host:1883",
    "::1",
  ])("rejects the host %j", (host) => {
    expect(isValidBrokerAddress(host, "1883")).toBe(false);
  });

  it.each(["0", "65536", "", "1e3"])("rejects the port %j", (port) => {
    expect(isValidBrokerAddress("alpha.local", port)).toBe(false);
  });
});

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

  // type="number" の入力欄からも "1e3" と "1883.0" は入力できる。
  // 通すと保存した broker URL を parseBrokerUrl が読めず、既定値に戻ってしまう。
  it("rejects exponent and decimal port strings", () => {
    expect(isValidBrokerPort("1e3")).toBe(false);
    expect(isValidBrokerPort("1883.0")).toBe(false);
  });

  it("rejects hex, signed and padded port strings", () => {
    expect(isValidBrokerPort("0x50")).toBe(false);
    expect(isValidBrokerPort("+1883")).toBe(false);
    expect(isValidBrokerPort(" 1883 ")).toBe(false);
  });

  it("accepts leading zeros within range", () => {
    expect(isValidBrokerPort("01883")).toBe(true);
    expect(isValidBrokerPort("00000")).toBe(false);
  });

  it("round-trips a valid port through compose/parse", () => {
    for (const port of ["1", "1883", "65535"]) {
      const parsed = parseBrokerUrl(composeBrokerUrl("mqtt", "host", port));
      expect(parsed).toEqual({ scheme: "mqtt", host: "host", port });
    }
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

  // parseBrokerUrl はホストのコロンを読めず、空白やパスは接続先として解釈できない。
  it("rejects hosts that parseBrokerUrl cannot read back", () => {
    expect(isValidProfileDraft({ ...valid, host: "::1" })).toBe(false);
    expect(isValidProfileDraft({ ...valid, host: "[::1]" })).toBe(false);
    expect(isValidProfileDraft({ ...valid, host: "host:1883" })).toBe(false);
    expect(isValidProfileDraft({ ...valid, host: " host " })).toBe(false);
    expect(isValidProfileDraft({ ...valid, host: "my host" })).toBe(false);
    expect(isValidProfileDraft({ ...valid, host: "host/path" })).toBe(false);
  });

  it("accepts hostnames and IPv4 addresses", () => {
    for (const host of ["localhost", "broker.example.com", "192.168.0.1"]) {
      expect(isValidProfileDraft({ ...valid, host })).toBe(true);
      const url = composeBrokerUrl("mqtt", host, valid.port);
      expect(parseBrokerUrl(url).host).toBe(host);
    }
  });

  it("rejects an invalid port", () => {
    expect(isValidProfileDraft({ ...valid, port: "0" })).toBe(false);
    expect(isValidProfileDraft({ ...valid, port: "65536" })).toBe(false);
  });
});
