import { ROOT_COLLECTION_ID } from "../../../src/domain/http/types";
import { expect, test } from "../../fixtures/ui";

// 偽バックエンドが Go の CollectionService・HTTPRequestService と同じ入力を、同じ文言で拒否する
// ことの回帰テスト。どれも画面からは送れない値なので、バインディングを直接呼ぶ (fake.callHttp)。
// 文言は internal/domain/errors.go の "<resource> not found: <id>"・"invalid <field>: <message>"。
const COLLECTION = { id: "col-strict", name: "Strict Collection" };
const FOLDER = { id: "folder-strict", folder: "Strict Folder" };
const REQUEST = { id: "req-strict", name: "Strict Request" };

const RESERVED = "invalid id: reserved collection cannot be modified";

test.use({
  seed: { collections: [{ ...COLLECTION, items: [FOLDER, REQUEST] }] },
});

test("fake backend rejects deleting and renaming the root collection", async ({
  fake,
}) => {
  expect(
    await fake.callHttp("AddRequest", ROOT_COLLECTION_ID, "", {
      id: "",
      name: "RootReq",
    }),
  ).toBeNull();
  const before = await fake.snapshot();
  expect(before.rootItems.map((i) => i.name)).toEqual(["RootReq"]);

  expect(await fake.callHttp("DeleteCollection", ROOT_COLLECTION_ID)).toBe(
    RESERVED,
  );
  expect(await fake.callHttp("RenameCollection", ROOT_COLLECTION_ID, "x")).toBe(
    RESERVED,
  );

  const after = await fake.snapshot();
  expect(after.rootItems).toEqual(before.rootItems);
  expect(after.sidebar).toEqual(before.sidebar);
});

test("fake backend rejects targets that do not exist, like the Go backend", async ({
  fake,
}) => {
  const before = await fake.snapshot();
  const request = { id: "missing", name: "" };

  expect(await fake.callHttp("UpdateRequest", COLLECTION.id, request)).toBe(
    "request not found: missing",
  );
  // フォルダはリクエストとして更新できない。
  expect(
    await fake.callHttp("UpdateRequest", COLLECTION.id, {
      ...request,
      id: FOLDER.id,
    }),
  ).toBe(`request not found: ${FOLDER.id}`);
  expect(await fake.callHttp("UpdateRequest", "nowhere", request)).toBe(
    "collection not found: nowhere",
  );
  expect(await fake.callHttp("RenameItem", COLLECTION.id, "missing", "x")).toBe(
    "item not found: missing",
  );
  expect(await fake.callHttp("DeleteItem", COLLECTION.id, "missing")).toBe(
    "item not found: missing",
  );
  expect(
    await fake.callHttp(
      "MoveItem",
      COLLECTION.id,
      "missing",
      COLLECTION.id,
      "",
      0,
    ),
  ).toBe("item not found: missing");
  expect(
    await fake.callHttp("MoveItemToSidebar", COLLECTION.id, "missing", 0),
  ).toBe("item not found: missing");
  expect(await fake.callHttp("DeleteCollection", "missing")).toBe(
    "collection not found: missing",
  );
  expect(await fake.callHttp("RenameCollection", "missing", "x")).toBe(
    "collection not found: missing",
  );
  expect(await fake.callHttp("MoveSidebarEntry", "item", "missing", 0)).toBe(
    "sidebar entry not found: missing",
  );

  expect(await fake.snapshot()).toEqual(before);
});

test("fake backend rejects a parent that is missing or not a folder", async ({
  fake,
}) => {
  const before = await fake.snapshot();

  expect(await fake.callHttp("AddFolder", COLLECTION.id, "missing", "x")).toBe(
    "parent not found: missing",
  );
  // リクエストは親になれない。
  expect(
    await fake.callHttp("AddRequest", COLLECTION.id, REQUEST.id, {
      id: "",
      name: "x",
    }),
  ).toBe(`parent not found: ${REQUEST.id}`);
  expect(
    await fake.callHttp(
      "MoveItem",
      COLLECTION.id,
      FOLDER.id,
      COLLECTION.id,
      REQUEST.id,
      0,
    ),
  ).toBe(`parent not found: ${REQUEST.id}`);

  expect(await fake.snapshot()).toEqual(before);
});

test("fake backend validates the collection name and the method", async ({
  fake,
}) => {
  expect(await fake.callHttp("CreateCollection", "   ")).toBe(
    "invalid name: is required",
  );
  expect((await fake.snapshot()).collections.map((c) => c.id)).toEqual([
    COLLECTION.id,
  ]);

  expect(
    await fake.callHttp("SendRequest", "exec-1", {
      method: "TRACE",
      url: "https://example.com",
      body: { type: "none", contents: {} },
    }),
  ).toBe("invalid method: TRACE");
});

test("fake backend returns collections sorted by name", async ({
  page,
  fake,
}) => {
  // 作った順は Strict → Zeta → Alpha。
  expect(await fake.callHttp("CreateCollection", "Zeta")).toBeNull();
  expect(await fake.callHttp("CreateCollection", "Alpha")).toBeNull();

  const names = await page.evaluate(async () => {
    const { HTTPHandler } = (
      window as unknown as {
        go: {
          adapters: {
            HTTPHandler: { GetCollections(): Promise<{ name: string }[]> };
          };
        };
      }
    ).go.adapters;
    return (await HTTPHandler.GetCollections()).map((c) => c.name);
  });
  expect(names).toEqual(["Alpha", COLLECTION.name, "Zeta"]);
});
