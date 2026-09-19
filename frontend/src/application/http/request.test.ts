import { createRoot } from "solid-js";
import { describe, expect, it, vi } from "vitest";
import type { HttpRequest, HttpResponse } from "../../domain/http/types";
import { DEFAULT_SETTINGS } from "../../domain/http/types";
import type { Logger } from "../logger";
import {
  createRequestState,
  type RequestApi,
  type RequestState,
} from "./request";

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

/** createRoot 内で state を作り、テスト本体を実行する。 */
function withState(fn: (state: RequestState) => void): void {
  createRoot((dispose) => {
    fn(createRequestState(makeApi().api, noopLogger));
    dispose();
  });
}

describe("createRequestState body content", () => {
  it("returns an empty string when the body type has no content", () => {
    withState((state) => {
      expect(state.bodyContent()).toBe("");
      state.setBody({ type: "text", contents: {} });
      expect(state.bodyContent()).toBe("");
    });
  });

  it("returns the json template only for an untouched json body", () => {
    withState((state) => {
      state.setBody({ type: "json", contents: {} });
      expect(state.bodyContent()).toBe('{\n  "": ""\n}');

      state.setBodyContent("");
      expect(state.bodyContent()).toBe("");
    });
  });

  it("keeps the content of each body type separately", () => {
    withState((state) => {
      state.setBody({ type: "text", contents: {} });
      state.setBodyContent("plain");
      state.setBody({ ...state.body(), type: "json" });
      state.setBodyContent('{"a":1}');

      expect(state.bodyContent()).toBe('{"a":1}');
      state.setBody({ ...state.body(), type: "text" });
      expect(state.bodyContent()).toBe("plain");
    });
  });
});

describe("createRequestState form pairs", () => {
  it("reports no form body type outside form bodies", () => {
    withState((state) => {
      expect(state.formBodyType()).toBeNull();
      state.setBody({ type: "json", contents: {} });
      expect(state.formBodyType()).toBeNull();
    });
  });

  it("returns a stable empty array while no row exists", () => {
    withState((state) => {
      const first = state.formPairs();
      expect(first).toEqual([]);
      state.setBody({ type: "form-data", contents: {} });
      expect(state.formPairs()).toBe(first);
    });
  });

  it("writes rows to the field matching the body type", () => {
    withState((state) => {
      state.setBody({ type: "form-data", contents: {} });
      state.setFormPairs([{ key: "a", value: "1", enabled: true }]);
      expect(state.body().formData).toEqual([
        { key: "a", value: "1", enabled: true },
      ]);
      expect(state.body().formUrlEncoded).toBeUndefined();

      state.setBody({ ...state.body(), type: "form-urlencoded" });
      state.setFormPairs([{ key: "b", value: "2", enabled: true }]);
      expect(state.formPairs()).toEqual([
        { key: "b", value: "2", enabled: true },
      ]);
      // 型を戻しても form-data 側の行は残る。
      state.setBody({ ...state.body(), type: "form-data" });
      expect(state.formPairs()).toEqual([
        { key: "a", value: "1", enabled: true },
      ]);
    });
  });

  it("keeps rows with an empty key", () => {
    withState((state) => {
      state.setBody({ type: "form-data", contents: {} });
      state.setFormPairs([{ key: "", value: "", enabled: true }]);
      expect(state.formPairs()).toHaveLength(1);
    });
  });

  it("ignores row writes when the body type is not a form", () => {
    withState((state) => {
      state.setBody({ type: "json", contents: {} });
      state.setFormPairs([{ key: "a", value: "1", enabled: true }]);
      expect(state.body().formData).toBeUndefined();
      expect(state.body().formUrlEncoded).toBeUndefined();
      expect(state.formPairs()).toEqual([]);
    });
  });
});
