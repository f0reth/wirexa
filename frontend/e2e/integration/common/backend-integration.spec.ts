import type { Page } from "@playwright/test";
import {
  expect,
  readGolden,
  readStoredEntities,
  readStoredFile,
  shapeOf,
  test,
  type WailsGo,
} from "../../fixtures/integration";

// 実 Go バックエンド (wails dev) に対する疎通と永続化の確認。保存先は playwright.integration.
// config.ts が APPDATA を一時ディレクトリへ向けて隔離しているので、実行のたびにまっさらな状態から
// 始まる。前回の残骸を UI 越しに消して回る beforeEach/afterEach はもう要らない。
// ディスクへの保存は APPDATA/Wirexa 配下の JSON を直接読み、各リポジトリの
// testdata/*.golden.json と同じ形で書かれていることまで確かめる。

/** 保存ディレクトリ (collections など) から名前でエンティティを探す。 */
const findStored = <T extends { name: string }>(dir: string, name: string) =>
  readStoredEntities<T>(dir).find((e) => e.data.name === name);

/** expect.poll で書き込みを待ったあとの findStored の結果。無ければ失敗させる。 */
function storedEntity<T>(entry: { file: string; data: T } | undefined): {
  file: string;
  data: T;
} {
  if (!entry) throw new Error("stored entity disappeared after polling");
  return entry;
}

const brokerRow = (page: Page, name: string | RegExp) =>
  page.locator('[role="button"]').filter({ hasText: name });

const createBrokerProfile = async (page: Page, name: string) => {
  await page.getByRole("button", { name: "New Broker", exact: true }).click();
  const dialog = page.getByRole("dialog");
  await expect(dialog).toBeVisible();
  await dialog.getByLabel("Name", { exact: true }).fill(name);
  await dialog.getByRole("button", { name: "Save", exact: true }).click();
  await expect(dialog).toBeHidden();
  await expect(brokerRow(page, name).first()).toBeVisible();
};

// ── 観点M-1: Wails バインディングがバックエンドのデータを返す ──────────────────
// UI を通さずバインディングで直接作った項目が、リロード後の一覧取得 (GetCollections /
// GetProfiles / GetTargets) で画面に出ることを確かめる。見出しは静的文字列なので、
// バインディングが失敗しても表示されてしまい検証にならない。

test.describe("M-1: Wails binding calls return backend data", () => {
  test("collection created in the backend is listed by GetCollections", async ({
    page,
    app,
  }) => {
    const name = "E2E Binding Seeded Collection";
    await page.evaluate(async (n) => {
      const { HTTPHandler } = (window as unknown as { go: WailsGo }).go
        .adapters;
      await HTTPHandler.CreateCollection(n);
    }, name);

    await page.reload();
    await app.switchTo("HTTP");
    await expect(app.collection(name)).toBeVisible();
  });

  test("broker profile saved in the backend is listed by GetProfiles", async ({
    page,
  }) => {
    const name = "E2E Binding Seeded Broker";
    await page.evaluate(async (n) => {
      const { MQTTHandler } = (window as unknown as { go: WailsGo }).go
        .adapters;
      await MQTTHandler.SaveProfile({
        id: "",
        name: n,
        broker: "tcp://127.0.0.1:1883",
        clientId: "",
        username: "",
        password: "",
        useTls: false,
      });
    }, name);

    await page.reload();
    await expect(page.getByText("Brokers", { exact: true })).toBeVisible();
    await expect(brokerRow(page, name).first()).toBeVisible();
  });

  test("udp target saved in the backend is listed by GetTargets", async ({
    page,
    app,
  }) => {
    const name = "E2E Binding Seeded Target";
    await page.evaluate(async (n) => {
      const { UDPHandler } = (window as unknown as { go: WailsGo }).go
        .adapters;
      await UDPHandler.SaveTarget({
        id: "",
        name: n,
        host: "127.0.0.1",
        port: 9999,
      });
    }, name);

    await page.reload();
    await app.switchTo("UDP");
    await expect(page.getByText(name, { exact: true }).first()).toBeVisible();
  });
});

// ── 観点M-2: コレクションの永続化 ────────────────────────────────────────────

interface StoredCollection {
  id: string;
  name: string;
  items: Array<{ name: string }>;
}

test.describe("M-2: Collection persistence", () => {
  const NAME = "E2E Backend Integration Collection";

  test("created collection persists after page reload", async ({ page, app }) => {
    await app.switchTo("HTTP");
    await app.createCollection(NAME);

    await page.reload();
    await app.switchTo("HTTP");
    await expect(app.collection(NAME)).toBeVisible();
  });

  test("created collection is written under APPDATA/Wirexa/collections", async ({
    app,
  }) => {
    const name = "E2E Disk Collection";
    await app.switchTo("HTTP");
    await app.createCollection(name);
    await app.addRequest(name, "E2E Disk Request");

    // 作成時は既定名で書かれ、リネームとリクエストの追加で書き直される
    const find = () => findStored<StoredCollection>("collections", name);
    await expect
      .poll(() => find()?.data.items.map((i) => i.name))
      .toEqual(["E2E Disk Request"]);

    const stored = storedEntity(find());
    expect(stored.file).toBe(`${stored.data.id}.json`);
    // golden は items の先頭がフォルダなので、トップレベルのキーと、リクエストの
    // アイテム (golden の "Minimal") の形とを分けて比べる。
    const golden = readGolden<StoredCollection>("http", "collection");
    expect(Object.keys(stored.data).sort()).toEqual(
      Object.keys(golden).sort(),
    );
    expect(shapeOf(stored.data.items[0])).toEqual(shapeOf(golden.items[1]));
  });
});

// ── 観点M-3: MQTT ブローカープロファイルの永続化 ─────────────────────────────

test.describe("M-3: MQTT broker profile persistence", () => {
  const NAME = "E2E Backend Integration Broker";

  test("mqtt broker profile persists after page reload", async ({ page }) => {
    await createBrokerProfile(page, NAME);

    await page.reload();
    await expect(page.getByText("Brokers", { exact: true })).toBeVisible();
    await expect(brokerRow(page, NAME).first()).toBeVisible();
  });

  test("saved broker profile is written under APPDATA/Wirexa/mqtt-profiles", async ({
    page,
  }) => {
    const name = "E2E Disk Broker";
    await createBrokerProfile(page, name);

    const find = () => findStored<{ id: string; name: string }>(
      "mqtt-profiles",
      name,
    );
    await expect.poll(() => find()?.data.name).toBe(name);

    const stored = storedEntity(find());
    expect(stored.file).toBe(`${stored.data.id}.json`);
    // ダイアログの既定値 (mqtt://localhost:1883) で作っている
    expect(stored.data).toMatchObject({
      name,
      broker: "mqtt://localhost:1883",
    });
    expect(shapeOf(stored.data)).toEqual(
      shapeOf(readGolden("mqtt", "profile")),
    );
  });
});

// ── 観点M-4: UDP ターゲットの永続化 ──────────────────────────────────────────

test.describe("M-4: UDP target persistence", () => {
  const NAME = "E2E Backend Integration Target";

  test("udp target persists after page reload", async ({ page, app }) => {
    await app.switchTo("UDP");
    await app.createUdpTarget(NAME, "127.0.0.1", 9999);

    await page.reload();
    await app.switchTo("UDP");
    await expect(page.getByText(NAME, { exact: true }).first()).toBeVisible();
  });

  test("saved udp target is written under APPDATA/Wirexa/udp-targets", async ({
    app,
  }) => {
    const name = "E2E Disk Target";
    await app.switchTo("UDP");
    await app.createUdpTarget(name, "127.0.0.1", 9998);

    const find = () => findStored<{ id: string; name: string }>(
      "udp-targets",
      name,
    );
    await expect.poll(() => find()?.data.name).toBe(name);

    const stored = storedEntity(find());
    expect(stored.file).toBe(`${stored.data.id}.json`);
    expect(stored.data).toEqual({
      id: expect.any(String),
      name,
      host: "127.0.0.1",
      port: 9998,
    });
    expect(shapeOf(stored.data)).toEqual(shapeOf(readGolden("udp", "target")));
  });
});

// ── 観点M-8: サイドバーレイアウトの永続化 ────────────────────────────────────
// HTTP のコレクションとルート直下アイテムの並び (sidebar_layout.json)。ブローカーの並びは
// localStorage (mqtt:profileOrder) に保存されるので、UI モードの e2e/ui/mqtt で確かめる。

test.describe("M-8: Sidebar layout persistence", () => {
  const FIRST = "E2E Layout First";
  const SECOND = "E2E Layout Second";

  test("reordering collections persists sidebar_layout.json", async ({
    page,
    app,
  }) => {
    await app.switchTo("HTTP");
    await app.createCollection(FIRST);
    await app.createCollection(SECOND);
    const rows = page.getByRole("button", {
      name: /^E2E Layout (First|Second)$/,
    });
    await expect(rows).toHaveText([FIRST, SECOND]);

    const idOf = (name: string) => () =>
      findStored<StoredCollection>("collections", name)?.data.id;
    await expect.poll(idOf(FIRST)).toBeDefined();
    await expect.poll(idOf(SECOND)).toBeDefined();
    const firstId = idOf(FIRST)();
    const secondId = idOf(SECOND)();

    // 前のテストのコレクションも並んでいるので、位置ではなく First の直前のゾーンに落とす
    await app.dragTreeNode(
      app.collection(SECOND),
      app.dropZoneBefore(app.collection(FIRST)),
    );
    await expect(rows).toHaveText([SECOND, FIRST]);

    const layout = () =>
      readStoredFile<Array<{ kind: string; id: string }>>(
        "sidebar_layout.json",
      ) ?? [];
    await expect
      .poll(() =>
        layout()
          .filter((e) => e.id === firstId || e.id === secondId)
          .map((e) => [e.kind, e.id]),
      )
      .toEqual([
        ["collection", secondId],
        ["collection", firstId],
      ]);
    expect(shapeOf(layout())).toEqual(
      shapeOf(readGolden("http", "sidebar_layout")),
    );

    // アプリが保存した並びを読み直しても保たれる
    await page.reload();
    await app.switchTo("HTTP");
    await expect(rows).toHaveText([SECOND, FIRST]);
  });
});
