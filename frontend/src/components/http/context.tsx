import {
  createContext,
  createSignal,
  type JSX,
  onCleanup,
  onMount,
  useContext,
} from "solid-js";
import {
  GetCollections,
  SendRequest,
  UpdateRequest,
} from "../../../wailsjs/go/http/HttpService";
import type {
  Collection,
  HttpMethod,
  HttpRequest,
  HttpResponse,
  KeyValuePair,
  RequestBody,
  TreeItem,
} from "./types";

interface HttpContextValue {
  // Request state
  method: () => HttpMethod;
  setMethod: (m: HttpMethod) => void;
  url: () => string;
  setUrl: (u: string) => void;
  headers: () => KeyValuePair[];
  setHeaders: (h: KeyValuePair[]) => void;
  params: () => KeyValuePair[];
  setParams: (p: KeyValuePair[]) => void;
  body: () => RequestBody;
  setBody: (b: RequestBody) => void;
  requestTab: () => string;
  setRequestTab: (t: string) => void;
  responseTab: () => string;
  setResponseTab: (t: string) => void;

  // Response state
  response: () => HttpResponse | null;
  loading: () => boolean;

  // Active request tracking
  activeRequestId: () => string | null;
  activeCollectionId: () => string | null;
  dirty: () => boolean;

  // Actions
  sendRequest: () => Promise<void>;
  loadRequest: (collectionId: string, item: TreeItem) => void;
  newRequest: () => void;
  saveCurrentRequest: () => Promise<void>;

  // Collections
  collections: () => Collection[];
  setCollections: (c: Collection[]) => void;
  refreshCollections: () => Promise<void>;
}

const HttpContext = createContext<HttpContextValue>();

export function useHttp() {
  const ctx = useContext(HttpContext);
  if (!ctx) throw new Error("useHttp must be used within HttpProvider");
  return ctx;
}

export function HttpProvider(props: { children: JSX.Element }) {
  const [method, setMethodRaw] = createSignal<HttpMethod>("GET");
  const [url, setUrlRaw] = createSignal("");
  const [headers, setHeadersRaw] = createSignal<KeyValuePair[]>([]);
  const [params, setParamsRaw] = createSignal<KeyValuePair[]>([]);
  const [body, setBodyRaw] = createSignal<RequestBody>({
    type: "none",
    content: "",
  });
  const [requestTab, setRequestTab] = createSignal("params");
  const [responseTab, setResponseTab] = createSignal("body");
  const [response, setResponse] = createSignal<HttpResponse | null>(null);
  const [loading, setLoading] = createSignal(false);
  const [activeRequestId, setActiveRequestId] = createSignal<string | null>(
    null,
  );
  const [activeRequestName, setActiveRequestName] = createSignal("");
  const [activeCollectionId, setActiveCollectionId] = createSignal<
    string | null
  >(null);
  const [collections, setCollections] = createSignal<Collection[]>([]);
  const [dirty, setDirty] = createSignal(false);

  // Dirty-tracking wrappers
  const setMethod = (m: HttpMethod) => {
    setMethodRaw(m);
    if (activeRequestId()) setDirty(true);
  };
  const setUrl = (u: string) => {
    setUrlRaw(u);
    if (activeRequestId()) setDirty(true);
  };
  const setHeaders = (h: KeyValuePair[]) => {
    setHeadersRaw(h);
    if (activeRequestId()) setDirty(true);
  };
  const setParams = (p: KeyValuePair[]) => {
    setParamsRaw(p);
    if (activeRequestId()) setDirty(true);
  };
  const setBody = (b: RequestBody) => {
    setBodyRaw(b);
    if (activeRequestId()) setDirty(true);
  };

  const saveCurrentRequest = async () => {
    const reqId = activeRequestId();
    const colId = activeCollectionId();
    if (!reqId || !colId) return;
    try {
      const req: HttpRequest = {
        id: reqId,
        name: activeRequestName(),
        method: method(),
        url: url(),
        headers: headers(),
        params: params(),
        body: body(),
      };
      // biome-ignore lint/suspicious/noExplicitAny: Wails generated types differ from local interfaces
      await UpdateRequest(colId, req as any);
      setDirty(false);
      await refreshCollections();
    } catch {
      // ignore
    }
  };

  const sendRequest = async () => {
    setLoading(true);
    setResponse(null);
    try {
      const req: HttpRequest = {
        id: activeRequestId() || "",
        name: "",
        method: method(),
        url: url(),
        headers: headers(),
        params: params(),
        body: body(),
      };
      // biome-ignore lint/suspicious/noExplicitAny: Wails generated types differ from local interfaces
      const resp = await SendRequest(req as any);
      setResponse(resp as unknown as HttpResponse);
    } catch (e) {
      setResponse({
        statusCode: 0,
        statusText: "",
        headers: {},
        body: "",
        contentType: "",
        size: 0,
        timingMs: 0,
        error: String(e),
      });
    } finally {
      setLoading(false);
    }
  };

  const loadRequest = (collectionId: string, item: TreeItem) => {
    if (item.type !== "request" || !item.request) return;
    const req = item.request;
    setActiveRequestId(req.id);
    setActiveRequestName(item.name);
    setActiveCollectionId(collectionId);
    setMethodRaw(req.method as HttpMethod);
    setUrlRaw(req.url);
    setHeadersRaw(req.headers || []);
    setParamsRaw(req.params || []);
    setBodyRaw(req.body || { type: "none", content: "" });
    setResponse(null);
    setDirty(false);
  };

  const newRequest = () => {
    setActiveRequestId(null);
    setActiveRequestName("");
    setActiveCollectionId(null);
    setMethodRaw("GET");
    setUrlRaw("");
    setHeadersRaw([]);
    setParamsRaw([]);
    setBodyRaw({ type: "none", content: "" });
    setResponse(null);
    setDirty(false);
  };

  const refreshCollections = async () => {
    try {
      const cols = await GetCollections();
      setCollections((cols || []) as unknown as Collection[]);
    } catch {
      // ignore
    }
  };

  // Ctrl+S handler
  const handleKeyDown = (e: KeyboardEvent) => {
    if ((e.ctrlKey || e.metaKey) && e.key === "s") {
      e.preventDefault();
      if (dirty()) {
        saveCurrentRequest();
      }
    }
  };

  onMount(() => {
    document.addEventListener("keydown", handleKeyDown);
  });

  onCleanup(() => {
    document.removeEventListener("keydown", handleKeyDown);
  });

  const value: HttpContextValue = {
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
    requestTab,
    setRequestTab,
    responseTab,
    setResponseTab,
    response,
    loading,
    activeRequestId,
    activeCollectionId,
    dirty,
    sendRequest,
    loadRequest,
    newRequest,
    saveCurrentRequest,
    collections,
    setCollections,
    refreshCollections,
  };

  return (
    <HttpContext.Provider value={value}>{props.children}</HttpContext.Provider>
  );
}
