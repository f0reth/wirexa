import { createSignal } from "solid-js";
import type {
  HttpMethod,
  HttpRequest,
  HttpResponse,
  KeyValuePair,
  RequestAuth,
  RequestBody,
} from "../../domain/http/types";
import { log } from "../../infrastructure/logger/client";

export interface RequestApi {
  sendRequest(req: HttpRequest): Promise<HttpResponse>;
  cancelRequest(): Promise<void>;
  updateRequest(collectionId: string, req: HttpRequest): Promise<void>;
  afterSave?: () => Promise<void>;
}

export function createRequestState(api: RequestApi) {
  const [method, setMethod] = createSignal<HttpMethod>("GET");
  const [url, setUrl] = createSignal("");
  const [requestName, setRequestName] = createSignal("");
  const [headers, setHeaders] = createSignal<KeyValuePair[]>([]);
  const [params, setParams] = createSignal<KeyValuePair[]>([]);
  const [body, setBody] = createSignal<RequestBody>({
    type: "none",
    content: "",
  });
  const [auth, setAuth] = createSignal<RequestAuth>({
    type: "none",
    username: "",
    password: "",
    token: "",
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
    setLoading(true);
    const m = method();
    const u = url();
    log({
      level: "INFO",
      source: "frontend:http",
      message: "HTTP request sent",
      attrs: { method: m, url: u },
    });
    try {
      const res = await api.sendRequest({
        id: activeRequestId() ?? "",
        name: "",
        method: m,
        url: u,
        headers: headers(),
        params: params(),
        body: body(),
        auth: auth(),
      });
      setResponse(res);
      log({
        level: "INFO",
        source: "frontend:http",
        message: "HTTP response received",
        attrs: {
          method: m,
          url: u,
          status: res.statusCode,
          latency_ms: res.timingMs,
        },
      });
    } catch (err) {
      log({
        level: "ERROR",
        source: "frontend:http",
        message: "HTTP request failed",
        attrs: { method: m, url: u, error: String(err) },
      });
      throw err;
    } finally {
      setLoading(false);
    }
    // エラーは呼び出し元 (Presentation 層) に伝播する
  }

  async function cancelRequest(): Promise<void> {
    await api.cancelRequest();
  }

  function loadRequest(req: HttpRequest, collectionId: string): void {
    setMethod(req.method);
    setUrl(req.url);
    setRequestName(req.name);
    setHeaders(req.headers);
    setParams(req.params);
    setBody(req.body);
    setAuth(req.auth);
    setActiveRequestId(req.id);
    setActiveCollectionId(collectionId);
  }

  function newRequest(): void {
    setMethod("GET");
    setUrl("");
    setHeaders([]);
    setParams([]);
    setBody({ type: "none", content: "" });
    setAuth({ type: "none", username: "", password: "", token: "" });
    setActiveRequestId(null);
    setActiveCollectionId(null);
  }

  async function saveCurrentRequest(): Promise<void> {
    const id = activeRequestId();
    const colId = activeCollectionId();
    if (!id || !colId) return;
    await api.updateRequest(colId, {
      id,
      name: requestName(),
      method: method(),
      url: url(),
      headers: headers(),
      params: params(),
      body: body(),
      auth: auth(),
    });
    await api.afterSave?.();
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
