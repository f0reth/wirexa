import { createRoot } from "solid-js";
import { describe, expect, it, vi } from "vitest";
import type { ExpandedFoldersStorage } from "../../domain/http/ports";
import type {
  Collection,
  HttpRequest,
  TreeItem,
} from "../../domain/http/types";
import { DEFAULT_SETTINGS, ROOT_COLLECTION_ID } from "../../domain/http/types";
import type { Notifier } from "../../domain/ui/ports";
import {
  type CollectionsApi,
  createCollectionsState,
  findRequestById,
} from "./collections";

function makeNotifier(): Notifier {
  return {
    error: vi.fn(),
    success: vi.fn(),
    info: vi.fn(),
    warning: vi.fn(),
  };
}

function makeCollection(id: string): Collection {
  return { id, name: id, items: [] };
}

function makeTreeItem(id: string): TreeItem {
  return { id, name: id, type: "folder", children: [] };
}

function makeRequest(): HttpRequest {
  return {
    id: "",
    name: "New Request",
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

/** すべて成功する API。個々のテストで失敗させたいメソッドだけ差し替える。 */
function makeApi(): CollectionsApi {
  return {
    getCollections: vi.fn(async () => []),
    getRootItems: vi.fn(async () => []),
    getSidebarLayout: vi.fn(async () => []),
    createCollection: vi.fn(async (name: string) => makeCollection(name)),
    deleteCollection: vi.fn(async () => {}),
    renameCollection: vi.fn(async () => {}),
    addFolder: vi.fn(async () => makeTreeItem("folder-1")),
    addRequest: vi.fn(async () => makeTreeItem("request-1")),
    renameItem: vi.fn(async () => {}),
    deleteItem: vi.fn(async () => {}),
    moveItem: vi.fn(async () => {}),
    moveSidebarEntry: vi.fn(async () => {}),
    moveItemToSidebar: vi.fn(async () => {}),
  };
}

/** 展開状態をメモリに持つストレージ。save の呼び出しは spy で検査する。 */
function makeExpandedStorage(initial: Record<string, boolean> = {}) {
  let stored = { ...initial };
  const storage = {
    load: vi.fn(() => ({ ...stored })),
    save: vi.fn((ids: Record<string, boolean>) => {
      stored = { ...ids };
    }),
  } satisfies ExpandedFoldersStorage;
  return { storage, stored: () => stored };
}

function withState(
  api: CollectionsApi,
  fn: (
    state: ReturnType<typeof createCollectionsState>,
    notifier: Notifier,
  ) => Promise<void>,
  expandedStorage: ExpandedFoldersStorage = makeExpandedStorage().storage,
): Promise<void> {
  const notifier = makeNotifier();
  return createRoot(async (dispose) => {
    await fn(createCollectionsState(api, notifier, expandedStorage), notifier);
    dispose();
  });
}

/** 指定 id のリクエストを持つツリーアイテムを作る。 */
function makeRequestItem(id: string, url: string): TreeItem {
  return {
    id,
    name: id,
    type: "request",
    children: [],
    request: { ...makeRequest(), id, url },
  };
}

describe("findRequestById", () => {
  const rootItems = [makeRequestItem("r-root", "https://root.example")];
  const collections: Collection[] = [
    {
      id: "col-1",
      name: "col-1",
      items: [
        {
          id: "folder-1",
          name: "folder-1",
          type: "folder",
          children: [
            makeRequestItem("r-nested", "https://nested.example"),
            makeTreeItem("folder-2"),
          ],
        },
        makeRequestItem("r-top", "https://top.example"),
      ],
    },
  ];

  it("finds a request directly under the sidebar root", () => {
    const req = findRequestById(
      collections,
      rootItems,
      ROOT_COLLECTION_ID,
      "r-root",
    );
    expect(req?.url).toBe("https://root.example");
  });

  it("finds a request nested in a folder", () => {
    const req = findRequestById(collections, rootItems, "col-1", "r-nested");
    expect(req?.url).toBe("https://nested.example");
  });

  it("finds a request at the top level of a collection", () => {
    const req = findRequestById(collections, rootItems, "col-1", "r-top");
    expect(req?.url).toBe("https://top.example");
  });

  it("returns null for an unknown collection, item or wrong pairing", () => {
    expect(findRequestById(collections, rootItems, "gone", "r-top")).toBeNull();
    expect(findRequestById(collections, rootItems, "col-1", "gone")).toBeNull();
    // ルート直下の id を通常コレクション側で探しても見つからない。
    expect(
      findRequestById(collections, rootItems, "col-1", "r-root"),
    ).toBeNull();
    // フォルダ（request ではない）は対象外。
    expect(
      findRequestById(collections, rootItems, "col-1", "folder-1"),
    ).toBeNull();
  });
});

describe("createCollectionsState expanded state", () => {
  it("starts from the stored expanded ids", async () => {
    const { storage } = makeExpandedStorage({
      "folder-1": true,
      "col-1": false,
    });

    await withState(
      makeApi(),
      async (state) => {
        expect(state.isExpanded("folder-1", false)).toBe(true);
        expect(state.isExpanded("col-1", true)).toBe(false);
        expect(state.isExpanded("unknown", true)).toBe(true);
      },
      storage,
    );
  });

  it("saves the merged ids when an item is expanded or collapsed", async () => {
    const { storage, stored } = makeExpandedStorage({ "folder-1": true });

    await withState(
      makeApi(),
      async (state) => {
        state.setExpanded("folder-2", true);
        state.setExpanded("folder-1", false);

        expect(state.isExpanded("folder-1", true)).toBe(false);
        expect(stored()).toEqual({ "folder-1": false, "folder-2": true });
      },
      storage,
    );
  });

  it("prunes and saves ids that no longer exist in any collection", async () => {
    const api = makeApi();
    api.getCollections = vi.fn(async () => [
      { id: "col-1", name: "col-1", items: [makeTreeItem("folder-1")] },
    ]);
    const { storage, stored } = makeExpandedStorage({
      "col-1": true,
      "folder-1": true,
      gone: true,
    });

    await withState(
      api,
      async (state) => {
        await state.refreshCollections();

        expect(storage.save).toHaveBeenCalledTimes(1);
        expect(stored()).toEqual({ "col-1": true, "folder-1": true });
        expect(state.isExpanded("gone", false)).toBe(false);
      },
      storage,
    );
  });

  it("does not save when no stored id is stale", async () => {
    const api = makeApi();
    api.getCollections = vi.fn(async () => [makeCollection("col-1")]);
    const { storage } = makeExpandedStorage({ "col-1": true });

    await withState(
      api,
      async (state) => {
        await state.refreshCollections();
        expect(storage.save).not.toHaveBeenCalled();
      },
      storage,
    );
  });
});

describe("createCollectionsState failure contract", () => {
  it("notifies and swallows the error for operations without a return value", async () => {
    const api = makeApi();
    api.deleteItem = vi.fn(async () => {
      throw new Error("gone");
    });

    await withState(api, async (state, notifier) => {
      await expect(
        state.deleteItem("col-1", "item-1"),
      ).resolves.toBeUndefined();
      expect(notifier.error).toHaveBeenCalledWith(
        "Failed to delete item",
        "gone",
      );
    });
  });

  it("notifies and rethrows for operations with a return value", async () => {
    const api = makeApi();
    api.createCollection = vi.fn(async () => {
      throw new Error("disk full");
    });

    await withState(api, async (state, notifier) => {
      await expect(state.createCollection("New Collection")).rejects.toThrow(
        "disk full",
      );
      expect(notifier.error).toHaveBeenCalledWith(
        "Failed to create collection",
        "disk full",
      );
    });
  });

  it("returns the created item and does not notify on success", async () => {
    const api = makeApi();

    await withState(api, async (state, notifier) => {
      const item = await state.addRequest("col-1", "", makeRequest());
      expect(item.id).toBe("request-1");
      expect(notifier.error).not.toHaveBeenCalled();
    });
  });

  it("notifies with the move label when a drag&drop move fails", async () => {
    const api = makeApi();
    api.moveSidebarEntry = vi.fn(async () => {
      throw new Error("locked");
    });

    await withState(api, async (state, notifier) => {
      await state.moveSidebarEntry("collection", "col-1", 0);
      expect(notifier.error).toHaveBeenCalledWith(
        "Failed to move item",
        "locked",
      );
    });
  });
});
