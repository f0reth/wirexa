import { type Locator, type Page, expect } from "@playwright/test";

export type Protocol = "MQTT" | "HTTP" | "UDP" | "OpenAPI";

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

  // ── UDP ─────────────────────────────────────────────────────────────────────

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
