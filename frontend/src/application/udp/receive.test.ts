import { createRoot } from "solid-js";
import { describe, expect, it, vi } from "vitest";
import { UDP_MAX_MESSAGES } from "../../config/limits";
import type {
  UdpListenSession,
  UdpReceivedMessage,
} from "../../domain/udp/types";
import type { Notifier } from "../../domain/ui/ports";
import { createUdpReceiveState, type UdpReceiveApi } from "./receive";

function makeNotifier(): Notifier {
  return {
    error: vi.fn(),
    success: vi.fn(),
    info: vi.fn(),
    warning: vi.fn(),
  };
}

function makeApi(listeners: UdpListenSession[]): UdpReceiveApi {
  return {
    startListen: vi.fn(async () => ({
      id: "new",
      port: 1,
      encoding: "text" as const,
    })),
    stopListen: vi.fn(async () => {}),
    getListeners: vi.fn(async () => listeners),
    onMessage: (_cb: (msg: UdpReceivedMessage) => void) => () => {},
  };
}

// createUdpReceiveState は初期化時に refreshListeners を fire-and-forget で呼ぶため、
// マイクロタスクを待ってから sessions を検証する。
const flush = () => new Promise((resolve) => setTimeout(resolve, 0));

describe("createUdpReceiveState restore", () => {
  it("restores active backend listeners into sessions on init", async () => {
    const listeners: UdpListenSession[] = [
      { id: "s1", port: 9000, encoding: "text" },
      { id: "s2", port: 9001, encoding: "json" },
    ];

    await createRoot(async (dispose) => {
      const state = createUdpReceiveState(makeApi(listeners), makeNotifier());
      await flush();
      expect(state.sessions.map((s) => s.id)).toEqual(["s1", "s2"]);
      expect(state.sessions[0].port).toBe(9000);
      dispose();
    });
  });

  it("starts with an empty sessions store when nothing is listening", async () => {
    await createRoot(async (dispose) => {
      const state = createUdpReceiveState(makeApi([]), makeNotifier());
      await flush();
      expect(state.sessions).toHaveLength(0);
      dispose();
    });
  });

  it("notifies when restoring listeners fails", async () => {
    const api = makeApi([]);
    api.getListeners = vi.fn(async () => {
      throw new Error("rpc down");
    });
    const notifier = makeNotifier();

    await createRoot(async (dispose) => {
      const state = createUdpReceiveState(api, notifier);
      await flush();
      expect(notifier.error).toHaveBeenCalledWith(
        "Failed to restore listeners",
        "rpc down",
      );
      expect(state.sessions).toHaveLength(0);
      dispose();
    });
  });
});

/** onMessage のコールバックを保持し、テストから受信を発生させられる API。 */
function makeMessageApi(listeners: UdpListenSession[] = []) {
  let deliver: ((msg: UdpReceivedMessage) => void) | undefined;
  const unsubscribe = vi.fn();
  const api = {
    ...makeApi(listeners),
    onMessage: vi.fn((cb: (msg: UdpReceivedMessage) => void) => {
      deliver = cb;
      return unsubscribe;
    }),
  };
  const receive = (payload: string) =>
    deliver?.({
      sessionId: "s1",
      port: 9000,
      remoteAddr: "127.0.0.1:50000",
      payload,
      encoding: "text",
      timestamp: 0,
    });
  return { api, receive, unsubscribe };
}

async function setupReceive(
  api: UdpReceiveApi = makeApi([]),
  notifier: Notifier = makeNotifier(),
) {
  const h = createRoot((dispose) => ({
    state: createUdpReceiveState(api, notifier),
    dispose,
  }));
  await flush();
  return { ...h, api, notifier };
}

describe("createUdpReceiveState messages", () => {
  it("keeps the newest UDP_MAX_MESSAGES messages, newest first", async () => {
    const { api, receive } = makeMessageApi();
    const { state, dispose } = await setupReceive(api);

    receive("first");
    receive("second");
    expect(state.messages.map((m) => m.payload)).toEqual(["second", "first"]);

    for (let i = 0; i < UDP_MAX_MESSAGES; i++) receive(`m${i}`);
    expect(state.messages).toHaveLength(UDP_MAX_MESSAGES);
    expect(state.messages[0].payload).toBe(`m${UDP_MAX_MESSAGES - 1}`);
    expect(state.messages[UDP_MAX_MESSAGES - 1].payload).toBe("m0");
    dispose();
  });

  it("clears the received messages", async () => {
    const { api, receive } = makeMessageApi();
    const { state, dispose } = await setupReceive(api);
    receive("a");

    state.clearMessages();

    expect(state.messages).toHaveLength(0);
    dispose();
  });

  it("unsubscribes from messages on dispose", async () => {
    const { api, unsubscribe } = makeMessageApi();
    const { dispose } = await setupReceive(api);
    expect(unsubscribe).not.toHaveBeenCalled();

    dispose();

    expect(unsubscribe).toHaveBeenCalledTimes(1);
  });
});

describe("createUdpReceiveState listen", () => {
  it("adds the new session and clears loading", async () => {
    const api = makeApi([{ id: "s1", port: 9000, encoding: "text" }]);
    let finish: (s: UdpListenSession) => void = () => {};
    api.startListen = vi.fn(
      () =>
        new Promise<UdpListenSession>((resolve) => {
          finish = resolve;
        }),
    );
    const { state, notifier, dispose } = await setupReceive(api);
    state.setListenPort(9100);
    state.setListenEncoding("json");

    const started = state.startListen();
    expect(state.loading()).toBe(true);
    finish({ id: "s2", port: 9100, encoding: "json" });
    await started;

    expect(api.startListen).toHaveBeenCalledWith(9100, "json");
    expect(state.sessions.map((s) => s.id)).toEqual(["s1", "s2"]);
    expect(state.loading()).toBe(false);
    expect(state.error()).toBeNull();
    expect(notifier.error).not.toHaveBeenCalled();
    dispose();
  });

  it("records the error and notifies when listening fails", async () => {
    const api = makeApi([]);
    api.startListen = vi.fn(async () => {
      throw new Error("address already in use");
    });
    const { state, notifier, dispose } = await setupReceive(api);

    await state.startListen();

    expect(state.error()).toBe("address already in use");
    expect(state.loading()).toBe(false);
    expect(state.sessions).toHaveLength(0);
    expect(notifier.error).toHaveBeenCalledWith(
      "Failed to start listening",
      "address already in use",
    );

    // 次の開始で前回のエラーを消す。
    api.startListen = vi.fn(async () => ({
      id: "s1",
      port: 9000,
      encoding: "text" as const,
    }));
    await state.startListen();
    expect(state.error()).toBeNull();
    dispose();
  });

  it("removes only the stopped session", async () => {
    const api = makeApi([
      { id: "s1", port: 9000, encoding: "text" },
      { id: "s2", port: 9001, encoding: "json" },
    ]);
    const { state, dispose } = await setupReceive(api);

    await state.stopListen("s1");

    expect(api.stopListen).toHaveBeenCalledWith("s1");
    expect(state.sessions.map((s) => s.id)).toEqual(["s2"]);
    dispose();
  });

  it("keeps the session and notifies when stopping fails", async () => {
    const api = makeApi([{ id: "s1", port: 9000, encoding: "text" }]);
    api.stopListen = vi.fn(async () => {
      throw new Error("not found");
    });
    const { state, notifier, dispose } = await setupReceive(api);

    await expect(state.stopListen("s1")).resolves.toBeUndefined();

    expect(state.sessions.map((s) => s.id)).toEqual(["s1"]);
    expect(notifier.error).toHaveBeenCalledWith(
      "Failed to stop listening",
      "not found",
    );
    dispose();
  });
});
