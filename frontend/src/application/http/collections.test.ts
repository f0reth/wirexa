// @vitest-environment jsdom
// createCollectionsState は展開状態を localStorage から読むため DOM 環境が要る。

import { createRoot } from "solid-js";
import { describe, expect, it, vi } from "vitest";
import type {
  Collection,
  HttpRequest,
  TreeItem,
} from "../../domain/http/types";
import { DEFAULT_SETTINGS } from "../../domain/http/types";
import type { Notifier } from "../../domain/ui/ports";
import { type CollectionsApi, createCollectionsState } from "./collections";

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

function withState(
  api: CollectionsApi,
  fn: (
    state: ReturnType<typeof createCollectionsState>,
    notifier: Notifier,
  ) => Promise<void>,
): Promise<void> {
  const notifier = makeNotifier();
  return createRoot(async (dispose) => {
    await fn(createCollectionsState(api, notifier), notifier);
    dispose();
  });
}

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
