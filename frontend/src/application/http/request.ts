import { createSignal } from "solid-js";
import type { Logger } from "../../application/logger";
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
import { withLoading } from "../../shared/async-op";

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
  const [response, setResponse] = createSignal<HttpResponse | null>(null);
  const [loading, setLoading] = createSignal(false);
  const [activeRequestId, setActiveRequestId] = createSignal<string | null>(
    null,
  );
  const [activeCollectionId, setActiveCollectionId] = createSignal<
    string | null
  >(null);

  async function sendRequest(): Promise<void> {
    const m = method();
    const u = url();
    setResponse(null);
    logger.info("HTTP request sent", { method: m, url: u });
    try {
      const res = await withLoading(setLoading, () =>
        api.sendRequest({
          id: activeRequestId() ?? "",
          name: "",
          method: m,
          url: u,
          headers: headers(),
          params: params(),
          body: body(),
          auth: auth(),
          settings: settings(),
        }),
      );
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
      });
      logger.error("HTTP request failed", {
        method: m,
        url: u,
        error: errorMsg,
      });
    }
  }

  async function cancelRequest(): Promise<void> {
    const id = activeRequestId();
    if (!id) return;
    await api.cancelRequest(id);
  }

  function loadRequest(req: HttpRequest, collectionId: string): void {
    saveCurrentRequest().catch((err) =>
      console.error("Failed to save current request", err),
    );
    setMethod(req.method);
    setUrl(req.url);
    setHeaders(req.headers);
    setParams(req.params);
    setBody(req.body);
    setAuth(req.auth);
    setSettings(req.settings ?? { ...DEFAULT_SETTINGS });
    setActiveRequestId(req.id);
    setActiveCollectionId(collectionId);
  }

  function newRequest(): void {
    saveCurrentRequest().catch((err) =>
      console.error("Failed to save current request", err),
    );
    setMethod("GET");
    setUrl("");
    setHeaders([]);
    setParams([]);
    setBody({ type: "none", contents: {} });
    setAuth({ type: "none", username: "", password: "", token: "" });
    setSettings({ ...DEFAULT_SETTINGS });
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
    };
    await api.updateRequest(colId, req);
    api.afterSave?.(colId, req);
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
    response,
    loading,
    activeRequestId,
    activeCollectionId,
    sendRequest,
    cancelRequest,
    loadRequest,
    newRequest,
    saveCurrentRequest,
  };
}
