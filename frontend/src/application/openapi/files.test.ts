import { createRoot } from "solid-js";
import { describe, expect, it, vi } from "vitest";
import type { OpenApiFile } from "../../domain/openapi/types";
import type { Notifier } from "../../domain/ui/ports";
import { createFilesState, type OpenApiRecentsApi } from "./files";

function makeNotifier(): Notifier {
  return {
    error: vi.fn(),
    success: vi.fn(),
    info: vi.fn(),
    warning: vi.fn(),
  };
}

function makeFile(path: string, order: number): OpenApiFile {
  return {
    path,
    name: path.split("/").pop() ?? path,
    order,
    lastOpenedAt: "2026-09-24T10:00:00Z",
  };
}

/** Go 側と同じく、削除すると一覧から消える偽 API。 */
function makeApi(initial: OpenApiFile[] = []): OpenApiRecentsApi {
  let recents = [...initial];
  return {
    getRecents: vi.fn(async () => [...recents]),
    removeRecent: vi.fn(async (path: string) => {
      recents = recents.filter((r) => r.path !== path);
    }),
    moveRecent: vi.fn(async () => {}),
  };
}

function withState(
  api: OpenApiRecentsApi,
  fn: (
    state: ReturnType<typeof createFilesState>,
    notifier: Notifier,
  ) => Promise<void>,
): Promise<void> {
  const notifier = makeNotifier();
  return createRoot(async (dispose) => {
    await fn(createFilesState(api, notifier), notifier);
    dispose();
  });
}

describe("createFilesState refreshRecents", () => {
  it("reflects the recents returned by the api", async () => {
    const recents = [
      makeFile("/specs/a.yaml", 0),
      makeFile("/specs/b.json", 1),
    ];

    await withState(makeApi(recents), async (state) => {
      expect(state.files()).toEqual([]);
      await state.refreshRecents();
      expect(state.files()).toEqual(recents);
    });
  });
});

describe("createFilesState removeFile", () => {
  it("clears the active document when its file is removed", async () => {
    const api = makeApi([makeFile("/specs/a.yaml", 0)]);

    await withState(api, async (state) => {
      state.setActiveDoc({ kind: "file", path: "/specs/a.yaml", name: "a" });
      await state.removeFile("/specs/a.yaml");

      expect(api.removeRecent).toHaveBeenCalledWith("/specs/a.yaml");
      expect(state.files()).toEqual([]);
      expect(state.activeDoc()).toBeNull();
    });
  });

  it("keeps the active document when another file is removed", async () => {
    const api = makeApi([
      makeFile("/specs/a.yaml", 0),
      makeFile("/specs/b.json", 1),
    ]);

    await withState(api, async (state) => {
      const active = {
        kind: "file",
        path: "/specs/a.yaml",
        name: "a",
      } as const;
      state.setActiveDoc(active);
      await state.removeFile("/specs/b.json");

      expect(state.files().map((f) => f.path)).toEqual(["/specs/a.yaml"]);
      expect(state.activeDoc()).toEqual(active);
    });
  });
});

describe("createFilesState moveFile", () => {
  it("moves the file and refreshes the recents", async () => {
    const api = makeApi([makeFile("/specs/a.yaml", 0)]);

    await withState(api, async (state, notifier) => {
      await state.moveFile("/specs/a.yaml", 1);

      expect(api.moveRecent).toHaveBeenCalledWith("/specs/a.yaml", 1);
      expect(api.getRecents).toHaveBeenCalledOnce();
      expect(notifier.error).not.toHaveBeenCalled();
    });
  });

  it("notifies and swallows the error when reordering fails", async () => {
    const api = makeApi();
    api.moveRecent = vi.fn(async () => {
      throw new Error("locked");
    });

    await withState(api, async (state, notifier) => {
      await expect(state.moveFile("/specs/a.yaml", 1)).resolves.toBeUndefined();
      expect(notifier.error).toHaveBeenCalledWith(
        "Failed to reorder file",
        "locked",
      );
    });
  });
});
