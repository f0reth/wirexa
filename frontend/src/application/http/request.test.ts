import { createRoot } from "solid-js";
import { describe, expect, it, vi } from "vitest";
import type { HttpRequest, HttpResponse } from "../../domain/http/types";
import { DEFAULT_SETTINGS } from "../../domain/http/types";
import type { Logger } from "../logger";
import { createRequestState, type RequestApi } from "./request";

const noopLogger: Logger = { info: () => {}, error: () => {} };

function makeResponse(): HttpResponse {
  return {
    statusCode: 200,
    statusText: "OK",
    headers: {},
    body: "",
    contentType: "",
    size: 0,
    timingMs: 1,
    error: "",
    bodyTruncated: false,
    tempFilePath: "",
    bodyBase64: false,
    bodyCapped: false,
  };
}

function makeRequest(id: string): HttpRequest {
  return {
    id,
    name: "saved",
    method: "GET",
    url: "https://example.com",
    headers: [],
    params: [],
    body: { type: "none", contents: {} },
    auth: { type: "none", username: "", password: "", token: "" },
    settings: { ...DEFAULT_SETTINGS },
    doc: "",
  };
}

/** sendRequest を任意のタイミングで解決できる API を作る。 */
function makeApi() {
  const sent: HttpRequest[] = [];
  const pending: Array<() => void> = [];
  const api: RequestApi = {
    sendRequest: (req) => {
      sent.push(req);
      return new Promise<HttpResponse>((resolve) => {
        pending.push(() => resolve(makeResponse()));
      });
    },
    cancelRequest: vi.fn(async () => {}),
    updateRequest: vi.fn(async () => {}),
  };
  return {
    api,
    sent,
    /** 送信済みリクエストをすべて解決する。 */
    settleAll: async () => {
      for (const resolve of pending.splice(0)) resolve();
      await Promise.resolve();
    },
  };
}

describe("createRequestState send id", () => {
  it("assigns a fresh send id per request instead of an empty id", async () => {
    await createRoot(async (dispose) => {
      const { api, sent, settleAll } = makeApi();
      const state = createRequestState(api, noopLogger);
      state.setUrl("https://example.com");

      const first = state.sendRequest();
      const second = state.sendRequest();
      await settleAll();
      await Promise.all([first, second]);

      expect(sent).toHaveLength(2);
      expect(sent[0].id).not.toBe("");
      expect(sent[1].id).not.toBe("");
      expect(sent[0].id).not.toBe(sent[1].id);
      dispose();
    });
  });

  it("does not reuse the saved request id as the send id", async () => {
    await createRoot(async (dispose) => {
      const { api, sent, settleAll } = makeApi();
      const state = createRequestState(api, noopLogger);
      state.loadRequest(makeRequest("saved-1"), "col-1");

      const done = state.sendRequest();
      await settleAll();
      await done;

      expect(sent[0].id).not.toBe("saved-1");
      expect(state.activeRequestId()).toBe("saved-1");
      dispose();
    });
  });

  it("cancels every in-flight send id", async () => {
    await createRoot(async (dispose) => {
      const { api, sent, settleAll } = makeApi();
      const state = createRequestState(api, noopLogger);

      const first = state.sendRequest();
      const second = state.sendRequest();
      expect(state.loading()).toBe(true);

      await state.cancelRequest();
      expect(api.cancelRequest).toHaveBeenCalledTimes(2);
      expect(api.cancelRequest).toHaveBeenCalledWith(sent[0].id);
      expect(api.cancelRequest).toHaveBeenCalledWith(sent[1].id);

      await settleAll();
      await Promise.all([first, second]);
      expect(state.loading()).toBe(false);
      dispose();
    });
  });

  it("keeps loading true until the last in-flight request settles", async () => {
    await createRoot(async (dispose) => {
      const { api, settleAll } = makeApi();
      const state = createRequestState(api, noopLogger);

      const first = state.sendRequest();
      expect(state.loading()).toBe(true);

      const second = state.sendRequest();
      await settleAll();
      await Promise.all([first, second]);

      expect(state.loading()).toBe(false);
      await state.cancelRequest();
      expect(api.cancelRequest).not.toHaveBeenCalled();
      dispose();
    });
  });
});
