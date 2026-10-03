import { describe, expect, it } from "vitest";
import {
  BROKER_SCHEMES,
  composeBrokerUrl,
  DEFAULT_BROKER_URL,
  defaultPort,
  parseBrokerUrl,
} from "./broker-url";

describe("defaultPort", () => {
  it("returns the well-known port per scheme", () => {
    expect(defaultPort("mqtt")).toBe("1883");
    expect(defaultPort("mqtts")).toBe("8883");
    expect(defaultPort("tcp")).toBe("1883");
    expect(defaultPort("ws")).toBe("9001");
    expect(defaultPort("wss")).toBe("8884");
  });

  it("falls back to 1883 for unknown schemes", () => {
    expect(defaultPort("http")).toBe("1883");
    expect(defaultPort("")).toBe("1883");
  });
});

describe("BROKER_SCHEMES", () => {
  // 選べるスキームは、保存した URL から読み戻せなければならない。
  it.each(BROKER_SCHEMES)("reads back a url with the %s scheme", (scheme) => {
    expect(parseBrokerUrl(composeBrokerUrl(scheme, "host", "1234"))).toEqual({
      scheme,
      host: "host",
      port: "1234",
    });
  });
});

describe("DEFAULT_BROKER_URL", () => {
  it("is the url an unreadable value falls back to", () => {
    expect(DEFAULT_BROKER_URL).toBe("mqtt://localhost:1883");
    const parts = parseBrokerUrl("not a url");
    expect(composeBrokerUrl(parts.scheme, parts.host, parts.port)).toBe(
      DEFAULT_BROKER_URL,
    );
  });
});

describe("parseBrokerUrl", () => {
  it("splits a full url into its parts", () => {
    expect(parseBrokerUrl("mqtts://broker.example.com:8883")).toEqual({
      scheme: "mqtts",
      host: "broker.example.com",
      port: "8883",
    });
  });

  it("fills in the scheme default when the port is omitted", () => {
    expect(parseBrokerUrl("ws://localhost")).toEqual({
      scheme: "ws",
      host: "localhost",
      port: "9001",
    });
  });

  it("falls back to mqtt://localhost:1883 for unparsable input", () => {
    const fallback = { scheme: "mqtt", host: "localhost", port: "1883" };
    expect(parseBrokerUrl("")).toEqual(fallback);
    expect(parseBrokerUrl("localhost:1883")).toEqual(fallback);
    expect(parseBrokerUrl("http://localhost:1883")).toEqual(fallback);
    // ホストにコロンを含む形（IPv6 リテラル）は未対応。
    expect(parseBrokerUrl("mqtt://[::1]:1883")).toEqual(fallback);
  });

  it("falls back for an upper-case scheme", () => {
    expect(parseBrokerUrl("MQTT://host:1883")).toEqual({
      scheme: "mqtt",
      host: "localhost",
      port: "1883",
    });
  });

  it("falls back when the port is empty", () => {
    expect(parseBrokerUrl("mqtt://h:")).toEqual({
      scheme: "mqtt",
      host: "localhost",
      port: "1883",
    });
  });

  // ポートの範囲は profile-validation 側で検証する。
  it("does not validate the port range", () => {
    expect(parseBrokerUrl("mqtt://h:99999").port).toBe("99999");
    expect(parseBrokerUrl("mqtt://h:0").port).toBe("0");
  });

  it("treats a path as part of the host", () => {
    expect(parseBrokerUrl("mqtt://host/path")).toEqual({
      scheme: "mqtt",
      host: "host/path",
      port: "1883",
    });
  });
});

describe("composeBrokerUrl", () => {
  it("joins the parts back into a url", () => {
    expect(composeBrokerUrl("mqtt", "localhost", "1883")).toBe(
      "mqtt://localhost:1883",
    );
  });

  it("round-trips a parsed url", () => {
    const url = "wss://broker.example.com:8884";
    const parts = parseBrokerUrl(url);
    expect(composeBrokerUrl(parts.scheme, parts.host, parts.port)).toBe(url);
  });
});
