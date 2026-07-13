import { createEffect, createSignal, onCleanup } from "solid-js";
import type { Logger } from "../../application/logger";
import { notify } from "../../application/ui/notifications";
import type {
  HttpMethod,
  HttpRequest,
  HttpResponse,
  KeyValuePair,
  RequestAuth,
  RequestBody,
  RequestSettings,
} from "../../domain/http/types";
import { DEFAULT_SETTINGS } from "../../domain/http/types";
import { generateId } from "../../infrastructure/id/generator";
import { errorMessage } from "../../shared/error";

export interface RequestApi {
  sendRequest(req: HttpRequest): Promise<HttpResponse>;
  cancelRequest(id: string): Promise<void>;
  updateRequest(collectionId: string, req: HttpRequest): Promise<void>;
  afterSave?: (collectionId: string, req: HttpRequest) => void;
}

export function createRequestState(api: RequestApi, logger: Logger) {
  const [method, setMethod] = createSignal<HttpMethod>("GET");
  const [url, setUrl] = createSignal("");
  const [headers, setHeaders] = createSignal<KeyValuePair[]>([]);
  const [params, setParams] = createSignal<KeyValuePair[]>([]);
  const [body, setBody] = createSignal<RequestBody>({
    type: "none",
    contents: {},
  });
  const [auth, setAuth] = createSignal<RequestAuth>({
    type: "none",
    username: "",
    password: "",
    token: "",
  });
  const [settings, setSettings] = createSignal<RequestSettings>({
    ...DEFAULT_SETTINGS,
  });
  const [doc, setDoc] = createSignal("");
  const [response, setResponse] = createSignal<HttpResponse | null>(null);
  // 実行中リクエストの send ID。保存用 ID とは別に送信ごとに採番するため、
  // 同じリクエストを連続送信してもバックエンドの cancels マップでキーが衝突しない。
  const [inFlight, setInFlight] = createSignal<readonly string[]>([]);
  const loading = () => inFlight().length > 0;
  const [activeRequestId, setActiveRequestId] = createSignal<string | null>(
    null,
  );
  const [activeCollectionId, setActiveCollectionId] = createSignal<
    string | null
  >(null);
  const [saveError, setSaveError] = createSignal<string | null>(null);

  async function sendRequest(): Promise<void> {
    const m = method();
    const u = url();
    const sendId = generateId();
    setResponse(null);
    setInFlight((ids) => [...ids, sendId]);
    logger.info("HTTP request sent", { method: m, url: u });
    try {
      const res = await api.sendRequest({
        id: sendId,
        name: "",
        method: m,
        url: u,
        headers: headers(),
        params: params(),
        body: body(),
        auth: auth(),
        settings: settings(),
        doc: doc(),
      });
      setResponse(res);
      logger.info("HTTP response received", {
        method: m,
        url: u,
        status: res.statusCode,
        latency_ms: res.timingMs,
      });
    } catch (err) {
      const errorMsg = err instanceof Error ? err.message : String(err);
      setResponse({
        statusCode: 0,
        statusText: "",
        headers: {},
        body: "",
        contentType: "",
        size: 0,
        timingMs: 0,
        error: errorMsg,
        bodyTruncated: false,
        tempFilePath: "",
        bodyBase64: false,
        bodyCapped: false,
      });
      logger.error("HTTP request failed", {
        method: m,
        url: u,
        error: errorMsg,
      });
    } finally {
      setInFlight((ids) => ids.filter((id) => id !== sendId));
    }
  }

  async function cancelRequest(): Promise<void> {
    const ids = inFlight();
    if (ids.length === 0) return;
    await Promise.all(ids.map((id) => api.cancelRequest(id)));
  }

  function loadRequest(req: HttpRequest, collectionId: string): void {
    saveCurrentRequest().catch((err) =>
      notify.error("Failed to save request", errorMessage(err)),
    );
    setMethod(req.method);
    setUrl(req.url);
    setHeaders(req.headers);
    setParams(req.params);
    setBody(req.body);
    setAuth(req.auth);
    setSettings(req.settings ?? { ...DEFAULT_SETTINGS });
    setDoc(req.doc ?? "");
    setActiveRequestId(req.id);
    setActiveCollectionId(collectionId);
  }

  function newRequest(): void {
    saveCurrentRequest().catch((err) =>
      notify.error("Failed to save request", errorMessage(err)),
    );
    setMethod("GET");
    setUrl("");
    setHeaders([]);
    setParams([]);
    setBody({ type: "none", contents: {} });
    setAuth({ type: "none", username: "", password: "", token: "" });
    setSettings({ ...DEFAULT_SETTINGS });
    setDoc("");
    setActiveRequestId(null);
    setActiveCollectionId(null);
  }

  async function saveCurrentRequest(): Promise<void> {
    const id = activeRequestId();
    const colId = activeCollectionId();
    if (!id || !colId) return;
    const req = {
      id,
      name: "",
      method: method(),
      url: url(),
      headers: headers(),
      params: params(),
      body: body(),
      auth: auth(),
      settings: settings(),
      doc: doc(),
    };
    try {
      await api.updateRequest(colId, req);
      setSaveError(null);
      api.afterSave?.(colId, req);
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      setSaveError(msg);
      throw err;
    }
  }

  return {
    method,
    setMethod,
    url,
    setUrl,
    headers,
    setHeaders,
    params,
    setParams,
    body,
    setBody,
    auth,
    setAuth,
    settings,
    setSettings,
    doc,
    setDoc,
    response,
    loading,
    activeRequestId,
    activeCollectionId,
    saveError,
    clearSaveError: () => setSaveError(null),
    sendRequest,
    cancelRequest,
    loadRequest,
    newRequest,
    saveCurrentRequest,
  };
}

export type RequestState = ReturnType<typeof createRequestState>;

export function createAutoSaveEffect(
  state: RequestState,
  debounceMs = 500,
): void {
  let saveVersion = 0;
  createEffect(() => {
    state.method();
    state.url();
    state.headers();
    state.params();
    state.body();
    state.auth();
    state.settings();
    state.doc();

    if (!state.activeRequestId()) return;

    const version = ++saveVersion;
    const timer = setTimeout(() => {
      if (version !== saveVersion) return;
      state.saveCurrentRequest().catch((err) => {
        notify.error("Failed to auto-save request", errorMessage(err), {
          key: "http-autosave",
        });
      });
    }, debounceMs);

    onCleanup(() => clearTimeout(timer));
  });
}
