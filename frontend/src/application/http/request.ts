import { createEffect, createSignal, onCleanup } from "solid-js";
import type { Logger } from "../../application/logger";
import type {
  ContentBodyType,
  FileReference,
  FormBodyType,
  FormRow,
  HttpMethod,
  HttpRequest,
  HttpResponse,
  KeyValuePair,
  RequestAuth,
  RequestBody,
  RequestSettings,
} from "../../domain/http/types";
import {
  DEFAULT_SETTINGS,
  FORM_PAIR_FIELDS,
  hasUnconfirmedFile,
  isFormBodyType,
  isResponseUnavailableError,
} from "../../domain/http/types";
import type { Notifier } from "../../domain/ui/ports";
import { generateId } from "../../infrastructure/id/generator";
import { errorMessage } from "../../shared/error";

const JSON_BODY_DEFAULT = '{\n  "": ""\n}';

// 行が無いときに毎回新しい配列を作らないよう、空配列の同一性を固定する。
const EMPTY_PAIRS: FormRow[] = [];

export interface RequestApi {
  // executionId は送信ごとの実行 ID で、req.id (保存済みリクエストの ID) とは独立している。
  sendRequest(executionId: string, req: HttpRequest): Promise<HttpResponse>;
  cancelRequest(executionId: string): Promise<void>;
  updateRequest(collectionId: string, req: HttpRequest): Promise<void>;
  /**
   * ネイティブのファイル選択ダイアログ。hint はダイアログの初期位置にだけ使われ、
   * 確定した token はダイアログの選択結果からだけ発行される。キャンセル時は undefined。
   */
  openFilePicker(hint: string): Promise<FileReference | undefined>;
  // 切り詰められたレスポンスの全文を execution ID で保存する。保存したら true。
  saveResponseBody(executionId: string): Promise<boolean>;
  // 切り詰められていないバイナリボディを、メモリ上の base64 から保存する。
  saveResponseBinary(base64Content: string, contentType: string): Promise<void>;
  discardResponseBody(executionId: string): Promise<void>;
  afterSave?: (collectionId: string, req: HttpRequest) => void;
}

// 表示中のレスポンスと、それを返した送信の execution ID。
// 全文の保存・破棄は execution ID で行うため、応答順が入れ替わっても組がずれないよう一体で持つ。
interface CurrentResponse {
  executionId: string;
  response: HttpResponse;
}

// 切り詰められたレスポンスの全文の保存状態。
// saved は保存済み（backend の一時ファイルは削除済み）、unavailable は backend が回収済み。
export type ResponseSaveState = "idle" | "saved" | "unavailable";

// 未確定・再選択待ちのファイルがあるときに送信を止める理由。
const UNCONFIRMED_FILE_ERROR =
  "Select the file with Browse... before sending. A typed path only sets where the file dialog opens.";

function errorResponse(error: string): HttpResponse {
  return {
    statusCode: 0,
    statusText: "",
    headers: {},
    body: "",
    contentType: "",
    size: 0,
    timingMs: 0,
    error,
    bodyTruncated: false,
    bodyBase64: false,
    bodyCapped: false,
  };
}

export function createRequestState(
  api: RequestApi,
  logger: Logger,
  notifier: Notifier,
) {
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
  const [current, setCurrent] = createSignal<CurrentResponse | null>(null);
  const response = () => current()?.response ?? null;
  const [responseSaveState, setResponseSaveState] =
    createSignal<ResponseSaveState>("idle");
  // 実行中リクエストの execution ID。保存済みリクエストの ID とは別に送信ごとに採番するため、
  // 同じリクエストを並行送信してもバックエンドの cancels マップでキーが衝突しない。
  const [inFlight, setInFlight] = createSignal<readonly string[]>([]);
  const loading = () => inFlight().length > 0;
  const [activeRequestId, setActiveRequestId] = createSignal<string | null>(
    null,
  );
  const [activeCollectionId, setActiveCollectionId] = createSignal<
    string | null
  >(null);
  const [saveError, setSaveError] = createSignal<string | null>(null);

  // file 種別は本文を contents に持たない（送信元は file 参照の token だけ）。
  const contentKey = (): ContentBodyType | null => {
    const type = body().type;
    return type === "file" ? null : type;
  };

  // 選択中の body type に対応する本文。json だけは未入力時に雛形を返す。
  const bodyContent = (): string => {
    const key = contentKey();
    const content = key ? body().contents[key] : undefined;
    if (content === undefined && key === "json") {
      return JSON_BODY_DEFAULT;
    }
    return content ?? "";
  };

  const setBodyContent = (content: string): void => {
    const key = contentKey();
    if (!key) return;
    setBody({
      ...body(),
      contents: { ...body().contents, [key]: content },
    });
  };

  // form 系の行は body の専用フィールドそのものを読み書きする。
  // 文字列へ畳んで導出し直すと空キー行が直列化で落ちて Add が効かなくなるため、
  // Params/Headers と同じく実体の state を編集対象にする。
  const formBodyType = (): FormBodyType | null => {
    const type = body().type;
    return isFormBodyType(type) ? type : null;
  };

  const formField = () => {
    const type = formBodyType();
    return type ? FORM_PAIR_FIELDS[type] : null;
  };

  const formPairs = (): FormRow[] => {
    const field = formField();
    return field ? (body()[field] ?? EMPTY_PAIRS) : EMPTY_PAIRS;
  };

  const setFormPairs = (rows: FormRow[]): void => {
    const field = formField();
    if (!field) return;
    setBody({ ...body(), [field]: rows });
  };

  // 表示中のレスポンスを置き換える。backend が全文の一時ファイルを追跡している
  // （切り詰められた）レスポンスは破棄を通知する。通知が届かなくても backend の
  // 件数・容量上限と TTL で回収されるため、失敗はログに留める。
  function replaceResponse(next: CurrentResponse | null): void {
    const prev = current();
    if (
      prev &&
      prev !== next &&
      prev.response.bodyTruncated &&
      responseSaveState() === "idle"
    ) {
      api.discardResponseBody(prev.executionId).catch((err) =>
        logger.error("Failed to discard response body", {
          error: errorMessage(err),
        }),
      );
    }
    setCurrent(next);
    setResponseSaveState("idle");
  }

  async function sendRequest(): Promise<void> {
    const m = method();
    const u = url();
    // 送信ごとの実行 ID。保存済みリクエストの ID とは別に採番する。
    const executionId = generateId();
    replaceResponse(null);
    // 入力しただけのパスや保存済みの参照は許可にならないため、backend に送る前に止める。
    if (hasUnconfirmedFile(body())) {
      replaceResponse({
        executionId,
        response: errorResponse(UNCONFIRMED_FILE_ERROR),
      });
      return;
    }
    setInFlight((ids) => [...ids, executionId]);
    logger.info("HTTP request sent", { method: m, url: u });
    try {
      const res = await api.sendRequest(executionId, {
        // 保存済みリクエストの ID。未保存 (新規タブ) なら空。
        id: activeRequestId() ?? "",
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
      replaceResponse({ executionId, response: res });
      logger.info("HTTP response received", {
        method: m,
        url: u,
        status: res.statusCode,
        latency_ms: res.timingMs,
      });
    } catch (err) {
      const errorMsg = err instanceof Error ? err.message : String(err);
      replaceResponse({
        executionId,
        response: errorResponse(errorMsg),
      });
      logger.error("HTTP request failed", {
        method: m,
        url: u,
        error: errorMsg,
      });
    } finally {
      setInFlight((ids) => ids.filter((id) => id !== executionId));
    }
  }

  // ファイル選択ダイアログ。hint は初期位置にしか使われず、確定した参照だけを返す。
  async function pickFile(hint: string): Promise<FileReference | undefined> {
    try {
      return await api.openFilePicker(hint);
    } catch (err) {
      notifier.error("Failed to open file picker", errorMessage(err));
      return undefined;
    }
  }

  async function cancelRequest(): Promise<void> {
    const ids = inFlight();
    if (ids.length === 0) return;
    await Promise.all(ids.map((id) => api.cancelRequest(id)));
  }

  // 表示中のレスポンスボディをファイルへ保存する。切り詰め時は backend が追跡する
  // 全文を execution ID で、非切り詰めのバイナリはメモリ上の base64 から保存する。
  async function saveResponseBody(): Promise<void> {
    const cur = current();
    if (!cur) return;
    const { executionId, response: resp } = cur;
    try {
      if (!resp.bodyTruncated) {
        await api.saveResponseBinary(resp.body, resp.contentType);
        return;
      }
      const saved = await api.saveResponseBody(executionId);
      if (saved && current() === cur) setResponseSaveState("saved");
    } catch (err) {
      const msg = errorMessage(err);
      if (resp.bodyTruncated && isResponseUnavailableError(msg)) {
        if (current() === cur) setResponseSaveState("unavailable");
        return;
      }
      notifier.error("Failed to save response", msg);
    }
  }

  function loadRequest(req: HttpRequest, collectionId: string): void {
    saveCurrentRequest().catch((err) =>
      notifier.error("Failed to save request", errorMessage(err)),
    );
    // 別のリクエストへ切り替えたら、前のリクエストのレスポンスは表示せず破棄する。
    if (req.id !== activeRequestId() || collectionId !== activeCollectionId()) {
      replaceResponse(null);
    }
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
      notifier.error("Failed to save request", errorMessage(err)),
    );
    replaceResponse(null);
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
    bodyContent,
    setBodyContent,
    formBodyType,
    formPairs,
    setFormPairs,
    auth,
    setAuth,
    settings,
    setSettings,
    doc,
    setDoc,
    response,
    responseSaveState,
    saveResponseBody,
    loading,
    activeRequestId,
    activeCollectionId,
    saveError,
    clearSaveError: () => setSaveError(null),
    sendRequest,
    cancelRequest,
    pickFile,
    loadRequest,
    newRequest,
    saveCurrentRequest,
  };
}

export type RequestState = ReturnType<typeof createRequestState>;

export function createAutoSaveEffect(
  state: RequestState,
  notifier: Notifier,
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
        notifier.error("Failed to auto-save request", errorMessage(err), {
          key: "http-autosave",
        });
      });
    }, debounceMs);

    onCleanup(() => clearTimeout(timer));
  });
}
