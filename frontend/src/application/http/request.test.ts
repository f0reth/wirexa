import { createRoot } from "solid-js";
import { describe, expect, it, vi } from "vitest";
import type {
  HttpRequest,
  HttpResponse,
  RequestBody,
} from "../../domain/http/types";
import { DEFAULT_SETTINGS } from "../../domain/http/types";
import type { Notifier } from "../../domain/ui/ports";
import type { Logger } from "../logger";
import {
  createRequestState,
  type RequestApi,
  type RequestState,
} from "./request";

const noopLogger: Logger = { info: () => {}, error: () => {} };

function makeNotifier(): Notifier {
  return {
    error: vi.fn(),
    success: vi.fn(),
    info: vi.fn(),
    warning: vi.fn(),
  };
}

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

function makeTruncatedResponse(body = "partial"): HttpResponse {
  return { ...makeResponse(), body, bodyTruncated: true };
}

/** sendRequest を任意のタイミングで解決できる API を作る。 */
function makeApi() {
  const sent: HttpRequest[] = [];
  // 送信ごとの execution ID。sent と同じ index で対応する。
  const execIds: string[] = [];
  // 送信順の index で個別に解決できるよう、解決済みの枠は undefined にして詰めない。
  const pending: Array<((res: HttpResponse) => void) | undefined> = [];
  const api = {
    sendRequest: (executionId: string, req: HttpRequest) => {
      execIds.push(executionId);
      sent.push(req);
      return new Promise<HttpResponse>((resolve) => {
        pending.push(resolve);
      });
    },
    cancelRequest: vi.fn(async (_id: string) => {}),
    updateRequest: vi.fn(async (_colId: string, _req: HttpRequest) => {}),
    openFilePicker: vi.fn(async (_hint: string) => undefined),
    saveResponseBody: vi.fn(async (_executionId: string) => true),
    saveResponseBinary: vi.fn(async (_body: string, _ct: string) => {}),
    discardResponseBody: vi.fn(async (_executionId: string) => {}),
  } satisfies RequestApi;
  return {
    api,
    sent,
    execIds,
    /** 送信済みリクエストをすべて解決する。 */
    settleAll: async () => {
      pending.forEach((resolve, i) => {
        resolve?.(makeResponse());
        pending[i] = undefined;
      });
      await Promise.resolve();
    },
    /** index 番目の送信を指定したレスポンスで解決する。 */
    settle: (index: number, res: HttpResponse) => {
      pending[index]?.(res);
      pending[index] = undefined;
    },
  };
}

describe("createRequestState execution id", () => {
  it("assigns a fresh execution id per request instead of an empty id", async () => {
    await createRoot(async (dispose) => {
      const { api, execIds, settleAll } = makeApi();
      const state = createRequestState(api, noopLogger, makeNotifier());
      state.setUrl("https://example.com");

      const first = state.sendRequest();
      const second = state.sendRequest();
      await settleAll();
      await Promise.all([first, second]);

      expect(execIds).toHaveLength(2);
      expect(execIds[0]).not.toBe("");
      expect(execIds[1]).not.toBe("");
      expect(execIds[0]).not.toBe(execIds[1]);
      dispose();
    });
  });

  it("sends the saved request id separately from the execution id", async () => {
    await createRoot(async (dispose) => {
      const { api, sent, execIds, settleAll } = makeApi();
      const state = createRequestState(api, noopLogger, makeNotifier());
      state.loadRequest(makeRequest("saved-1"), "col-1");

      const done = state.sendRequest();
      await settleAll();
      await done;

      expect(sent[0].id).toBe("saved-1");
      expect(execIds[0]).not.toBe("saved-1");
      expect(state.activeRequestId()).toBe("saved-1");
      dispose();
    });
  });

  it("sends an empty request id when the request is not saved", async () => {
    await createRoot(async (dispose) => {
      const { api, sent, execIds, settleAll } = makeApi();
      const state = createRequestState(api, noopLogger, makeNotifier());
      state.setUrl("https://example.com");

      const done = state.sendRequest();
      await settleAll();
      await done;

      expect(sent[0].id).toBe("");
      expect(execIds[0]).not.toBe("");
      dispose();
    });
  });

  it("cancels every in-flight send id", async () => {
    await createRoot(async (dispose) => {
      const { api, execIds, settleAll } = makeApi();
      const state = createRequestState(api, noopLogger, makeNotifier());

      const first = state.sendRequest();
      const second = state.sendRequest();
      expect(state.loading()).toBe(true);

      await state.cancelRequest();
      expect(api.cancelRequest).toHaveBeenCalledTimes(2);
      expect(api.cancelRequest).toHaveBeenCalledWith(execIds[0]);
      expect(api.cancelRequest).toHaveBeenCalledWith(execIds[1]);

      await settleAll();
      await Promise.all([first, second]);
      expect(state.loading()).toBe(false);
      dispose();
    });
  });

  it("keeps loading true until the last in-flight request settles", async () => {
    await createRoot(async (dispose) => {
      const { api, settleAll } = makeApi();
      const state = createRequestState(api, noopLogger, makeNotifier());

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
    fn(createRequestState(makeApi().api, noopLogger, makeNotifier()));
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

describe("createRequestState response body lifecycle", () => {
  it("discards the previous truncated body when a new send starts", async () => {
    await createRoot(async (dispose) => {
      const { api, execIds, settle } = makeApi();
      const state = createRequestState(api, noopLogger, makeNotifier());

      const first = state.sendRequest();
      settle(0, makeTruncatedResponse());
      await first;

      const second = state.sendRequest();
      expect(api.discardResponseBody).toHaveBeenCalledWith(execIds[0]);
      settle(1, makeResponse());
      await second;
      dispose();
    });
  });

  it("does not discard bodies the backend is not tracking", async () => {
    await createRoot(async (dispose) => {
      const { api, settle } = makeApi();
      const state = createRequestState(api, noopLogger, makeNotifier());

      const first = state.sendRequest();
      settle(0, makeResponse());
      await first;
      const second = state.sendRequest();
      settle(1, makeResponse());
      await second;

      expect(api.discardResponseBody).not.toHaveBeenCalled();
      dispose();
    });
  });

  it("saves by execution ID and does not discard after a successful save", async () => {
    await createRoot(async (dispose) => {
      const { api, execIds, settle } = makeApi();
      const state = createRequestState(api, noopLogger, makeNotifier());

      const send = state.sendRequest();
      settle(0, makeTruncatedResponse());
      await send;
      await state.saveResponseBody();

      expect(api.saveResponseBody).toHaveBeenCalledWith(execIds[0]);
      expect(state.responseSaveState()).toBe("saved");

      state.newRequest();
      expect(api.discardResponseBody).not.toHaveBeenCalled();
      expect(state.response()).toBeNull();
      dispose();
    });
  });

  it("keeps the body saveable after the save dialog is canceled", async () => {
    await createRoot(async (dispose) => {
      const { api, settle } = makeApi();
      api.saveResponseBody.mockResolvedValueOnce(false);
      const state = createRequestState(api, noopLogger, makeNotifier());

      const send = state.sendRequest();
      settle(0, makeTruncatedResponse());
      await send;
      await state.saveResponseBody();

      expect(state.responseSaveState()).toBe("idle");
      dispose();
    });
  });

  it("marks the body unavailable when the backend has reclaimed it", async () => {
    await createRoot(async (dispose) => {
      const { api, settle } = makeApi();
      api.saveResponseBody.mockRejectedValueOnce(
        new Error("response body unavailable"),
      );
      const state = createRequestState(api, noopLogger, makeNotifier());

      const send = state.sendRequest();
      settle(0, makeTruncatedResponse());
      await send;
      await state.saveResponseBody();

      expect(state.responseSaveState()).toBe("unavailable");
      dispose();
    });
  });

  it("clears and discards the response when switching to another request", async () => {
    await createRoot(async (dispose) => {
      const { api, execIds, settle } = makeApi();
      const state = createRequestState(api, noopLogger, makeNotifier());
      state.loadRequest(makeRequest("a"), "col-1");

      const send = state.sendRequest();
      settle(0, makeTruncatedResponse());
      await send;

      // 同じリクエストを開き直しただけでは破棄しない。
      state.loadRequest(makeRequest("a"), "col-1");
      expect(api.discardResponseBody).not.toHaveBeenCalled();
      expect(state.response()).not.toBeNull();

      state.loadRequest(makeRequest("b"), "col-1");
      expect(api.discardResponseBody).toHaveBeenCalledWith(execIds[0]);
      expect(state.response()).toBeNull();
      dispose();
    });
  });

  it("keeps the displayed response paired with its execution ID when responses arrive out of order", async () => {
    await createRoot(async (dispose) => {
      const { api, execIds, settle } = makeApi();
      const state = createRequestState(api, noopLogger, makeNotifier());

      const first = state.sendRequest();
      const second = state.sendRequest();
      settle(1, makeTruncatedResponse("second"));
      await second;
      settle(0, makeTruncatedResponse("first"));
      await first;

      expect(state.response()?.body).toBe("first");
      expect(api.discardResponseBody).toHaveBeenCalledWith(execIds[1]);
      await state.saveResponseBody();
      expect(api.saveResponseBody).toHaveBeenCalledWith(execIds[0]);
      dispose();
    });
  });

  it("saves a non-truncated binary body from memory", async () => {
    await createRoot(async (dispose) => {
      const { api, settle } = makeApi();
      const state = createRequestState(api, noopLogger, makeNotifier());

      const send = state.sendRequest();
      settle(0, {
        ...makeResponse(),
        body: "AAEC",
        bodyBase64: true,
        contentType: "application/octet-stream",
      });
      await send;
      await state.saveResponseBody();

      expect(api.saveResponseBinary).toHaveBeenCalledWith(
        "AAEC",
        "application/octet-stream",
      );
      expect(api.saveResponseBody).not.toHaveBeenCalled();
      dispose();
    });
  });
});

describe("createRequestState file confirmation", () => {
  const blocked: Array<[string, RequestBody]> = [
    [
      "a typed path that was never confirmed",
      { type: "file", contents: {}, file: { hint: "/tmp/a.bin" } },
    ],
    [
      "a saved file that needs reselecting",
      {
        type: "file",
        contents: {},
        file: { name: "a.bin", needsReselect: true },
      },
    ],
    [
      "an unconfirmed form-data file row",
      {
        type: "form-data",
        contents: {},
        formData: [
          {
            key: "f",
            value: "",
            kind: "file",
            enabled: true,
            file: { hint: "/tmp/b.bin" },
          },
        ],
      },
    ],
  ];

  for (const [label, body] of blocked) {
    it(`does not send ${label}`, async () => {
      await createRoot(async (dispose) => {
        const { api, sent } = makeApi();
        const state = createRequestState(api, noopLogger, makeNotifier());
        state.setBody(body);

        await state.sendRequest();

        expect(sent).toHaveLength(0);
        expect(state.response()?.error).toContain("Browse");
        dispose();
      });
    });
  }

  it("ignores disabled and keyless file rows", async () => {
    await createRoot(async (dispose) => {
      const { api, sent, settleAll } = makeApi();
      const state = createRequestState(api, noopLogger, makeNotifier());
      state.setBody({
        type: "form-data",
        contents: {},
        formData: [
          {
            key: "",
            value: "",
            kind: "file",
            enabled: true,
            file: { hint: "x" },
          },
          {
            key: "f",
            value: "",
            kind: "file",
            enabled: false,
            file: { hint: "y" },
          },
        ],
      });

      const send = state.sendRequest();
      await settleAll();
      await send;

      expect(sent).toHaveLength(1);
      dispose();
    });
  });

  it("sends a confirmed file by its token", async () => {
    await createRoot(async (dispose) => {
      const { api, sent, settleAll } = makeApi();
      const state = createRequestState(api, noopLogger, makeNotifier());
      state.setBody({
        type: "file",
        contents: {},
        file: { token: "tok", name: "a.bin" },
      });

      const send = state.sendRequest();
      await settleAll();
      await send;

      expect(sent[0].body.file?.token).toBe("tok");
      dispose();
    });
  });
});
