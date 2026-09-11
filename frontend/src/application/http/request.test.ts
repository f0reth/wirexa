import { createRoot } from "solid-js";
import { describe, expect, it, vi } from "vitest";
import type {
  HttpRequest,
  HttpResponse,
  RequestBody,
} from "../../domain/http/types";
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
  // 送信順の index で個別に解決できるよう、解決済みの枠は undefined にして詰めない。
  const pending: Array<((res: HttpResponse) => void) | undefined> = [];
  const api = {
    sendRequest: (req: HttpRequest) => {
      sent.push(req);
      return new Promise<HttpResponse>((resolve) => {
        pending.push(resolve);
      });
    },
    cancelRequest: vi.fn(async (_id: string) => {}),
    updateRequest: vi.fn(async (_colId: string, _req: HttpRequest) => {}),
    saveResponseBody: vi.fn(async (_executionId: string) => true),
    saveResponseBinary: vi.fn(async (_body: string, _ct: string) => {}),
    discardResponseBody: vi.fn(async (_executionId: string) => {}),
  } satisfies RequestApi;
  return {
    api,
    sent,
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

describe("createRequestState response body lifecycle", () => {
  it("discards the previous truncated body when a new send starts", async () => {
    await createRoot(async (dispose) => {
      const { api, sent, settle } = makeApi();
      const state = createRequestState(api, noopLogger);

      const first = state.sendRequest();
      settle(0, makeTruncatedResponse());
      await first;

      const second = state.sendRequest();
      expect(api.discardResponseBody).toHaveBeenCalledWith(sent[0].id);
      settle(1, makeResponse());
      await second;
      dispose();
    });
  });

  it("does not discard bodies the backend is not tracking", async () => {
    await createRoot(async (dispose) => {
      const { api, settle } = makeApi();
      const state = createRequestState(api, noopLogger);

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
      const { api, sent, settle } = makeApi();
      const state = createRequestState(api, noopLogger);

      const send = state.sendRequest();
      settle(0, makeTruncatedResponse());
      await send;
      await state.saveResponseBody();

      expect(api.saveResponseBody).toHaveBeenCalledWith(sent[0].id);
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
      const state = createRequestState(api, noopLogger);

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
      const state = createRequestState(api, noopLogger);

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
      const { api, sent, settle } = makeApi();
      const state = createRequestState(api, noopLogger);
      state.loadRequest(makeRequest("a"), "col-1");

      const send = state.sendRequest();
      settle(0, makeTruncatedResponse());
      await send;

      // 同じリクエストを開き直しただけでは破棄しない。
      state.loadRequest(makeRequest("a"), "col-1");
      expect(api.discardResponseBody).not.toHaveBeenCalled();
      expect(state.response()).not.toBeNull();

      state.loadRequest(makeRequest("b"), "col-1");
      expect(api.discardResponseBody).toHaveBeenCalledWith(sent[0].id);
      expect(state.response()).toBeNull();
      dispose();
    });
  });

  it("keeps the displayed response paired with its execution ID when responses arrive out of order", async () => {
    await createRoot(async (dispose) => {
      const { api, sent, settle } = makeApi();
      const state = createRequestState(api, noopLogger);

      const first = state.sendRequest();
      const second = state.sendRequest();
      settle(1, makeTruncatedResponse("second"));
      await second;
      settle(0, makeTruncatedResponse("first"));
      await first;

      expect(state.response()?.body).toBe("first");
      expect(api.discardResponseBody).toHaveBeenCalledWith(sent[1].id);
      await state.saveResponseBody();
      expect(api.saveResponseBody).toHaveBeenCalledWith(sent[0].id);
      dispose();
    });
  });

  it("saves a non-truncated binary body from memory", async () => {
    await createRoot(async (dispose) => {
      const { api, settle } = makeApi();
      const state = createRequestState(api, noopLogger);

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
        const state = createRequestState(api, noopLogger);
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
      const state = createRequestState(api, noopLogger);
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
      const state = createRequestState(api, noopLogger);
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
