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

  /**
   * リネーム入力を確定する。fill() はイベント順によって onKeyDown が古い値を読むことがあるため、
   * click → Ctrl+A → type のキーボード操作で入力する。
   */
  async confirmRename(name: string): Promise<void> {
    const input = this.renameInput;
    await expect(input).toBeVisible();
    await input.click();
    await this.page.keyboard.press("Control+a");
    await this.page.keyboard.type(name);
    await this.page.keyboard.press("Enter");
  }

  collection(name: string): Locator {
    return this.page.getByRole("button", { name, exact: true }).first();
  }

  request(name: string | RegExp): Locator {
    return this.page.getByRole("button", { name }).first();
  }

  /**
   * Add ドロップダウンから新規コレクションを作る。
   * name を省くと既定名 "New Collection" のまま確定する。
   */
  async createCollection(name?: string): Promise<Locator> {
    await this.page.locator('[aria-label="Add"]').click();
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

  /** コレクションの行アクション（hover で現れる）を押す。 */
  private async rowAction(collectionName: string, action: string): Promise<void> {
    await this.collection(collectionName).hover();
    await this.page.getByRole("button", { name: action }).first().click();
  }

  /** コレクションにリクエストを追加する。name を省くと既定名のまま確定する。 */
  async addRequest(collectionName: string, name?: string): Promise<void> {
    await this.rowAction(collectionName, "Add request");
    await expect(this.renameInput).toBeVisible();
    if (name === undefined) {
      await this.renameInput.press("Enter");
    } else {
      await this.confirmRename(name);
    }
  }

  /** コレクションにフォルダを追加する。name を省くと既定名のまま確定する。 */
  async addFolder(collectionName: string, name?: string): Promise<void> {
    await this.rowAction(collectionName, "Add folder");
    await expect(this.renameInput).toBeVisible();
    if (name === undefined) {
      await this.renameInput.press("Escape");
    } else {
      await this.confirmRename(name);
    }
  }

  async deleteCollection(name: string): Promise<void> {
    await this.rowAction(name, "Delete collection");
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
