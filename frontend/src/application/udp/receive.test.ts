import { createRoot } from "solid-js";
import { describe, expect, it, vi } from "vitest";
import type {
  UdpListenSession,
  UdpReceivedMessage,
} from "../../domain/udp/types";
import { createUdpReceiveState, type UdpReceiveApi } from "./receive";

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
      const state = createUdpReceiveState(makeApi(listeners));
      await flush();
      expect(state.sessions.map((s) => s.id)).toEqual(["s1", "s2"]);
      expect(state.sessions[0].port).toBe(9000);
      dispose();
    });
  });

  it("starts with an empty sessions store when nothing is listening", async () => {
    await createRoot(async (dispose) => {
      const state = createUdpReceiveState(makeApi([]));
      await flush();
      expect(state.sessions).toHaveLength(0);
      dispose();
    });
  });
});
