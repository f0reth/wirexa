import { createRoot } from "solid-js";
import { describe, expect, it, vi } from "vitest";
import type { ExpandedFoldersStorage } from "../../domain/http/ports";
import type {
  Collection,
  HttpRequest,
  SidebarEntry,
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

  it("finds a request inside a folder under the sidebar root", () => {
    const items: TreeItem[] = [
      {
        ...makeTreeItem("root-folder"),
        children: [
          {
            ...makeTreeItem("inner-folder"),
            children: [makeRequestItem("r-deep", "https://deep.example")],
          },
        ],
      },
    ];
    expect(findRequestById([], items, ROOT_COLLECTION_ID, "r-deep")?.url).toBe(
      "https://deep.example",
    );
  });

  it("ignores a root folder with the same id", () => {
    const items: TreeItem[] = [makeTreeItem("dup")];
    expect(findRequestById([], items, ROOT_COLLECTION_ID, "dup")).toBeNull();
  });

  it("keeps searching children of a request item without a request body", () => {
    const cols: Collection[] = [
      {
        id: "col-1",
        name: "col-1",
        items: [
          {
            id: "r-1",
            name: "r-1",
            type: "request",
            children: [makeRequestItem("r-1", "https://child.example")],
          },
        ],
      },
    ];
    expect(findRequestById(cols, [], "col-1", "r-1")?.url).toBe(
      "https://child.example",
    );
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

  it("keeps expanded ids of deeply nested folders", async () => {
    const api = makeApi();
    api.getCollections = vi.fn(async () => [
      {
        id: "col-1",
        name: "col-1",
        items: [
          {
            ...makeTreeItem("folder-1"),
            children: [
              {
                ...makeTreeItem("folder-2"),
                children: [makeTreeItem("folder-3")],
              },
            ],
          },
        ],
      },
    ]);
    const { storage } = makeExpandedStorage({
      "folder-2": true,
      "folder-3": true,
    });

    await withState(
      api,
      async (state) => {
        await state.refreshCollections();
        expect(storage.save).not.toHaveBeenCalled();
        expect(state.isExpanded("folder-3", false)).toBe(true);
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

  // API は成功しているが、直後の再読み込みも notifyOnError の中にあるため失敗として扱われる。
  it("reports failure when the refresh after create fails", async () => {
    const api = makeApi();
    api.getCollections = vi.fn(async () => {
      throw new Error("io");
    });

    await withState(api, async (state, notifier) => {
      await expect(state.createCollection("C")).rejects.toThrow("io");
      expect(api.createCollection).toHaveBeenCalledWith("C");
      expect(notifier.error).toHaveBeenCalledWith(
        "Failed to create collection",
        "io",
      );
    });
  });
});

describe("createCollectionsState refresh", () => {
  it("loads collections, root items and layout and marks them loaded", async () => {
    const api = makeApi();
    const cols = [makeCollection("col-1")];
    const roots = [makeRequestItem("r-root", "https://root.example")];
    const layout: SidebarEntry[] = [
      { kind: "collection", id: "col-1" },
      { kind: "item", id: "r-root" },
    ];
    api.getCollections = vi.fn(async () => cols);
    api.getRootItems = vi.fn(async () => roots);
    api.getSidebarLayout = vi.fn(async () => layout);

    await withState(api, async (state) => {
      expect(state.collectionsLoaded()).toBe(false);
      await state.refreshCollections();

      expect(state.collections).toEqual(cols);
      expect(state.rootItems).toEqual(roots);
      expect(state.sidebarLayout).toEqual(layout);
      expect(state.collectionsLoaded()).toBe(true);
    });
  });

  // refreshCollections はガードせず、呼び出し側（起動時の読み込み）に失敗を伝える。
  it("propagates a getCollections rejection from refreshCollections", async () => {
    const api = makeApi();
    api.getCollections = vi.fn(async () => {
      throw new Error("io");
    });

    await withState(api, async (state, notifier) => {
      await expect(state.refreshCollections()).rejects.toThrow("io");
      expect(state.collectionsLoaded()).toBe(false);
      expect(notifier.error).not.toHaveBeenCalled();
    });
  });
});

describe("createCollectionsState mutations", () => {
  type State = ReturnType<typeof createCollectionsState>;

  it.each<
    [string, (s: State) => Promise<unknown>, keyof CollectionsApi, unknown[]]
  >([
    [
      "createCollection",
      (s) => s.createCollection("C"),
      "createCollection",
      ["C"],
    ],
    [
      "deleteCollection",
      (s) => s.deleteCollection("col-1"),
      "deleteCollection",
      ["col-1"],
    ],
    [
      "renameCollection",
      (s) => s.renameCollection("col-1", "N"),
      "renameCollection",
      ["col-1", "N"],
    ],
    [
      "addFolder",
      (s) => s.addFolder("col-1", "p", "F"),
      "addFolder",
      ["col-1", "p", "F"],
    ],
    [
      "addRequest",
      (s) => s.addRequest("col-1", "p", makeRequest()),
      "addRequest",
      ["col-1", "p", makeRequest()],
    ],
    [
      "renameItem",
      (s) => s.renameItem("col-1", "i", "N"),
      "renameItem",
      ["col-1", "i", "N"],
    ],
    [
      "deleteItem",
      (s) => s.deleteItem("col-1", "i"),
      "deleteItem",
      ["col-1", "i"],
    ],
    [
      "moveItem",
      (s) => s.moveItem("col-1", "i", "col-2", "p", 3),
      "moveItem",
      ["col-1", "i", "col-2", "p", 3],
    ],
    [
      "moveItemToSidebar",
      (s) => s.moveItemToSidebar("col-1", "i", 2),
      "moveItemToSidebar",
      ["col-1", "i", 2],
    ],
  ])("%s calls the api and refreshes everything", async (_, run, method, args) => {
    const api = makeApi();

    await withState(api, async (state, notifier) => {
      await run(state);

      expect(api[method]).toHaveBeenCalledWith(...args);
      expect(api.getCollections).toHaveBeenCalledTimes(1);
      expect(api.getRootItems).toHaveBeenCalledTimes(1);
      expect(api.getSidebarLayout).toHaveBeenCalledTimes(1);
      expect(state.collectionsLoaded()).toBe(true);
      expect(notifier.error).not.toHaveBeenCalled();
    });
  });

  it("moveSidebarEntry only refreshes the layout", async () => {
    const api = makeApi();
    const layout: SidebarEntry[] = [{ kind: "item", id: "r-1" }];
    api.getSidebarLayout = vi.fn(async () => layout);

    await withState(api, async (state) => {
      await state.moveSidebarEntry("item", "r-1", 0);

      expect(api.moveSidebarEntry).toHaveBeenCalledWith("item", "r-1", 0);
      expect(api.getSidebarLayout).toHaveBeenCalledTimes(1);
      expect(api.getCollections).not.toHaveBeenCalled();
      expect(api.getRootItems).not.toHaveBeenCalled();
      expect(state.sidebarLayout).toEqual(layout);
    });
  });
});

describe("createCollectionsState patchRequest", () => {
  const edited = (id: string): HttpRequest => ({
    ...makeRequest(),
    id,
    name: "edited name",
    method: "POST",
    url: "https://edited.example",
  });

  it("patches a nested request but keeps its backend name", async () => {
    const api = makeApi();
    api.getCollections = vi.fn(async () => [
      {
        id: "col-1",
        name: "col-1",
        items: [
          {
            ...makeTreeItem("folder-1"),
            children: [makeRequestItem("r-nested", "https://old.example")],
          },
          makeRequestItem("r-other", "https://other.example"),
        ],
      },
    ]);

    await withState(api, async (state) => {
      await state.refreshCollections();
      state.patchRequest("col-1", edited("r-nested"));

      const find = (id: string) =>
        findRequestById(state.collections, state.rootItems, "col-1", id);
      expect(find("r-nested")).toEqual({
        ...edited("r-nested"),
        name: "New Request",
      });
      expect(find("r-other")?.url).toBe("https://other.example");
    });
  });

  it("patches a request directly under the sidebar root", async () => {
    const api = makeApi();
    api.getRootItems = vi.fn(async () => [
      makeRequestItem("r-root", "https://old.example"),
    ]);

    await withState(api, async (state) => {
      await state.refreshCollections();
      state.patchRequest(ROOT_COLLECTION_ID, edited("r-root"));

      expect(state.rootItems[0].request).toEqual({
        ...edited("r-root"),
        name: "New Request",
      });
    });
  });

  it("patches a request inside a folder under the sidebar root", async () => {
    const api = makeApi();
    api.getRootItems = vi.fn(async () => [
      {
        ...makeTreeItem("root-folder"),
        children: [makeRequestItem("r-deep", "https://old.example")],
      },
    ]);

    await withState(api, async (state) => {
      await state.refreshCollections();
      state.patchRequest(ROOT_COLLECTION_ID, edited("r-deep"));

      expect(state.rootItems[0].children[0].request).toEqual({
        ...edited("r-deep"),
        name: "New Request",
      });
    });
  });

  it("does not patch a request of another collection with the same id", async () => {
    const api = makeApi();
    api.getCollections = vi.fn(async () => [
      {
        id: "col-1",
        name: "col-1",
        items: [makeRequestItem("r-1", "https://one.example")],
      },
      {
        id: "col-2",
        name: "col-2",
        items: [makeRequestItem("r-1", "https://two.example")],
      },
    ]);

    await withState(api, async (state) => {
      await state.refreshCollections();
      state.patchRequest("col-2", edited("r-1"));

      const find = (colId: string) =>
        findRequestById(state.collections, state.rootItems, colId, "r-1");
      expect(find("col-1")?.url).toBe("https://one.example");
      expect(find("col-2")?.url).toBe("https://edited.example");
    });
  });
});
