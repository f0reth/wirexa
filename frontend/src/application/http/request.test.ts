import { createRoot } from "solid-js";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type {
  FileReference,
  HttpRequest,
  HttpResponse,
  RequestBody,
} from "../../domain/http/types";
import { DEFAULT_SETTINGS } from "../../domain/http/types";
import type { Notifier } from "../../domain/ui/ports";
import type { Logger } from "../logger";
import {
  createAutoSaveEffect,
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
  const pending: Array<
    | { resolve: (res: HttpResponse) => void; reject: (err: unknown) => void }
    | undefined
  > = [];
  const api = {
    sendRequest: (executionId: string, req: HttpRequest) => {
      execIds.push(executionId);
      sent.push(req);
      return new Promise<HttpResponse>((resolve, reject) => {
        pending.push({ resolve, reject });
      });
    },
    cancelRequest: vi.fn(async (_id: string) => {}),
    updateRequest: vi.fn(async (_colId: string, _req: HttpRequest) => {}),
    openFilePicker: vi.fn(
      async (_hint: string): Promise<FileReference | undefined> => undefined,
    ),
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
      pending.forEach((p, i) => {
        p?.resolve(makeResponse());
        pending[i] = undefined;
      });
      await Promise.resolve();
    },
    /** index 番目の送信を指定したレスポンスで解決する。 */
    settle: (index: number, res: HttpResponse) => {
      pending[index]?.resolve(res);
      pending[index] = undefined;
    },
    /** index 番目の送信を指定した値で失敗させる。 */
    fail: (index: number, err: unknown) => {
      pending[index]?.reject(err);
      pending[index] = undefined;
    },
  };
}

/** 呼び出しを記録する logger。 */
function makeLogger() {
  return { info: vi.fn(), error: vi.fn() } satisfies Logger;
}

/** 保留中の Promise の後続処理（catch など）を流す。 */
async function flush(): Promise<void> {
  await new Promise((resolve) => setTimeout(resolve, 0));
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

  it("ignores body content writes for file bodies", () => {
    withState((state) => {
      const body: RequestBody = {
        type: "file",
        contents: { text: "kept" },
        file: { token: "t" },
      };
      state.setBody(body);
      expect(state.bodyContent()).toBe("");

      state.setBodyContent("ignored");
      expect(state.body()).toBe(body);
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

describe("createRequestState send", () => {
  it("sends the current editor state", async () => {
    await createRoot(async (dispose) => {
      const { api, sent, settleAll } = makeApi();
      const state = createRequestState(api, noopLogger, makeNotifier());
      state.loadRequest(makeRequest("saved-1"), "col-1");
      state.setMethod("POST");
      state.setUrl("https://api.example.com/users");
      state.setHeaders([{ key: "X-A", value: "1", enabled: true }]);
      state.setParams([{ key: "q", value: "x", enabled: false }]);
      state.setBody({ type: "text", contents: { text: "hello" } });
      state.setAuth({ type: "bearer", username: "", password: "", token: "t" });
      state.setSettings({ ...DEFAULT_SETTINGS, timeoutSec: 5 });
      state.setDoc("memo");

      const done = state.sendRequest();
      await settleAll();
      await done;

      expect(sent[0]).toEqual({
        id: "saved-1",
        name: "",
        method: "POST",
        url: "https://api.example.com/users",
        headers: [{ key: "X-A", value: "1", enabled: true }],
        params: [{ key: "q", value: "x", enabled: false }],
        body: { type: "text", contents: { text: "hello" } },
        auth: { type: "bearer", username: "", password: "", token: "t" },
        settings: { ...DEFAULT_SETTINGS, timeoutSec: 5 },
        doc: "memo",
      });
      dispose();
    });
  });

  it("shows an error response and stops loading when sending fails", async () => {
    await createRoot(async (dispose) => {
      const { api, fail } = makeApi();
      const logger = makeLogger();
      const state = createRequestState(api, logger, makeNotifier());

      const done = state.sendRequest();
      expect(state.loading()).toBe(true);
      fail(0, new Error("dial tcp: connection refused"));
      await done;

      expect(state.loading()).toBe(false);
      expect(state.response()).toMatchObject({
        statusCode: 0,
        error: "dial tcp: connection refused",
        bodyTruncated: false,
      });
      expect(logger.error).toHaveBeenCalledWith(
        "HTTP request failed",
        expect.objectContaining({ error: "dial tcp: connection refused" }),
      );
      dispose();
    });
  });

  it("stringifies a non-Error rejection", async () => {
    await createRoot(async (dispose) => {
      const { api, fail } = makeApi();
      const state = createRequestState(api, noopLogger, makeNotifier());

      const done = state.sendRequest();
      fail(0, "canceled");
      await done;

      expect(state.response()?.error).toBe("canceled");
      dispose();
    });
  });

  it("does not show a late response after switching to another request", async () => {
    await createRoot(async (dispose) => {
      const { api, execIds, settle } = makeApi();
      const state = createRequestState(api, noopLogger, makeNotifier());
      state.loadRequest(makeRequest("a"), "col-1");

      const done = state.sendRequest();
      state.loadRequest(makeRequest("b"), "col-1");
      settle(0, makeTruncatedResponse("late"));
      await done;

      expect(state.response()).toBeNull();
      expect(state.loading()).toBe(false);
      // 表示しない全文は backend に残さない。
      expect(api.discardResponseBody).toHaveBeenCalledWith(execIds[0]);
      dispose();
    });
  });

  it("does not show a late error after starting a new request", async () => {
    await createRoot(async (dispose) => {
      const { api, fail } = makeApi();
      const state = createRequestState(api, noopLogger, makeNotifier());
      state.loadRequest(makeRequest("a"), "col-1");

      const done = state.sendRequest();
      state.newRequest();
      fail(0, new Error("timeout"));
      await done;

      expect(state.response()).toBeNull();
      expect(api.discardResponseBody).not.toHaveBeenCalled();
      dispose();
    });
  });

  it("still shows the response after reopening the same request", async () => {
    await createRoot(async (dispose) => {
      const { api, settle } = makeApi();
      const state = createRequestState(api, noopLogger, makeNotifier());
      state.loadRequest(makeRequest("a"), "col-1");

      const done = state.sendRequest();
      state.loadRequest(makeRequest("a"), "col-1");
      settle(0, makeTruncatedResponse("same"));
      await done;

      expect(state.response()?.body).toBe("same");
      expect(api.discardResponseBody).not.toHaveBeenCalled();
      dispose();
    });
  });

  it("rejects when one of the cancellations fails", async () => {
    await createRoot(async (dispose) => {
      const { api, settleAll } = makeApi();
      api.cancelRequest.mockRejectedValueOnce(new Error("unknown id"));
      const state = createRequestState(api, noopLogger, makeNotifier());

      const first = state.sendRequest();
      const second = state.sendRequest();
      await expect(state.cancelRequest()).rejects.toThrow("unknown id");
      expect(api.cancelRequest).toHaveBeenCalledTimes(2);

      await settleAll();
      await Promise.all([first, second]);
      dispose();
    });
  });
});

describe("createRequestState pickFile", () => {
  it("returns the confirmed reference", async () => {
    await createRoot(async (dispose) => {
      const { api } = makeApi();
      api.openFilePicker.mockResolvedValueOnce({ token: "t", name: "a.bin" });
      const notifier = makeNotifier();
      const state = createRequestState(api, noopLogger, notifier);

      await expect(state.pickFile("C:\\a.bin")).resolves.toEqual({
        token: "t",
        name: "a.bin",
      });
      expect(api.openFilePicker).toHaveBeenCalledWith("C:\\a.bin");
      expect(notifier.error).not.toHaveBeenCalled();
      dispose();
    });
  });

  it("notifies and returns undefined when the picker fails", async () => {
    await createRoot(async (dispose) => {
      const { api } = makeApi();
      api.openFilePicker.mockRejectedValueOnce(new Error("dialog"));
      const notifier = makeNotifier();
      const state = createRequestState(api, noopLogger, notifier);

      await expect(state.pickFile("")).resolves.toBeUndefined();
      expect(notifier.error).toHaveBeenCalledWith(
        "Failed to open file picker",
        "dialog",
      );
      dispose();
    });
  });
});

describe("createRequestState response save errors", () => {
  it("does nothing without a response", async () => {
    await createRoot(async (dispose) => {
      const { api } = makeApi();
      const notifier = makeNotifier();
      const state = createRequestState(api, noopLogger, notifier);

      await state.saveResponseBody();

      expect(api.saveResponseBody).not.toHaveBeenCalled();
      expect(api.saveResponseBinary).not.toHaveBeenCalled();
      expect(notifier.error).not.toHaveBeenCalled();
      dispose();
    });
  });

  it("notifies non-reclaim save errors", async () => {
    await createRoot(async (dispose) => {
      const { api, settle } = makeApi();
      api.saveResponseBody.mockRejectedValueOnce(new Error("disk full"));
      const notifier = makeNotifier();
      const state = createRequestState(api, noopLogger, notifier);

      const send = state.sendRequest();
      settle(0, makeTruncatedResponse());
      await send;
      await state.saveResponseBody();

      expect(notifier.error).toHaveBeenCalledWith(
        "Failed to save response",
        "disk full",
      );
      expect(state.responseSaveState()).toBe("idle");
      dispose();
    });
  });

  it("notifies a binary save failure even with the reclaim message", async () => {
    await createRoot(async (dispose) => {
      const { api, settle } = makeApi();
      api.saveResponseBinary.mockRejectedValueOnce(
        new Error("response body unavailable"),
      );
      const notifier = makeNotifier();
      const state = createRequestState(api, noopLogger, notifier);

      const send = state.sendRequest();
      settle(0, { ...makeResponse(), body: "AAEC", bodyBase64: true });
      await send;
      await state.saveResponseBody();

      expect(notifier.error).toHaveBeenCalledWith(
        "Failed to save response",
        "response body unavailable",
      );
      expect(state.responseSaveState()).toBe("idle");
      dispose();
    });
  });

  it("does not mark saved when the response changed during the save", async () => {
    await createRoot(async (dispose) => {
      const { api, settle } = makeApi();
      let finishSave: (saved: boolean) => void = () => {};
      api.saveResponseBody.mockImplementationOnce(
        () =>
          new Promise<boolean>((resolve) => {
            finishSave = resolve;
          }),
      );
      const state = createRequestState(api, noopLogger, makeNotifier());

      const first = state.sendRequest();
      settle(0, makeTruncatedResponse("first"));
      await first;
      const saving = state.saveResponseBody();

      const second = state.sendRequest();
      settle(1, makeTruncatedResponse("second"));
      await second;
      finishSave(true);
      await saving;

      expect(state.response()?.body).toBe("second");
      expect(state.responseSaveState()).toBe("idle");
      dispose();
    });
  });

  it("does not mark unavailable when the response changed during the save", async () => {
    await createRoot(async (dispose) => {
      const { api, settle } = makeApi();
      let failSave: (err: Error) => void = () => {};
      api.saveResponseBody.mockImplementationOnce(
        () =>
          new Promise<boolean>((_, reject) => {
            failSave = reject;
          }),
      );
      const notifier = makeNotifier();
      const state = createRequestState(api, noopLogger, notifier);

      const first = state.sendRequest();
      settle(0, makeTruncatedResponse("first"));
      await first;
      const saving = state.saveResponseBody();

      const second = state.sendRequest();
      settle(1, makeTruncatedResponse("second"));
      await second;
      failSave(new Error("response body unavailable"));
      await saving;

      expect(state.responseSaveState()).toBe("idle");
      expect(notifier.error).not.toHaveBeenCalled();
      dispose();
    });
  });
});

describe("createRequestState discard", () => {
  it("does not discard a body the backend already reclaimed", async () => {
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

      state.newRequest();
      expect(api.discardResponseBody).not.toHaveBeenCalled();
      expect(state.responseSaveState()).toBe("idle");
      dispose();
    });
  });

  it("logs a failed discard", async () => {
    await createRoot(async (dispose) => {
      const { api, settle } = makeApi();
      api.discardResponseBody.mockRejectedValueOnce(new Error("gone"));
      const logger = makeLogger();
      const state = createRequestState(api, logger, makeNotifier());

      const send = state.sendRequest();
      settle(0, makeTruncatedResponse());
      await send;
      state.newRequest();
      await flush();

      expect(logger.error).toHaveBeenCalledWith(
        "Failed to discard response body",
        { error: "gone" },
      );
      dispose();
    });
  });
});

describe("createRequestState load and save", () => {
  it("fills default settings and doc for legacy requests", () => {
    withState((state) => {
      const legacy = { ...makeRequest("old") } as Partial<HttpRequest>;
      delete legacy.settings;
      delete legacy.doc;

      state.loadRequest(legacy as HttpRequest, "col-1");

      expect(state.settings()).toEqual(DEFAULT_SETTINGS);
      expect(state.settings()).not.toBe(DEFAULT_SETTINGS);
      expect(state.doc()).toBe("");
    });
  });

  it("saves the previous request before switching", async () => {
    await createRoot(async (dispose) => {
      const { api } = makeApi();
      const state = createRequestState(api, noopLogger, makeNotifier());
      state.loadRequest(makeRequest("a"), "col-1");
      state.setUrl("https://edited.example");

      state.loadRequest(makeRequest("b"), "col-2");

      expect(api.updateRequest).toHaveBeenCalledTimes(1);
      expect(api.updateRequest).toHaveBeenCalledWith(
        "col-1",
        expect.objectContaining({ id: "a", url: "https://edited.example" }),
      );
      expect(state.activeRequestId()).toBe("b");
      expect(state.activeCollectionId()).toBe("col-2");
      dispose();
    });
  });

  it("clears the response when only the collection differs", async () => {
    await createRoot(async (dispose) => {
      const { api, execIds, settle } = makeApi();
      const state = createRequestState(api, noopLogger, makeNotifier());
      state.loadRequest(makeRequest("a"), "col-1");

      const send = state.sendRequest();
      settle(0, makeTruncatedResponse());
      await send;
      state.loadRequest(makeRequest("a"), "col-2");

      expect(state.response()).toBeNull();
      expect(api.discardResponseBody).toHaveBeenCalledWith(execIds[0]);
      dispose();
    });
  });

  it("notifies when saving the previous request fails on switch", async () => {
    await createRoot(async (dispose) => {
      const { api } = makeApi();
      api.updateRequest.mockRejectedValue(new Error("locked"));
      const notifier = makeNotifier();
      const state = createRequestState(api, noopLogger, notifier);
      state.loadRequest(makeRequest("a"), "col-1");

      state.loadRequest(makeRequest("b"), "col-1");
      await flush();
      expect(notifier.error).toHaveBeenCalledWith(
        "Failed to save request",
        "locked",
      );

      state.newRequest();
      await flush();
      expect(notifier.error).toHaveBeenCalledTimes(2);
      dispose();
    });
  });

  it("does not call updateRequest for an unsaved request", async () => {
    await createRoot(async (dispose) => {
      const { api } = makeApi();
      const state = createRequestState(api, noopLogger, makeNotifier());
      state.setUrl("https://example.com");

      await state.saveCurrentRequest();

      expect(api.updateRequest).not.toHaveBeenCalled();
      dispose();
    });
  });

  it("calls afterSave with the saved request and clears saveError", async () => {
    await createRoot(async (dispose) => {
      const { api } = makeApi();
      const afterSave = vi.fn();
      api.updateRequest.mockRejectedValueOnce(new Error("locked"));
      const state = createRequestState(
        { ...api, afterSave },
        noopLogger,
        makeNotifier(),
      );
      state.loadRequest(makeRequest("a"), "col-1");
      await expect(state.saveCurrentRequest()).rejects.toThrow("locked");
      expect(state.saveError()).toBe("locked");
      expect(afterSave).not.toHaveBeenCalled();

      await state.saveCurrentRequest();

      expect(state.saveError()).toBeNull();
      expect(afterSave).toHaveBeenCalledWith("col-1", {
        ...makeRequest("a"),
        name: "",
      });
      dispose();
    });
  });

  it("records saveError and rethrows the original non-Error value", async () => {
    await createRoot(async (dispose) => {
      const { api } = makeApi();
      api.updateRequest.mockRejectedValueOnce("offline");
      const state = createRequestState(api, noopLogger, makeNotifier());
      state.loadRequest(makeRequest("a"), "col-1");

      await expect(state.saveCurrentRequest()).rejects.toBe("offline");
      expect(state.saveError()).toBe("offline");

      state.clearSaveError();
      expect(state.saveError()).toBeNull();
      dispose();
    });
  });
});

describe("createAutoSaveEffect", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  function setup() {
    const { api } = makeApi();
    const notifier = makeNotifier();
    let state!: RequestState;
    const dispose = createRoot((dispose) => {
      state = createRequestState(api, noopLogger, notifier);
      createAutoSaveEffect(state, notifier, 500);
      return dispose;
    });
    return { api, notifier, state, dispose };
  }

  it("saves once after the debounce window", async () => {
    const { api, state, dispose } = setup();
    state.loadRequest(makeRequest("a"), "col-1");
    // loadRequest 直後の effect による保存を済ませてから編集を数える。
    await vi.advanceTimersByTimeAsync(500);
    api.updateRequest.mockClear();

    state.setUrl("https://1.example");
    await vi.advanceTimersByTimeAsync(499);
    expect(api.updateRequest).not.toHaveBeenCalled();

    await vi.advanceTimersByTimeAsync(1);
    expect(api.updateRequest).toHaveBeenCalledTimes(1);
    expect(api.updateRequest).toHaveBeenCalledWith(
      "col-1",
      expect.objectContaining({ url: "https://1.example" }),
    );
    dispose();
  });

  it("coalesces edits within the debounce window into the last one", async () => {
    const { api, state, dispose } = setup();
    state.loadRequest(makeRequest("a"), "col-1");
    await vi.advanceTimersByTimeAsync(500);
    api.updateRequest.mockClear();

    state.setUrl("https://1.example");
    await vi.advanceTimersByTimeAsync(300);
    state.setUrl("https://2.example");
    await vi.advanceTimersByTimeAsync(300);
    state.setDoc("memo");
    await vi.advanceTimersByTimeAsync(500);

    expect(api.updateRequest).toHaveBeenCalledTimes(1);
    expect(api.updateRequest).toHaveBeenCalledWith(
      "col-1",
      expect.objectContaining({ url: "https://2.example", doc: "memo" }),
    );
    dispose();
  });

  it("does not save an unsaved (new) request", async () => {
    const { api, state, dispose } = setup();
    state.setUrl("https://1.example");
    state.setDoc("memo");
    await vi.advanceTimersByTimeAsync(1000);

    expect(api.updateRequest).not.toHaveBeenCalled();
    dispose();
  });

  it("notifies auto-save failures with a dedup key", async () => {
    const { api, notifier, state, dispose } = setup();
    api.updateRequest.mockRejectedValue(new Error("locked"));
    state.loadRequest(makeRequest("a"), "col-1");
    await vi.advanceTimersByTimeAsync(500);

    expect(notifier.error).toHaveBeenCalledWith(
      "Failed to auto-save request",
      "locked",
      { key: "http-autosave" },
    );
    expect(state.saveError()).toBe("locked");
    dispose();
  });

  it("drops the pending save when the owner is disposed", async () => {
    const { api, state, dispose } = setup();
    state.loadRequest(makeRequest("a"), "col-1");
    await vi.advanceTimersByTimeAsync(500);
    api.updateRequest.mockClear();

    state.setUrl("https://1.example");
    dispose();
    await vi.advanceTimersByTimeAsync(1000);

    expect(api.updateRequest).not.toHaveBeenCalled();
  });
});
