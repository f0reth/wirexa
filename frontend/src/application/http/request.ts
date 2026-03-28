import { createSignal } from "solid-js";
import type {
  HttpMethod,
  HttpRequest,
  HttpResponse,
  KeyValuePair,
  RequestBody,
} from "../../domain/http/types";

export interface RequestApi {
  sendRequest(req: HttpRequest): Promise<HttpResponse>;
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
    try {
      const res = await api.sendRequest({
        id: activeRequestId() ?? "",
        name: "",
        method: method(),
        url: url(),
        headers: headers(),
        params: params(),
        body: body(),
      });
      setResponse(res);
    } finally {
      setLoading(false);
    }
    // エラーは呼び出し元 (Presentation 層) に伝播する
  }

  function loadRequest(req: HttpRequest, collectionId: string): void {
    setMethod(req.method);
    setUrl(req.url);
    setRequestName(req.name);
    setHeaders(req.headers);
    setParams(req.params);
    setBody(req.body);
    setActiveRequestId(req.id);
    setActiveCollectionId(collectionId);
  }

  function newRequest(): void {
    setMethod("GET");
    setUrl("");
    setHeaders([]);
    setParams([]);
    setBody({ type: "none", content: "" });
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
    response,
    loading,
    activeRequestId,
    activeCollectionId,
    sendRequest,
    loadRequest,
    newRequest,
    saveCurrentRequest,
  };
}
