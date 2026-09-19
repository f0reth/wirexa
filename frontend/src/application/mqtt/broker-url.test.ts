import { describe, expect, it } from "vitest";
import { composeBrokerUrl, defaultPort, parseBrokerUrl } from "./broker-url";

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
