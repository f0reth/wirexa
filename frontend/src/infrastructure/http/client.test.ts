import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("../../../wailsjs/go/adapters/HttpHandler", () => ({
  AddFolder: vi.fn(),
  AddRequest: vi.fn(),
  CreateCollection: vi.fn(),
  DeleteCollection: vi.fn(),
  DeleteItem: vi.fn(),
  GetCollections: vi.fn(),
  RenameCollection: vi.fn(),
  RenameItem: vi.fn(),
  SendRequest: vi.fn(),
  UpdateRequest: vi.fn(),
}));

import * as Handler from "../../../wailsjs/go/adapters/HttpHandler";
import {
  addFolder,
  addRequest,
  createCollection,
  deleteCollection,
  deleteItem,
  getCollections,
  renameCollection,
  renameItem,
  sendRequest,
  updateRequest,
} from "./client";

// ---- helpers ----

function makeWailsResponse(overrides: Record<string, unknown> = {}) {
  return {
    statusCode: 200,
    statusText: "OK",
    headers: { "Content-Type": "application/json" },
    body: '{"ok":true}',
    contentType: "application/json",
    size: 11,
    timingMs: 42,
    error: "",
    ...overrides,
  };
}

function makeWailsCollection(overrides: Record<string, unknown> = {}) {
  return {
    id: "col-1",
    name: "My Collection",
    items: [],
    ...overrides,
  };
}

function makeWailsRequest(overrides: Record<string, unknown> = {}) {
  return {
    id: "req-1",
    name: "Test",
    method: "GET",
    url: "https://example.com",
    headers: [],
    params: [],
    body: { type: "none", contents: {} },
    auth: { type: "none", username: "", password: "", token: "" },
    ...overrides,
  };
}

function makeWailsTreeItem(overrides: Record<string, unknown> = {}) {
  return {
    type: "folder",
    id: "item-1",
    name: "Folder",
    children: [],
    request: undefined,
    ...overrides,
  };
}

function makeDomainRequest() {
  return {
    id: "req-1",
    name: "Test",
    method: "GET" as const,
    url: "https://example.com",
    headers: [] as { key: string; value: string; enabled: boolean }[],
    params: [] as { key: string; value: string; enabled: boolean }[],
    body: { type: "none" as const, contents: {} },
    auth: { type: "none" as const, username: "", password: "", token: "" },
    settings: {
      timeoutSec: 0,
      proxyMode: "system" as const,
      proxyURL: "",
      insecureSkipVerify: false,
      disableRedirects: false,
      maxResponseBodyMB: 0,
    },
  };
}

// ---- tests ----

beforeEach(() => {
  vi.clearAllMocks();
});

describe("sendRequest", () => {
  it("maps the Wails response to a domain HttpResponse", async () => {
    vi.mocked(Handler.SendRequest).mockResolvedValue(
      makeWailsResponse() as never,
    );

    const result = await sendRequest(makeDomainRequest());

    expect(result).toEqual({
      statusCode: 200,
      statusText: "OK",
      headers: { "Content-Type": "application/json" },
      body: '{"ok":true}',
      contentType: "application/json",
      size: 11,
      timingMs: 42,
      error: "",
      bodyTruncated: false,
      tempFilePath: "",
    });
  });

  it("maps an error response", async () => {
    vi.mocked(Handler.SendRequest).mockResolvedValue(
      makeWailsResponse({
        statusCode: 0,
        statusText: "",
        body: "",
        contentType: "",
        size: 0,
        timingMs: 0,
        error: "connection refused",
      }) as never,
    );

    const result = await sendRequest(makeDomainRequest());

    expect(result.error).toBe("connection refused");
    expect(result.statusCode).toBe(0);
  });

  it("maps a 4xx response correctly", async () => {
    vi.mocked(Handler.SendRequest).mockResolvedValue(
      makeWailsResponse({
        statusCode: 404,
        statusText: "Not Found",
        body: "not found",
        error: "",
      }) as never,
    );

    const result = await sendRequest(makeDomainRequest());

    expect(result.statusCode).toBe(404);
    expect(result.statusText).toBe("Not Found");
  });

  it("calls SendRequest once", async () => {
    vi.mocked(Handler.SendRequest).mockResolvedValue(
      makeWailsResponse() as never,
    );
    await sendRequest(makeDomainRequest());
    expect(Handler.SendRequest).toHaveBeenCalledOnce();
  });

  it("propagates rejection from the backend", async () => {
    vi.mocked(Handler.SendRequest).mockRejectedValue(
      new Error("network error"),
    );
    await expect(sendRequest(makeDomainRequest())).rejects.toThrow(
      "network error",
    );
  });
});

describe("getCollections", () => {
  it("returns an empty array when there are no collections", async () => {
    vi.mocked(Handler.GetCollections).mockResolvedValue([]);
    const result = await getCollections();
    expect(result).toEqual([]);
  });

  it("maps a collection with no items", async () => {
    vi.mocked(Handler.GetCollections).mockResolvedValue([
      makeWailsCollection() as never,
    ]);
    const result = await getCollections();
    expect(result).toEqual([
      { id: "col-1", name: "My Collection", items: [], order: 0 },
    ]);
  });

  it("maps multiple collections", async () => {
    vi.mocked(Handler.GetCollections).mockResolvedValue([
      makeWailsCollection({ id: "col-1", name: "First" }) as never,
      makeWailsCollection({ id: "col-2", name: "Second" }) as never,
    ]);
    const result = await getCollections();
    expect(result).toHaveLength(2);
    expect(result[0].id).toBe("col-1");
    expect(result[1].id).toBe("col-2");
  });

  it("maps a folder tree item", async () => {
    vi.mocked(Handler.GetCollections).mockResolvedValue([
      makeWailsCollection({
        items: [
          makeWailsTreeItem({ type: "folder", id: "f1", name: "Folder" }),
        ],
      }) as never,
    ]);
    const result = await getCollections();
    expect(result[0].items[0]).toEqual({
      type: "folder",
      id: "f1",
      name: "Folder",
      children: [],
      request: undefined,
    });
  });

  it("maps a request tree item with a nested request", async () => {
    vi.mocked(Handler.GetCollections).mockResolvedValue([
      makeWailsCollection({
        items: [
          makeWailsTreeItem({
            type: "request",
            id: "r1",
            name: "My Request",
            request: makeWailsRequest(),
          }),
        ],
      }) as never,
    ]);
    const result = await getCollections();
    const item = result[0].items[0];
    expect(item.type).toBe("request");
    expect(item.request?.method).toBe("GET");
    expect(item.request?.url).toBe("https://example.com");
  });

  it("maps nested children recursively", async () => {
    vi.mocked(Handler.GetCollections).mockResolvedValue([
      makeWailsCollection({
        items: [
          makeWailsTreeItem({
            type: "folder",
            id: "f1",
            children: [
              makeWailsTreeItem({ type: "request", id: "r1", name: "Req" }),
            ],
          }),
        ],
      }) as never,
    ]);
    const result = await getCollections();
    expect(result[0].items[0].children[0]).toMatchObject({
      id: "r1",
      type: "request",
    });
  });

  it("throws on an unknown tree item type", async () => {
    vi.mocked(Handler.GetCollections).mockResolvedValue([
      makeWailsCollection({
        items: [makeWailsTreeItem({ type: "unknown" })],
      }) as never,
    ]);
    await expect(getCollections()).rejects.toThrow(
      "Unknown tree item type: unknown",
    );
  });

  it("throws on an unknown HTTP method in a stored request", async () => {
    vi.mocked(Handler.GetCollections).mockResolvedValue([
      makeWailsCollection({
        items: [
          makeWailsTreeItem({
            type: "request",
            request: makeWailsRequest({ method: "TRACE" }),
          }),
        ],
      }) as never,
    ]);
    await expect(getCollections()).rejects.toThrow(
      "Unknown HTTP method: TRACE",
    );
  });

  it("throws on an unknown body type in a stored request", async () => {
    vi.mocked(Handler.GetCollections).mockResolvedValue([
      makeWailsCollection({
        items: [
          makeWailsTreeItem({
            type: "request",
            request: makeWailsRequest({ body: { type: "xml", content: "" } }),
          }),
        ],
      }) as never,
    ]);
    await expect(getCollections()).rejects.toThrow("Unknown body type: xml");
  });

  it("falls back to auth type 'none' for an unknown auth type", async () => {
    vi.mocked(Handler.GetCollections).mockResolvedValue([
      makeWailsCollection({
        items: [
          makeWailsTreeItem({
            type: "request",
            request: makeWailsRequest({
              auth: {
                type: "UNKNOWN",
                username: "u",
                password: "p",
                token: "",
              },
            }),
          }),
        ],
      }) as never,
    ]);
    const result = await getCollections();
    expect(result[0].items[0].request?.auth.type).toBe("none");
  });

  it("preserves valid auth fields when auth type is valid", async () => {
    vi.mocked(Handler.GetCollections).mockResolvedValue([
      makeWailsCollection({
        items: [
          makeWailsTreeItem({
            type: "request",
            request: makeWailsRequest({
              auth: {
                type: "basic",
                username: "user",
                password: "pass",
                token: "",
              },
            }),
          }),
        ],
      }) as never,
    ]);
    const result = await getCollections();
    const auth = result[0].items[0].request?.auth;
    expect(auth?.type).toBe("basic");
    expect(auth?.username).toBe("user");
    expect(auth?.password).toBe("pass");
  });
});

describe("createCollection", () => {
  it("calls CreateCollection with the given name", async () => {
    vi.mocked(Handler.CreateCollection).mockResolvedValue(
      makeWailsCollection({ name: "New Col" }) as never,
    );
    await createCollection("New Col");
    expect(Handler.CreateCollection).toHaveBeenCalledWith("New Col");
  });

  it("returns the mapped collection", async () => {
    vi.mocked(Handler.CreateCollection).mockResolvedValue(
      makeWailsCollection({ id: "col-99", name: "New Col" }) as never,
    );
    const result = await createCollection("New Col");
    expect(result).toMatchObject({ id: "col-99", name: "New Col", items: [] });
  });
});

describe("deleteCollection", () => {
  it("calls DeleteCollection with the correct id", async () => {
    vi.mocked(Handler.DeleteCollection).mockResolvedValue(undefined);
    await deleteCollection("col-123");
    expect(Handler.DeleteCollection).toHaveBeenCalledWith("col-123");
  });
});

describe("renameCollection", () => {
  it("calls RenameCollection with the correct id and name", async () => {
    vi.mocked(Handler.RenameCollection).mockResolvedValue(undefined);
    await renameCollection("col-1", "New Name");
    expect(Handler.RenameCollection).toHaveBeenCalledWith("col-1", "New Name");
  });
});

describe("addFolder", () => {
  it("calls AddFolder with the correct params", async () => {
    vi.mocked(Handler.AddFolder).mockResolvedValue(
      makeWailsTreeItem() as never,
    );
    await addFolder("col-1", "parent-1", "New Folder");
    expect(Handler.AddFolder).toHaveBeenCalledWith(
      "col-1",
      "parent-1",
      "New Folder",
    );
  });

  it("returns the mapped tree item", async () => {
    vi.mocked(Handler.AddFolder).mockResolvedValue(
      makeWailsTreeItem({ id: "f2", name: "New Folder" }) as never,
    );
    const result = await addFolder("col-1", "parent-1", "New Folder");
    expect(result.id).toBe("f2");
    expect(result.type).toBe("folder");
  });
});

describe("addRequest", () => {
  it("returns the mapped tree item", async () => {
    vi.mocked(Handler.AddRequest).mockResolvedValue(
      makeWailsTreeItem({ type: "request", id: "r9" }) as never,
    );
    const result = await addRequest("col-1", "parent-1", makeDomainRequest());
    expect(result.type).toBe("request");
    expect(result.id).toBe("r9");
  });

  it("calls AddRequest once", async () => {
    vi.mocked(Handler.AddRequest).mockResolvedValue(
      makeWailsTreeItem({ type: "request" }) as never,
    );
    await addRequest("col-1", "parent-1", makeDomainRequest());
    expect(Handler.AddRequest).toHaveBeenCalledOnce();
  });
});

describe("updateRequest", () => {
  it("calls UpdateRequest with the correct collection id", async () => {
    vi.mocked(Handler.UpdateRequest).mockResolvedValue(undefined);
    await updateRequest("col-1", makeDomainRequest());
    expect(Handler.UpdateRequest).toHaveBeenCalledOnce();
    const [collectionId] = vi.mocked(Handler.UpdateRequest).mock.calls[0];
    expect(collectionId).toBe("col-1");
  });
});

describe("renameItem", () => {
  it("calls RenameItem with the correct params", async () => {
    vi.mocked(Handler.RenameItem).mockResolvedValue(undefined);
    await renameItem("col-1", "item-1", "New Name");
    expect(Handler.RenameItem).toHaveBeenCalledWith(
      "col-1",
      "item-1",
      "New Name",
    );
  });
});

describe("deleteItem", () => {
  it("calls DeleteItem with the correct params", async () => {
    vi.mocked(Handler.DeleteItem).mockResolvedValue(undefined);
    await deleteItem("col-1", "item-1");
    expect(Handler.DeleteItem).toHaveBeenCalledWith("col-1", "item-1");
  });
});
