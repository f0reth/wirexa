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
import {
  getRecents,
  moveRecent,
  openFilePicker,
  readFile,
  removeRecent,
  saveFileAs,
  writeFile,
} from "./file-io";

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

  it("propagates rejection from the backend", async () => {
    vi.mocked(Handler.MoveRecent).mockRejectedValue(new Error("locked"));
    await expect(moveRecent("a.yaml", 0)).rejects.toThrow("locked");
  });
});

describe("removeRecent failures", () => {
  it("propagates rejection from the backend", async () => {
    vi.mocked(Handler.RemoveRecent).mockRejectedValue(new Error("locked"));
    await expect(removeRecent("a.yaml")).rejects.toThrow("locked");
  });
});

describe("openFilePicker", () => {
  it("returns the selected path", async () => {
    vi.mocked(Handler.OpenFilePicker).mockResolvedValue("C:\\specs\\a.yaml");
    await expect(openFilePicker()).resolves.toBe("C:\\specs\\a.yaml");
    expect(Handler.OpenFilePicker).toHaveBeenCalledWith();
  });

  it("returns '' when the dialog is canceled", async () => {
    vi.mocked(Handler.OpenFilePicker).mockResolvedValue("");
    await expect(openFilePicker()).resolves.toBe("");
  });
});

describe("readFile", () => {
  it("passes the path and returns the content", async () => {
    vi.mocked(Handler.ReadFile).mockResolvedValue("openapi: 3.0.0");
    await expect(readFile("C:\\specs\\a.yaml")).resolves.toBe("openapi: 3.0.0");
    expect(Handler.ReadFile).toHaveBeenCalledWith("C:\\specs\\a.yaml");
  });

  it("propagates rejection from the backend", async () => {
    vi.mocked(Handler.ReadFile).mockRejectedValue(new Error("not granted"));
    await expect(readFile("a.yaml")).rejects.toThrow("not granted");
  });
});

describe("writeFile", () => {
  it("passes the path and content to WriteFile", async () => {
    vi.mocked(Handler.WriteFile).mockResolvedValue();
    await writeFile("C:\\specs\\a.yaml", "openapi: 3.1.0");
    expect(Handler.WriteFile).toHaveBeenCalledWith(
      "C:\\specs\\a.yaml",
      "openapi: 3.1.0",
    );
  });

  it("propagates rejection from the backend", async () => {
    vi.mocked(Handler.WriteFile).mockRejectedValue(new Error("not granted"));
    await expect(writeFile("a.yaml", "")).rejects.toThrow("not granted");
  });
});

describe("saveFileAs", () => {
  it("passes the default name and content and returns the saved path", async () => {
    vi.mocked(Handler.SaveFileAs).mockResolvedValue("C:\\specs\\new.yaml");
    await expect(saveFileAs("new.yaml", "openapi: 3.1.0")).resolves.toBe(
      "C:\\specs\\new.yaml",
    );
    expect(Handler.SaveFileAs).toHaveBeenCalledWith(
      "new.yaml",
      "openapi: 3.1.0",
    );
  });

  it("returns '' when the save dialog is canceled", async () => {
    vi.mocked(Handler.SaveFileAs).mockResolvedValue("");
    await expect(saveFileAs("new.yaml", "")).resolves.toBe("");
  });
});
