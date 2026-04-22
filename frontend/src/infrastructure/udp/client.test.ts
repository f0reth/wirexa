import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("../../../wailsjs/go/adapters/UdpHandler", () => ({
  DeleteTarget: vi.fn(),
  GetListeners: vi.fn(),
  GetTargets: vi.fn(),
  SaveTarget: vi.fn(),
  Send: vi.fn(),
  StartListen: vi.fn(),
  StopListen: vi.fn(),
}));

vi.mock("../../../wailsjs/runtime/runtime", () => ({
  EventsOn: vi.fn(),
}));

import * as Handler from "../../../wailsjs/go/adapters/UdpHandler";
import * as Runtime from "../../../wailsjs/runtime/runtime";
import {
  deleteTarget,
  getListeners,
  getTargets,
  onMessage,
  saveTarget,
  send,
  startListen,
  stopListen,
} from "./client";

// ---- helpers ----

function makeWailsSendResult(overrides: Record<string, unknown> = {}) {
  return { bytesSent: 64, ...overrides };
}

function makeWailsTarget(overrides: Record<string, unknown> = {}) {
  return {
    id: "target-1",
    name: "Local",
    host: "127.0.0.1",
    port: 9000,
    ...overrides,
  };
}

function makeWailsListenSession(overrides: Record<string, unknown> = {}) {
  return {
    id: "session-1",
    port: 9001,
    encoding: "text",
    ...overrides,
  };
}

function makeSendRequest(overrides: Record<string, unknown> = {}) {
  return {
    host: "127.0.0.1",
    port: 9000,
    encoding: "text" as const,
    payload: "hello",
    messageLength: 0,
    fixedLengthPayload: { fields: [] },
    endianness: "big" as const,
    ...overrides,
  };
}

// ---- tests ----

beforeEach(() => {
  vi.clearAllMocks();
});

describe("send", () => {
  it("maps the result to the domain UdpSendResult", async () => {
    vi.mocked(Handler.Send).mockResolvedValue(
      makeWailsSendResult({ bytesSent: 128 }) as never,
    );
    const result = await send(makeSendRequest());
    expect(result).toEqual({ bytesSent: 128 });
  });

  it("calls Send once", async () => {
    vi.mocked(Handler.Send).mockResolvedValue(makeWailsSendResult() as never);
    await send(makeSendRequest());
    expect(Handler.Send).toHaveBeenCalledOnce();
  });

  it("propagates rejection from the backend", async () => {
    vi.mocked(Handler.Send).mockRejectedValue(new Error("send failed"));
    await expect(send(makeSendRequest())).rejects.toThrow("send failed");
  });

  it("passes the host and port to the backend", async () => {
    vi.mocked(Handler.Send).mockResolvedValue(makeWailsSendResult() as never);
    const req = makeSendRequest({ host: "192.168.1.1", port: 5000 });
    await send(req);
    const callArg = vi.mocked(Handler.Send).mock.calls[0][0];
    expect(callArg).toMatchObject({ host: "192.168.1.1", port: 5000 });
  });

  it("passes all fields to the backend", async () => {
    vi.mocked(Handler.Send).mockResolvedValue(makeWailsSendResult() as never);
    const req = makeSendRequest({
      host: "10.0.0.1",
      port: 8080,
      encoding: "text",
      payload: "hello",
      endianness: "big",
    });
    await send(req);
    const callArg = vi.mocked(Handler.Send).mock.calls[0][0];
    expect(callArg).toMatchObject({
      host: "10.0.0.1",
      port: 8080,
      encoding: "text",
      payload: "hello",
      endianness: "big",
    });
  });
});

describe("getTargets", () => {
  it("returns an empty array when there are no targets", async () => {
    vi.mocked(Handler.GetTargets).mockResolvedValue([]);
    expect(await getTargets()).toEqual([]);
  });

  it("maps the list of targets", async () => {
    vi.mocked(Handler.GetTargets).mockResolvedValue([
      makeWailsTarget() as never,
      makeWailsTarget({ id: "target-2", name: "Remote", port: 9001 }) as never,
    ]);
    const result = await getTargets();
    expect(result).toHaveLength(2);
    expect(result[0]).toEqual({
      id: "target-1",
      name: "Local",
      host: "127.0.0.1",
      port: 9000,
    });
    expect(result[1]).toEqual({
      id: "target-2",
      name: "Remote",
      host: "127.0.0.1",
      port: 9001,
    });
  });
});

describe("saveTarget", () => {
  it("returns the mapped saved target with all fields", async () => {
    vi.mocked(Handler.SaveTarget).mockResolvedValue(
      makeWailsTarget({ id: "target-99" }) as never,
    );
    const result = await saveTarget({
      id: "",
      name: "New",
      host: "10.0.0.1",
      port: 7000,
    });
    expect(result).toEqual({
      id: "target-99",
      name: "Local",
      host: "127.0.0.1",
      port: 9000,
    });
  });

  it("passes the target fields to the backend", async () => {
    vi.mocked(Handler.SaveTarget).mockResolvedValue(makeWailsTarget() as never);
    await saveTarget({ id: "", name: "Dev", host: "localhost", port: 1234 });
    expect(Handler.SaveTarget).toHaveBeenCalledOnce();
    const callArg = vi.mocked(Handler.SaveTarget).mock.calls[0][0];
    expect(callArg).toMatchObject({
      name: "Dev",
      host: "localhost",
      port: 1234,
    });
  });
});

describe("deleteTarget", () => {
  it("calls DeleteTarget with the correct id", async () => {
    vi.mocked(Handler.DeleteTarget).mockResolvedValue(undefined);
    await deleteTarget("target-1");
    expect(Handler.DeleteTarget).toHaveBeenCalledWith("target-1");
  });
});

describe("startListen", () => {
  it("maps the listen session from the backend", async () => {
    vi.mocked(Handler.StartListen).mockResolvedValue(
      makeWailsListenSession({
        id: "s1",
        port: 9001,
        encoding: "json",
      }) as never,
    );
    const result = await startListen(9001, "json");
    expect(result).toEqual({ id: "s1", port: 9001, encoding: "json" });
  });

  it("calls StartListen with the correct port and encoding", async () => {
    vi.mocked(Handler.StartListen).mockResolvedValue(
      makeWailsListenSession() as never,
    );
    await startListen(8080, "text");
    expect(Handler.StartListen).toHaveBeenCalledWith(8080, "text");
  });

  it.each([
    "text",
    "json",
    "fixed",
  ])("passes encoding '%s' to the backend", async (encoding) => {
    vi.mocked(Handler.StartListen).mockResolvedValue(
      makeWailsListenSession({ encoding }) as never,
    );
    const result = await startListen(9000, encoding);
    expect(result.encoding).toBe(encoding);
  });
});

describe("stopListen", () => {
  it("calls StopListen with the correct session id", async () => {
    vi.mocked(Handler.StopListen).mockResolvedValue(undefined);
    await stopListen("session-1");
    expect(Handler.StopListen).toHaveBeenCalledWith("session-1");
  });

  it("propagates rejection from the backend", async () => {
    vi.mocked(Handler.StopListen).mockRejectedValue(new Error("stop failed"));
    await expect(stopListen("session-1")).rejects.toThrow("stop failed");
  });
});

describe("getListeners", () => {
  it("returns an empty array when there are no active listeners", async () => {
    vi.mocked(Handler.GetListeners).mockResolvedValue([]);
    expect(await getListeners()).toEqual([]);
  });

  it("maps the list of listen sessions", async () => {
    vi.mocked(Handler.GetListeners).mockResolvedValue([
      makeWailsListenSession({
        id: "s1",
        port: 9001,
        encoding: "text",
      }) as never,
      makeWailsListenSession({ id: "s2", port: 9002 }) as never,
    ]);
    const result = await getListeners();
    expect(result).toHaveLength(2);
    expect(result[0].id).toBe("s1");
    expect(result[0].encoding).toBe("text");
    expect(result[1].port).toBe(9002);
  });

  it("propagates rejection from the backend", async () => {
    vi.mocked(Handler.GetListeners).mockRejectedValue(
      new Error("get listeners failed"),
    );
    await expect(getListeners()).rejects.toThrow("get listeners failed");
  });

  it("silently passes unknown encoding (as-cast risk)", async () => {
    vi.mocked(Handler.GetListeners).mockResolvedValue([
      makeWailsListenSession({ encoding: "unknown" }) as never,
    ]);
    const result = await getListeners();
    expect(result[0].encoding).toBe("unknown");
  });
});

describe("onMessage", () => {
  it("registers a listener for the udp:message event", () => {
    vi.mocked(Runtime.EventsOn).mockReturnValue(vi.fn());
    const cb = vi.fn();
    onMessage(cb);
    expect(Runtime.EventsOn).toHaveBeenCalledWith("udp:message", cb);
  });

  it("returns the cleanup function from EventsOn", () => {
    const mockCleanup = vi.fn();
    vi.mocked(Runtime.EventsOn).mockReturnValue(mockCleanup);

    const cleanup = onMessage(vi.fn());
    expect(cleanup).toBe(mockCleanup);
  });

  it("forwards the received message to the callback", () => {
    const cb = vi.fn();
    let capturedCb: ((msg: unknown) => void) | undefined;

    vi.mocked(Runtime.EventsOn).mockImplementation((_event, handler) => {
      capturedCb = handler as (msg: unknown) => void;
      return vi.fn();
    });

    onMessage(cb);

    const msg = {
      sessionId: "s1",
      port: 9001,
      remoteAddr: "192.168.1.1:50000",
      payload: "hello",
      encoding: "text",
      timestamp: 1234567890,
    };
    capturedCb?.(msg);
    expect(cb).toHaveBeenCalledWith(msg);
  });
});
