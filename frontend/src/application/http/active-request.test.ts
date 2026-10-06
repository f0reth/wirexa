import { batch, createRoot, createSignal } from "solid-js";
import { createStore } from "solid-js/store";
import { describe, expect, it, vi } from "vitest";
import type {
  Collection,
  HttpRequest,
  TreeItem,
} from "../../domain/http/types";
import { DEFAULT_SETTINGS, ROOT_COLLECTION_ID } from "../../domain/http/types";
import type { Notifier } from "../../domain/ui/ports";
import { createActiveRequestTracking } from "./active-request";
import { createRequestState, type RequestApi } from "./request";

function makeRequest(id: string): HttpRequest {
  return {
    id,
    name: id,
    method: "GET",
    url: "",
    headers: [],
    params: [],
    body: { type: "none", contents: {} },
    auth: { type: "none", username: "", password: "", token: "" },
    settings: { ...DEFAULT_SETTINGS },
    doc: "",
  };
}

function requestItem(id: string): TreeItem {
  return {
    id,
    name: id,
    type: "request",
    children: [],
    request: makeRequest(id),
  };
}

function folder(id: string, children: TreeItem[]): TreeItem {
  return { id, name: id, type: "folder", children };
}

function collection(id: string, items: TreeItem[]): Collection {
  return { id, name: id, items };
}

/**
 * 手で書き換えられるツリーと、選択中のリクエストを持つだけの request。
 * createRoot の中の変更は 1 つにまとめて反映されるので、操作は外から行う。
 */
function setup(initial: { collections: Collection[]; rootItems?: TreeItem[] }) {
  return createRoot((dispose) => {
    const [collections, setCollections] = createStore<Collection[]>(
      initial.collections,
    );
    const [rootItems, setRootItems] = createStore<TreeItem[]>(
      initial.rootItems ?? [],
    );
    const [collectionsLoaded, setCollectionsLoaded] = createSignal(false);
    const [activeRequestId, setActiveRequestId] = createSignal<string | null>(
      null,
    );
    const [activeCollectionId, setActiveCollectionId] = createSignal<
      string | null
    >(null);
    const request = {
      activeRequestId,
      activeCollectionId,
      closeRequest: vi.fn(() => {
        setActiveRequestId(null);
        setActiveCollectionId(null);
      }),
      relocateActiveRequest: vi.fn((collectionId: string) => {
        setActiveCollectionId(collectionId);
      }),
    };
    createActiveRequestTracking(request, {
      collections,
      rootItems,
      collectionsLoaded,
    });
    return {
      request,
      setCollections,
      setRootItems,
      setCollectionsLoaded,
      select: (id: string, collectionId: string) => {
        batch(() => {
          setActiveRequestId(id);
          setActiveCollectionId(collectionId);
        });
      },
      dispose,
    };
  });
}

describe("createActiveRequestTracking", () => {
  it("does nothing before the collections are loaded", () => {
    // 読み込む前の空のツリーを、削除と解釈しない。
    const s = setup({ collections: [] });
    s.select("r-1", "col-1");

    expect(s.request.closeRequest).not.toHaveBeenCalled();
    expect(s.request.relocateActiveRequest).not.toHaveBeenCalled();
    s.dispose();
  });

  it("relocates the open request when it moved to another collection", () => {
    const s = setup({
      collections: [
        collection("col-1", [requestItem("r-1")]),
        collection("col-2", []),
      ],
    });
    s.setCollectionsLoaded(true);
    s.select("r-1", "col-1");
    expect(s.request.relocateActiveRequest).not.toHaveBeenCalled();

    s.setCollections([
      collection("col-1", []),
      collection("col-2", [folder("f-1", [requestItem("r-1")])]),
    ]);

    expect(s.request.relocateActiveRequest).toHaveBeenCalledTimes(1);
    expect(s.request.relocateActiveRequest).toHaveBeenCalledWith("col-2");
    expect(s.request.closeRequest).not.toHaveBeenCalled();
    s.dispose();
  });

  it("relocates the open request to __root__ when it moved to the sidebar root", () => {
    const s = setup({
      collections: [collection("col-1", [requestItem("r-1")])],
    });
    s.setCollectionsLoaded(true);
    s.select("r-1", "col-1");

    batch(() => {
      s.setCollections([collection("col-1", [])]);
      s.setRootItems([requestItem("r-1")]);
    });

    expect(s.request.relocateActiveRequest).toHaveBeenCalledTimes(1);
    expect(s.request.relocateActiveRequest).toHaveBeenCalledWith(
      ROOT_COLLECTION_ID,
    );
    expect(s.request.closeRequest).not.toHaveBeenCalled();
    s.dispose();
  });

  it("closes the open request when it was deleted together with its parent", () => {
    const s = setup({
      collections: [collection("col-1", [folder("f-1", [requestItem("r-1")])])],
    });
    s.setCollectionsLoaded(true);
    s.select("r-1", "col-1");

    s.setCollections([collection("col-1", [])]);

    expect(s.request.closeRequest).toHaveBeenCalledTimes(1);
    expect(s.request.relocateActiveRequest).not.toHaveBeenCalled();
    s.dispose();
  });

  it("calls nothing while the open request stays where it is", () => {
    const s = setup({
      collections: [collection("col-1", [requestItem("r-1")])],
    });
    s.setCollectionsLoaded(true);
    s.select("r-1", "col-1");

    // 並び替え・ほかのアイテムの追加では所属は変わらない。
    s.setCollections([
      collection("col-1", [requestItem("r-2"), requestItem("r-1")]),
    ]);

    expect(s.request.closeRequest).not.toHaveBeenCalled();
    expect(s.request.relocateActiveRequest).not.toHaveBeenCalled();
    s.dispose();
  });

  // loadRequest がリクエストと所属を別々に切り替えると、その間の「新しいリクエストと前の所属」を
  // 移動と解釈して保存し直してしまう。
  it("does not relocate or save when another request is selected", async () => {
    const api = {
      sendRequest: vi.fn(),
      cancelRequest: vi.fn(),
      updateRequest: vi.fn(async () => {}),
      openFilePicker: vi.fn(),
      saveResponseBody: vi.fn(),
      saveResponseBinary: vi.fn(),
      discardResponseBody: vi.fn(async () => {}),
    } satisfies RequestApi;
    const notifier: Notifier = {
      error: vi.fn(),
      success: vi.fn(),
      info: vi.fn(),
      warning: vi.fn(),
    };
    const { state, dispose } = createRoot((dispose) => {
      const [collections] = createStore<Collection[]>([
        collection("col-1", [requestItem("r-1")]),
        collection("col-2", [requestItem("r-2")]),
      ]);
      const [rootItems] = createStore<TreeItem[]>([]);
      const state = createRequestState(
        api,
        { info: () => {}, error: () => {} },
        notifier,
      );
      createActiveRequestTracking(state, {
        collections,
        rootItems,
        collectionsLoaded: () => true,
      });
      return { state, dispose };
    });

    state.loadRequest(makeRequest("r-1"), "col-1");
    expect(api.updateRequest).not.toHaveBeenCalled();

    state.loadRequest(makeRequest("r-2"), "col-2");
    await Promise.resolve();

    // 切り替え前のリクエストの保存だけが走る。
    expect(api.updateRequest).toHaveBeenCalledTimes(1);
    expect(api.updateRequest).toHaveBeenCalledWith(
      "col-1",
      expect.objectContaining({ id: "r-1" }),
    );
    expect(state.activeCollectionId()).toBe("col-2");
    dispose();
  });
});
