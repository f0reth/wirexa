import type { Page } from "@playwright/test";
import { ROOT_COLLECTION_ID } from "../../../src/domain/http/types";
import { expect, test } from "../../fixtures/ui";

// UI には root を削除・リネームする操作が無いので、偽バックエンドのバインディングを直接呼ぶ。
// Go 側 CollectionService と同じく予約済みの __root__ を拒否することの回帰テスト。

type HTTPHandler = Record<string, (...args: unknown[]) => Promise<unknown>>;

/** HTTPHandler のバインディングを呼び、reject されたらそのメッセージを、成功したら null を返す。 */
function callHandler(
  page: Page,
  method: string,
  ...callArgs: unknown[]
): Promise<string | null> {
  return page.evaluate(
    async ([m, a]) => {
      const handler = (
        window as unknown as { go: { adapters: { HTTPHandler: HTTPHandler } } }
      ).go.adapters.HTTPHandler;
      try {
        await handler[m](...a);
        return null;
      } catch (e) {
        return e instanceof Error ? e.message : String(e);
      }
    },
    [method, callArgs] as const,
  );
}

test("fake backend rejects deleting and renaming the root collection", async ({
  page,
  fake,
}) => {
  const added = await callHandler(page, "AddRequest", ROOT_COLLECTION_ID, "", {
    id: "root-req",
    name: "RootReq",
  });
  expect(added).toBeNull();
  const before = await fake.snapshot();
  expect(before.rootItems.map((i) => i.id)).toEqual(["root-req"]);

  expect(
    await callHandler(page, "DeleteCollection", ROOT_COLLECTION_ID),
  ).not.toBeNull();
  expect(
    await callHandler(page, "RenameCollection", ROOT_COLLECTION_ID, "x"),
  ).not.toBeNull();

  const after = await fake.snapshot();
  expect(after.rootItems).toEqual(before.rootItems);
  expect(after.sidebar).toEqual(before.sidebar);
});
