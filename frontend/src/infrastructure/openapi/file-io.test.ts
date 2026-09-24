import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("../../../wailsjs/go/adapters/OpenAPIHandler", () => ({
  GetRecents: vi.fn(),
  MoveRecent: vi.fn(),
  OpenFilePicker: vi.fn(),
  ReadFile: vi.fn(),
  RemoveRecent: vi.fn(),
  SaveFileAs: vi.fn(),
  WriteFile: vi.fn(),
}));

import * as Handler from "../../../wailsjs/go/adapters/OpenAPIHandler";
import { getRecents, moveRecent, removeRecent } from "./file-io";

// ---- helpers ----

function makeWailsRecent(overrides: Record<string, unknown> = {}) {
  return {
    path: "C:\\specs\\petstore.yaml",
    name: "petstore.yaml",
    lastOpenedAt: "2026-09-24T10:00:00Z",
    order: 0,
    ...overrides,
  };
}

// ---- tests ----

beforeEach(() => {
  vi.clearAllMocks();
});

describe("getRecents", () => {
  it("maps each recent to the domain OpenApiFile", async () => {
    vi.mocked(Handler.GetRecents).mockResolvedValue([
      makeWailsRecent(),
      makeWailsRecent({
        path: "C:\\specs\\other.json",
        name: "other.json",
        lastOpenedAt: "2026-09-23T08:30:00Z",
        order: 1,
      }),
    ] as never);

    expect(await getRecents()).toEqual([
      {
        path: "C:\\specs\\petstore.yaml",
        name: "petstore.yaml",
        order: 0,
        lastOpenedAt: "2026-09-24T10:00:00Z",
      },
      {
        path: "C:\\specs\\other.json",
        name: "other.json",
        order: 1,
        lastOpenedAt: "2026-09-23T08:30:00Z",
      },
    ]);
  });

  it("returns [] when there are no recents", async () => {
    vi.mocked(Handler.GetRecents).mockResolvedValue([]);
    expect(await getRecents()).toEqual([]);
  });

  it("propagates rejection from the backend", async () => {
    vi.mocked(Handler.GetRecents).mockRejectedValue(new Error("read failed"));
    await expect(getRecents()).rejects.toThrow("read failed");
  });
});

describe("removeRecent", () => {
  it("passes the path to the backend", async () => {
    vi.mocked(Handler.RemoveRecent).mockResolvedValue();
    await removeRecent("C:\\specs\\petstore.yaml");
    expect(Handler.RemoveRecent).toHaveBeenCalledWith(
      "C:\\specs\\petstore.yaml",
    );
  });
});

describe("moveRecent", () => {
  it("passes the path and index to the backend", async () => {
    vi.mocked(Handler.MoveRecent).mockResolvedValue();
    await moveRecent("C:\\specs\\petstore.yaml", 2);
    expect(Handler.MoveRecent).toHaveBeenCalledWith(
      "C:\\specs\\petstore.yaml",
      2,
    );
  });
});
