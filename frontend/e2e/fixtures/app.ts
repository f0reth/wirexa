import { type Locator, type Page, expect } from "@playwright/test";

export type Protocol = "MQTT" | "HTTP" | "UDP" | "OpenAPI";

/** 要素の中心のビューポート座標。表示されていなければ (display: none など) 失敗させる。 */
async function centerOf(locator: Locator): Promise<{ x: number; y: number }> {
  const box = await locator.boundingBox();
  if (!box) throw new Error(`element is not visible: ${locator}`);
  return { x: box.x + box.width / 2, y: box.y + box.height / 2 };
}

/**
 * アプリ操作のページオブジェクト。UI モック版 (e2e/ui) と実バックエンド版 (e2e/integration) の
 * 両方から使う。ここに集約する前は同じヘルパーが 6 ファイルにコピペされていた。
 */
export class App {
  constructor(readonly page: Page) {}

  // ── 共通 ────────────────────────────────────────────────────────────────────

  protocolButton(protocol: Protocol): Locator {
    return this.page.getByRole("button", { name: protocol, exact: true });
  }

  /** プロトコルを切り替え、そのパネルのサイドバー見出しが出るまで待つ。 */
  async switchTo(protocol: Protocol): Promise<void> {
    await this.protocolButton(protocol).click();
    const heading: Record<Protocol, string> = {
      MQTT: "Brokers",
      HTTP: "Collections",
      UDP: "Targets",
      OpenAPI: "OpenAPI Files",
    };
    await expect(
      this.page.getByText(heading[protocol], { exact: protocol !== "OpenAPI" }),
    ).toBeVisible();
  }

  /** 確認ダイアログの Delete を押す。 */
  async confirmDelete(): Promise<void> {
    await this.page
      .getByRole("dialog")
      .getByRole("button", { name: "Delete" })
      .click();
  }

  // ── HTTP: サイドバー ────────────────────────────────────────────────────────

  get renameInput(): Locator {
    return this.page.getByTestId("rename-input");
  }

  /** リネーム入力を置き換えて確定する。 */
  async confirmRename(name: string): Promise<void> {
    const input = this.renameInput;
    await expect(input).toBeVisible();
    await input.fill(name);
    await input.press("Enter");
  }

  /** コレクションの開閉トグル (名前をクリックすると開閉する)。 */
  collection(name: string): Locator {
    return this.page.getByRole("button", { name, exact: true }).first();
  }

  /** フォルダの開閉トグル。コレクションと同じ形のボタンなので、名前が重ならないようにして使う。 */
  folder(name: string): Locator {
    return this.page.getByRole("button", { name, exact: true }).first();
  }

  /** リクエストの選択ボタン。アクセシブル名はメソッド付き ("GET Name") なので正規表現で渡す。 */
  request(name: string | RegExp): Locator {
    return this.page.getByRole("button", { name }).first();
  }

  /**
   * サイドバーの行アクション (Add folder / Add request / Delete ...) のボタン。
   * 親子の行が同名のアクションを持つので、node (collection / folder / request の戻り値) の行でスコープする。
   */
  rowAction(node: Locator, action: string): Locator {
    return node
      .locator("xpath=..")
      .getByRole("button", { name: action, exact: true });
  }

  /** 表示名をダブルクリックしてリネーム入力を開く。 */
  async startRename(node: Locator, name: string): Promise<void> {
    await node.getByText(name, { exact: true }).dblclick();
    await expect(this.renameInput).toBeVisible();
  }

  private async openAddMenu(): Promise<void> {
    await this.page.locator('[aria-label="Add"]').click();
  }

  /**
   * Add ドロップダウンから新規コレクションを作る。
   * name を省くと既定名 "New Collection" のまま確定する。
   */
  async createCollection(name?: string): Promise<Locator> {
    await this.openAddMenu();
    await this.page
      .getByRole("button", { name: "New Collection" })
      .first()
      .click();
    await expect(this.renameInput).toBeVisible();
    if (name === undefined) {
      await this.renameInput.press("Enter");
    } else {
      await this.confirmRename(name);
    }
    const created = this.collection(name ?? "New Collection");
    await expect(created).toBeVisible();
    return created;
  }

  /** Add ドロップダウンの New Request で、どのコレクションにも属さないリクエストを作る。 */
  async createRootRequest(name: string): Promise<Locator> {
    await this.openAddMenu();
    await this.page
      .getByRole("button", { name: "New Request", exact: true })
      .click();
    await this.confirmRename(name);
    const escaped = name.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
    const created = this.request(new RegExp(` ${escaped}$`));
    await expect(created).toBeVisible();
    return created;
  }

  /**
   * コレクションかフォルダにリクエストを追加する。name を省くと既定名のまま確定する。
   * フォルダは既定で閉じており、閉じたフォルダに足すとリネーム入力が出ないので、先に開いておく。
   */
  async addRequest(parentName: string, name?: string): Promise<void> {
    await this.rowAction(this.collection(parentName), "Add request").click();
    await expect(this.renameInput).toBeVisible();
    if (name === undefined) {
      await this.renameInput.press("Enter");
    } else {
      await this.confirmRename(name);
    }
  }

  /** コレクションかフォルダにフォルダを追加する。name を省くと既定名のまま確定する。 */
  async addFolder(parentName: string, name?: string): Promise<void> {
    await this.rowAction(this.collection(parentName), "Add folder").click();
    await expect(this.renameInput).toBeVisible();
    if (name === undefined) {
      await this.renameInput.press("Escape");
    } else {
      await this.confirmRename(name);
    }
  }

  /**
   * サイドバー最上位 (コレクションとルートのアイテムの並び) の挿入ゾーン。
   * position はサイドバーレイアウト上の挿入位置。
   */
  sidebarDropZone(position: number): Locator {
    return this.page.locator(
      `[data-drop-zone][data-drop-kind="sidebar"][data-drop-position="${position}"]`,
    );
  }

  /**
   * コレクションかフォルダ (node) の直前にある挿入ゾーン。レイアウト上の位置が分からない
   * (前のテストの残りがある fullstack など) ときに sidebarDropZone の代わりに使う。
   */
  dropZoneBefore(node: Locator): Locator {
    // node → 見出し行 → ノード全体、の直前の兄弟が挿入ゾーン。
    return node.locator(
      "xpath=../../preceding-sibling::div[1][@data-drop-zone]",
    );
  }

  /** コレクションかフォルダ (node) の子の末尾にある挿入ゾーン。node が開いているときだけある。 */
  childrenEndDropZone(node: Locator): Locator {
    // 見出し行 (node の親) の次の兄弟が子の一覧で、その最後の挿入ゾーンが末尾。
    return node.locator(
      "xpath=../following-sibling::div[1]/div[@data-drop-zone][last()]",
    );
  }

  /**
   * ツリーのマウス方式 D&D (use-long-press-drag.ts → use-tree-drag-drop.ts) で source を
   * target の中心へ落とす。HTML5 DnD ではないので locator.dragTo() は使えない。
   * 5px を超えて動かした時点でドラッグが始まり、挿入ゾーンが広がって行の位置がずれるので、
   * ドロップ先の座標はドラッグを始めてから取る。
   */
  async dragTreeNode(source: Locator, target: Locator): Promise<void> {
    const mouse = this.page.mouse;
    await source.scrollIntoViewIfNeeded();
    const from = await centerOf(source);
    await mouse.move(from.x, from.y);
    await mouse.down();
    await mouse.move(from.x, from.y + 6);
    await target.scrollIntoViewIfNeeded();
    const to = await centerOf(target);
    await mouse.move(to.x, to.y, { steps: 5 });
    await mouse.up();
  }

  async deleteCollection(name: string): Promise<void> {
    await this.rowAction(this.collection(name), "Delete collection").click();
    await this.confirmDelete();
    await expect(this.collection(name)).toBeHidden();
  }

  // ── HTTP: リクエストバー ────────────────────────────────────────────────────

  get urlInput(): Locator {
    return this.page.getByPlaceholder("https://api.example.com/endpoint");
  }

  get sendButton(): Locator {
    return this.page.getByRole("button", { name: "Send", exact: true });
  }

  get cancelButton(): Locator {
    return this.page.getByRole("button", { name: "Cancel", exact: true });
  }

  /** リクエストのメソッド選択。 */
  async selectMethod(method: string): Promise<void> {
    const methodSelect = this.page.getByTestId("method-select");
    await methodSelect.getByRole("button").first().click();
    await methodSelect.getByRole("button", { name: method, exact: true }).click();
  }

  // ── HTTP: リクエスト編集エリア ──────────────────────────────────────────────

  /**
   * リクエスト編集エリア (Params〜Doc のタブとそのパネル)。"Body" / "Headers" タブは
   * レスポンス側と同名で tabpanel の id も衝突するため、Params タブを持つタブ列の親でスコープする。
   */
  get requestEditor(): Locator {
    return this.page
      .getByRole("tablist")
      .filter({
        has: this.page.getByRole("tab", { name: "Params", exact: true }),
      })
      .locator("xpath=..");
  }

  /** リクエスト編集エリアのタブを開き、そのパネルを返す。 */
  async openRequestTab(name: string): Promise<Locator> {
    const editor = this.requestEditor;
    await editor.getByRole("tab", { name, exact: true }).click();
    return editor.getByRole("tabpanel");
  }

  /**
   * パネル内の Select で項目を選ぶ。トリガーには現在の値がそのまま表示されるので
   * current で指定する (例: Body の種別なら "none")。
   */
  async chooseOption(
    panel: Locator,
    current: string,
    label: string,
  ): Promise<void> {
    await panel.getByRole("button", { name: current, exact: true }).click();
    await panel.getByRole("button", { name: label, exact: true }).click();
  }

  /** Headers / Params のキー・値の行を 1 行足して埋める。 */
  async addKeyValue(
    panel: Locator,
    keyPlaceholder: string,
    key: string,
    value: string,
  ): Promise<void> {
    await panel.getByRole("button", { name: "Add", exact: true }).click();
    // Add は末尾に行を足すので、最後の行が今足した行。
    await panel.getByPlaceholder(keyPlaceholder).last().fill(key);
    await panel.getByPlaceholder("Value").last().fill(value);
  }

  // ── ブローカー・ターゲットの一覧 (profile-list.tsx) ─────────────────────────

  /**
   * 一覧の行 (role="button" の div) を HTML5 DnD (list-reorder.tsx) で target の前へ動かす。
   * 行の上半分に落とすとその行の前に入る。ドラッグが始まると挿入ゾーンが広がって行が下へ
   * ずれるので、上端から少し離して落とす。
   */
  async dragListRowBefore(source: Locator, target: Locator): Promise<void> {
    const box = await target.boundingBox();
    if (!box) throw new Error(`element is not visible: ${target}`);
    await source.dragTo(target, {
      targetPosition: { x: box.width / 2, y: box.height / 3 },
    });
  }

  // ── UDP ─────────────────────────────────────────────────────────────────────

  /** サイドバーのターゲット行。行には名前と host:port が並ぶ。 */
  udpTarget(name: string): Locator {
    return this.page
      .getByRole("button")
      .filter({ has: this.page.getByText(name, { exact: true }) });
  }

  /**
   * ターゲットを選んで送信フォームに読み込む。行をクリックすると、ホバーで出る Edit / Delete
   * ボタンに当たることがあるので、フォーカスして Enter で選ぶ。
   */
  async selectUdpTarget(name: string): Promise<void> {
    const row = this.udpTarget(name);
    await row.focus();
    await row.press("Enter");
    await expect(
      this.page.getByRole("button", { name: "Listen", exact: true }),
    ).toBeVisible();
  }

  /**
   * Send / Listen タブを開く。タブは role="tab" ではない素のボタンで、Send タブは送信ボタンと
   * 同名になる。タブバーは送信フォームより前にあるので先頭を取る。
   */
  async openUdpTab(name: "Send" | "Listen"): Promise<void> {
    await this.page.getByRole("button", { name, exact: true }).first().click();
  }

  /**
   * 送信フォームの送信ボタン。Send タブと同名なので末尾を取る。送信中は "Sending..." に
   * 変わるので、このロケーターでは取れない。
   */
  get udpSendButton(): Locator {
    return this.page.getByRole("button", { name: "Send", exact: true }).last();
  }

  /** 固定長ペイロードの index 番目 (0 始まり) のフィールド。Name / Type / Length / Value とバイト数を含む。 */
  udpFixedField(index: number): Locator {
    // Name 欄から、Value 欄まで含む最も近い祖先 (フィールド 1 件分の枠) へ上がる。
    return this.page
      .getByLabel("Field name")
      .nth(index)
      .locator("xpath=ancestor::div[.//*[@aria-label='Field value']][1]");
  }

  async createUdpTarget(
    name: string,
    host: string,
    port: number,
  ): Promise<void> {
    await this.page.getByRole("button", { name: "New Target" }).click();
    const dialog = this.page.getByRole("dialog");
    await expect(dialog).toBeVisible();
    await dialog.getByLabel("Name").fill(name);
    await dialog.getByLabel("Host").fill(host);
    await dialog.getByLabel("Port").fill(String(port));
    await dialog.getByRole("button", { name: "Save" }).click();
    await expect(dialog).toBeHidden();
    await expect(this.page.getByText(name, { exact: true }).first()).toBeVisible();
  }

  // ── OpenAPI ─────────────────────────────────────────────────────────────────

  /** OpenAPI エディタ (CodeMirror) の編集領域。CodeMirror の内部クラスへの依存をここに閉じ込める。 */
  get openApiEditor(): Locator {
    return this.page.locator(".cm-content");
  }

  /** OpenAPI エディタの内容を text で置き換える。 */
  async fillOpenApiEditor(text: string): Promise<void> {
    await this.page.locator(".cm-editor").click();
    await this.page.evaluate((t) => {
      const el = document.querySelector(".cm-content") as HTMLElement | null;
      if (!el) return;
      el.focus();
      document.execCommand("selectAll");
      document.execCommand("insertText", false, t);
    }, text);
  }

  /** サイドバーの New で無題文書を作る (未保存の文書があれば確認ダイアログが出る)。 */
  async newOpenApiDocument(): Promise<void> {
    await this.page.getByRole("button", { name: "New (paste a spec)" }).click();
  }

  // ── MQTT ────────────────────────────────────────────────────────────────────

  async createBrokerProfile(name: string): Promise<void> {
    await this.page.getByRole("button", { name: "New Broker" }).click();
    const dialog = this.page.getByRole("dialog");
    await expect(dialog).toBeVisible();
    await dialog.getByLabel("Name", { exact: true }).fill(name);
    await dialog.getByRole("button", { name: "Save", exact: true }).click();
    await expect(dialog).toBeHidden();
  }
}
