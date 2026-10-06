import { type Locator, type Page, expect } from "@playwright/test";

export type Protocol = "MQTT" | "HTTP" | "UDP" | "OpenAPI";

/** 要素の中心のビューポート座標。表示されていなければ (display: none など) 失敗させる。 */
async function centerOf(locator: Locator): Promise<{ x: number; y: number }> {
  const box = await locator.boundingBox();
  if (!box) throw new Error(`element is not visible: ${locator}`);
  return { x: box.x + box.width / 2, y: box.y + box.height / 2 };
}

/**
 * CodeMirror のエディタ (App.editor で取る)。CodeMirror の内部クラス (.cm-*) には
 * アクセシブルな代わりが無いので、その依存をここに閉じ込める。
 */
export class CodeMirrorEditor {
  constructor(private readonly scope: Locator | Page) {}

  /** エディタ全体。表示の有無を見るときに使う。 */
  get root(): Locator {
    return this.scope.locator(".cm-editor");
  }

  /** 編集領域。クリックしてフォーカスを移し、内容を toHaveText などで見る。 */
  get content(): Locator {
    return this.root.locator(".cm-content");
  }

  /** linter がエラーとして付けた印 (位置のエラーと範囲のエラー)。 */
  get lintErrors(): Locator {
    return this.root.locator(".cm-lintPoint-error, .cm-lintRange-error");
  }

  /** 内容を text で置き換える。 */
  async fill(text: string): Promise<void> {
    await this.root.click();
    await this.content.evaluate((el, t) => {
      (el as HTMLElement).focus();
      document.execCommand("selectAll");
      document.execCommand("insertText", false, t);
    }, text);
  }
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

  /**
   * CodeMirror のエディタ。HTTP の Body / Doc パネルのように画面に複数ありうるので、
   * 使う側のパネル (scope) でスコープする。OpenAPI パネルのエディタは 1 つなので省いてよい。
   */
  editor(scope: Locator | Page = this.page): CodeMirrorEditor {
    return new CodeMirrorEditor(scope);
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

  /**
   * サイドバーの見出し "Collections" の行。Add ボタンとそのドロップダウンを含む。
   * リクエスト編集エリアの key-value エディタにも同名の "Add" ボタンがあり、
   * 作成済みのコレクション "New Collection" はメニュー項目と同名なので、この行でスコープする。
   */
  private get collectionsHeader(): Locator {
    return this.page
      .getByText("Collections", { exact: true })
      .locator("xpath=..");
  }

  private async openAddMenu(): Promise<void> {
    await this.collectionsHeader
      .getByRole("button", { name: "Add", exact: true })
      .click();
  }

  /**
   * Add ドロップダウンから新規コレクションを作る。
   * name を省くと既定名 "New Collection" のまま確定する。
   */
  async createCollection(name?: string): Promise<Locator> {
    await this.openAddMenu();
    await this.collectionsHeader
      .getByRole("button", { name: "New Collection", exact: true })
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
    await this.collectionsHeader
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

  /** 自動保存の失敗を知らせるバナー。閉じるボタン (Dismiss) を含む。 */
  get saveErrorBanner(): Locator {
    return this.page.getByText(/^Save failed: /).locator("xpath=..");
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

  /**
   * リクエスト編集エリアの、タブ name のパネル。表示中のタブのパネルだけが DOM にあるので、
   * タブを開かずに使うのは既定の Params か、開いてあるタブのときだけ。
   */
  requestTabPanel(name: string): Locator {
    return this.requestEditor.getByRole("tabpanel", { name, exact: true });
  }

  /** リクエスト編集エリアのタブを開き、そのパネルを返す。 */
  async openRequestTab(name: string): Promise<Locator> {
    await this.requestEditor.getByRole("tab", { name, exact: true }).click();
    return this.requestTabPanel(name);
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

  // ── HTTP: レスポンス表示エリア ──────────────────────────────────────────────

  /** レスポンス表示エリア。見出し "Response" → 見出し行 → パネル、と上がる。 */
  get responseViewer(): Locator {
    return this.page
      .getByText("Response", { exact: true })
      .locator("xpath=../..");
  }

  /**
   * レスポンス表示エリアのタブ (Body / Headers / Timing) を開き、その内容の領域を返す。
   * "Body" / "Headers" はリクエスト編集エリアにもあるので、レスポンス表示エリアでスコープする。
   */
  async openResponseTab(name: "Body" | "Headers" | "Timing"): Promise<Locator> {
    const viewer = this.responseViewer;
    await viewer.getByRole("tab", { name, exact: true }).click();
    return viewer.getByRole("tabpanel", { name, exact: true });
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
      this.page.getByRole("tab", { name: "Listen", exact: true }),
    ).toBeVisible();
  }

  /** Send / Listen タブを開く。 */
  async openUdpTab(name: "Send" | "Listen"): Promise<void> {
    await this.page.getByRole("tab", { name, exact: true }).click();
  }

  /**
   * 送信フォームの送信ボタン。Send タブは role="tab" なので同名でも当たらない。
   * 送信中は "Sending..." に変わるので、このロケーターでは取れない。
   */
  get udpSendButton(): Locator {
    return this.page.getByRole("button", { name: "Send", exact: true });
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

  /** OpenAPI エディタ (CodeMirror) の編集領域。 */
  get openApiEditor(): Locator {
    return this.editor().content;
  }

  /** OpenAPI エディタの内容を text で置き換える。 */
  async fillOpenApiEditor(text: string): Promise<void> {
    await this.editor().fill(text);
  }

  /** サイドバーの New で無題文書を作る (未保存の文書があれば確認ダイアログが出る)。 */
  async newOpenApiDocument(): Promise<void> {
    await this.page.getByRole("button", { name: "New (paste a spec)" }).click();
  }

  /** 最近使ったファイル一覧の、名前が name のファイルを選ぶボタン。 */
  openApiFile(name: string): Locator {
    return this.page.getByRole("button", { name, exact: true });
  }

  /**
   * 最近使ったファイル一覧の、ファイルを選ぶボタンすべて (表示順)。一覧は挿入ゾーンとファイル行が
   * 交互に並び、行の先頭のボタンが選択ボタン (2 つ目は Remove from history)。
   */
  get openApiFiles(): Locator {
    return this.page.locator(
      "[data-openapi-drop-zone] + div > button:first-child",
    );
  }

  /** 最近使ったファイル行の Remove from history ボタン。行ごとにあるので name の行でスコープする。 */
  removeOpenApiFileButton(name: string): Locator {
    return this.openApiFile(name)
      .locator("xpath=..")
      .getByRole("button", { name: "Remove from history" });
  }

  /**
   * 最近使ったファイル一覧の挿入ゾーン。index は移動前の一覧に対する挿入位置で、
   * ファイル数と同じ値なら末尾。ドラッグ中だけ高さを持つので dragTreeNode の target に使う。
   */
  openApiDropZone(index: number): Locator {
    return this.page.locator(
      `[data-openapi-drop-zone][data-openapi-drop-index="${index}"]`,
    );
  }

  /**
   * OS からのファイル D&D を模して、name・content のファイルを target に落とす。
   * target は OpenAPI パネルの中なら何でもよい (drop はパネル全体で受ける)。ただし
   * CodeMirror はエディタへのファイルのドロップを自前で挿入するので、エディタは避ける。
   * dragover が受理された (preventDefault された) かを返す。
   */
  async dropOpenApiFile(
    target: Locator,
    name: string,
    content: string,
  ): Promise<boolean> {
    return target.evaluate(
      (el, [n, c]) => {
        const dataTransfer = new DataTransfer();
        dataTransfer.items.add(new File([c], n, { type: "application/yaml" }));
        const init = { bubbles: true, cancelable: true, dataTransfer };
        const dragOver = new DragEvent("dragover", init);
        el.dispatchEvent(dragOver);
        el.dispatchEvent(new DragEvent("drop", init));
        return dragOver.defaultPrevented;
      },
      [name, content] as const,
    );
  }

  // ── MQTT ────────────────────────────────────────────────────────────────────

  /**
   * New Broker のダイアログでプロファイルを作り、action のボタンで閉じる。scheme・host・port を
   * 省くとダイアログの既定値 (mqtt://localhost:1883) のまま、clientId・username・password を省くと
   * 空のまま保存する。
   */
  async createBrokerProfile(
    name: string,
    options: {
      scheme?: "mqtt" | "mqtts" | "tcp" | "ws" | "wss";
      host?: string;
      port?: number;
      clientId?: string;
      username?: string;
      password?: string;
      action?: "Save" | "Save & Connect";
    } = {},
  ): Promise<void> {
    const dialog = await this.openNewBrokerDialog();
    await dialog.getByLabel("Name", { exact: true }).fill(name);
    // スキームを変えるとポートが既定値に戻るので、ポートより先に選ぶ。
    if (options.scheme !== undefined) {
      await this.brokerSchemeSelect(dialog).selectOption(options.scheme);
    }
    if (options.clientId !== undefined) {
      await dialog.getByLabel("Client ID").fill(options.clientId);
    }
    if (options.username !== undefined) {
      await dialog.getByLabel("Username").fill(options.username);
    }
    if (options.password !== undefined) {
      await dialog.getByLabel("Password").fill(options.password);
    }
    if (options.host !== undefined) {
      await this.brokerHostInput(dialog).fill(options.host);
    }
    if (options.port !== undefined) {
      await this.brokerPortInput(dialog).fill(String(options.port));
    }
    await dialog
      .getByRole("button", { name: options.action ?? "Save", exact: true })
      .click();
    await expect(dialog).toBeHidden();
  }

  /** ブローカーの追加・編集ダイアログ。title を省くとどちらでも当たる。 */
  brokerDialog(title?: "New Profile" | "Edit Profile"): Locator {
    return this.page.getByRole("dialog", { name: title });
  }

  /** サイドバーの New Broker を押してブローカーの追加ダイアログを開き、ダイアログを返す。 */
  async openNewBrokerDialog(): Promise<Locator> {
    await this.page.getByRole("button", { name: "New Broker" }).click();
    const dialog = this.brokerDialog("New Profile");
    await expect(dialog).toBeVisible();
    return dialog;
  }

  /** ブローカーをどれも選んでいないときに、MQTT パネルに出る案内。 */
  get mqttEmptyState(): Locator {
    return this.page.getByText(
      "No active connection. Select a broker from the sidebar to connect.",
    );
  }

  /** サイドバーのブローカー行。行には名前とブローカー URL が並ぶ。 */
  broker(name: string): Locator {
    return this.page
      .getByRole("button")
      .filter({ has: this.page.getByText(name, { exact: true }) });
  }

  /**
   * サイドバーの行のボタン (Edit / Delete)。ホバーで出るので、行をホバーしてから返す。
   */
  async brokerRowAction(
    name: string,
    action: "Edit broker" | "Delete broker",
  ): Promise<Locator> {
    const row = this.broker(name);
    await row.hover();
    return row.getByRole("button", { name: action, exact: true });
  }

  /**
   * ブローカーを選び、接続バーに Connect が出るまで待つ。接続済みのブローカーは Connect が
   * 出ないので、connected: true を渡して Disconnect が出るまで待つ。行をクリックすると、
   * ホバーで出る Edit / Delete ボタンに当たることがあるので、フォーカスして Enter で選ぶ。
   */
  async selectBroker(
    name: string,
    options: { connected?: boolean } = {},
  ): Promise<void> {
    const row = this.broker(name);
    await row.focus();
    await row.press("Enter");
    await expect(
      options.connected ? this.brokerDisconnectButton : this.brokerConnectButton,
    ).toBeVisible();
  }

  /** 接続バーの Connect ボタン。未接続のブローカーを選んでいるときだけ出る。 */
  get brokerConnectButton(): Locator {
    return this.page.getByRole("button", { name: "Connect", exact: true });
  }

  /** 接続バーの Disconnect ボタン。接続済みのブローカーを選んでいるときだけ出る。 */
  get brokerDisconnectButton(): Locator {
    return this.page.getByRole("button", { name: "Disconnect", exact: true });
  }

  /** 接続バーの接続状態の表示。 */
  mqttStatus(text: "Connected" | "Disconnected"): Locator {
    return this.page.getByText(text, { exact: true });
  }

  /**
   * ブローカー URL のホスト欄。scope を省くと接続バー、ダイアログを渡すとダイアログの欄。
   * 接続バーとダイアログの欄は同じラベルなので、接続バーの欄はダイアログを閉じた状態で使う。
   */
  brokerHostInput(scope: Locator | Page = this.page): Locator {
    return scope.getByLabel("Broker host");
  }

  /** ブローカー URL のポート欄。scope は brokerHostInput と同じ。 */
  brokerPortInput(scope: Locator | Page = this.page): Locator {
    return scope.getByLabel("Broker port");
  }

  /** ブローカー URL のスキーム欄 (ネイティブ select)。scope は brokerHostInput と同じ。 */
  brokerSchemeSelect(scope: Locator | Page = this.page): Locator {
    return scope.getByLabel("Broker scheme");
  }

  /** サイドバーの行の Edit ボタンからブローカーの編集ダイアログを開き、ダイアログを返す。 */
  async openBrokerEditDialog(name: string): Promise<Locator> {
    await (await this.brokerRowAction(name, "Edit broker")).click();
    const dialog = this.brokerDialog("Edit Profile");
    await expect(dialog).toBeVisible();
    return dialog;
  }

  /** ブローカーを選んで Connect を押し、Connected になるまで待つ。 */
  async connectBroker(name: string): Promise<void> {
    await this.selectBroker(name);
    await this.brokerConnectButton.click();
    await expect(this.mqttStatus("Connected")).toBeVisible();
  }

  /**
   * 画面を通さずに Connect のバインディングを直接呼ぶ。失敗したらそのメッセージを、成功したら
   * null を返す。画面から送れない値 (TLS で使えないスキームなど) の検証を確かめるのに使う。
   */
  async mqttConnectError(config: {
    broker: string;
    useTls: boolean;
  }): Promise<string | null> {
    return this.page.evaluate(async (c) => {
      const { MQTTHandler } = (
        window as unknown as {
          go: {
            adapters: {
              MQTTHandler: { Connect(config: unknown): Promise<string> };
            };
          };
        }
      ).go.adapters;
      try {
        await MQTTHandler.Connect({
          name: "direct",
          clientId: "",
          username: "",
          password: "",
          profileId: "",
          ...c,
        });
        return null;
      } catch (err) {
        // Wails はエラーを文字列で、偽バックエンドは Error で返す。
        return err instanceof Error ? err.message : String(err);
      }
    }, config);
  }

  /** MQTT パネルの Subscribe / Publish タブ。 */
  mqttTab(name: "Subscribe" | "Publish"): Locator {
    return this.page.getByRole("tab", { name, exact: true });
  }

  /** MQTT パネルのタブを切り替える。 */
  async openMqttTab(name: "Subscribe" | "Publish"): Promise<void> {
    await this.mqttTab(name).click();
  }

  /**
   * 見出し (Subscriptions / Messages / Publish など) を持つ MQTT のパネル。
   * 見出し → 見出し行 → パネル、と上がる。非表示のタブの見出しは getByRole が拾わないので、
   * Publish タブにも同名の "Messages" があっても表示中のタブの方だけが当たる。
   */
  mqttSection(title: string): Locator {
    return this.page
      .getByRole("heading", { name: title, exact: true })
      .locator("xpath=ancestor::div[2]");
  }

  /** Subscriptions パネルの購読 1 件分の行。Mute / Unmute ボタンを持つ最も近い祖先を取る。 */
  mqttSubscription(topic: string): Locator {
    return this.mqttSection("Subscriptions")
      .getByText(topic, { exact: true })
      .locator(
        "xpath=ancestor::div[.//button[@title='Mute' or @title='Unmute']][1]",
      );
  }

  /** Subscriptions パネルのトピックの入力欄。 */
  get mqttTopicInput(): Locator {
    return this.mqttSection("Subscriptions").getByPlaceholder(
      "Topic (e.g., sensors/#)",
    );
  }

  /** Subscriptions パネルの Subscribe ボタン。 */
  get mqttSubscribeButton(): Locator {
    return this.mqttSection("Subscriptions").getByRole("button", {
      name: "Subscribe",
      exact: true,
    });
  }

  /**
   * Subscriptions パネルから購読し、購読の行が出るまで待つ。qos を省くと選び直さない (QoS 0)。
   */
  async subscribeMqtt(topic: string, qos?: 1 | 2): Promise<void> {
    await this.mqttTopicInput.fill(topic);
    // QoS の Select はトリガーに現在の値が出る。
    if (qos !== undefined) {
      await this.chooseOption(
        this.mqttSection("Subscriptions"),
        "0",
        `QoS ${qos}`,
      );
    }
    await this.mqttSubscribeButton.click();
    await expect(this.mqttSubscription(topic)).toBeVisible();
  }

  /** 購読の行の削除ボタン。 */
  removeMqttSubscriptionButton(topic: string): Locator {
    return this.mqttSubscription(topic).getByRole("button", {
      name: "Remove subscription",
    });
  }

  /** Broker Topics パネルの Scan ボタン。スキャン中は Stop に替わる。 */
  get mqttScanButton(): Locator {
    return this.mqttSection("Broker Topics").getByRole("button", {
      name: "Scan",
      exact: true,
    });
  }

  /** Broker Topics パネルの Stop ボタン。スキャン中だけ出る。 */
  get mqttStopScanButton(): Locator {
    return this.mqttSection("Broker Topics").getByRole("button", {
      name: "Stop",
      exact: true,
    });
  }

  /** Messages パネルの見出し行のボタン。 */
  mqttMessagesAction(name: "Auto" | "Clear"): Locator {
    return this.mqttSection("Messages").getByRole("button", {
      name,
      exact: true,
    });
  }

  /** Messages パネルのトピックの絞り込み (ネイティブ select)。値が空なら絞り込まない。 */
  get mqttTopicFilter(): Locator {
    return this.page.getByRole("combobox", { name: "Filter by topic" });
  }

  /**
   * Messages パネルの一覧に描かれている行。一覧は仮想スクロールで、表示中の行だけを
   * data-index (一覧の中の位置) 付きで描く。行の位置や並びを測るときに使うもので、
   * アクセシブルな代わりが無いので DOM の属性で取る。項目の中身は mqttMessages で見る。
   */
  get mqttMessageRows(): Locator {
    return this.mqttSection("Messages").locator("[data-index]");
  }

  /** Messages パネルの一覧の項目 (ボタン)。見出し行の Auto / Clear は除く。 */
  get mqttMessages(): Locator {
    return this.mqttSection("Messages")
      .getByRole("button")
      .filter({ hasNotText: /^(Auto|Clear)$/ });
  }

  /** ペイロードが payload の、Messages パネルの一覧の項目。 */
  mqttMessage(payload: string): Locator {
    return this.mqttMessages.filter({
      has: this.page.getByText(payload, { exact: true }),
    });
  }

  /** Publish タブのフォームと、その入力欄・送信ボタン。Publish タブを開いた状態で使う。 */
  mqttPublishForm() {
    const form = this.mqttSection("Publish");
    return {
      form,
      topic: form.getByPlaceholder("Topic", { exact: true }),
      payload: form.getByPlaceholder("Message payload"),
      retain: form.getByRole("checkbox", { name: "Retain" }),
      publish: form.getByRole("button", { name: "Publish", exact: true }),
    };
  }

  /**
   * Publish タブのプリセット一覧の行 (role="button" の div)。行は QoS バッジを持つので、
   * 見出し行の追加ボタンや行内の削除ボタンと区別できる。
   */
  get mqttPresets(): Locator {
    return this.mqttSection("Messages")
      .getByRole("button")
      .filter({ hasText: /QoS [0-2]/ });
  }

  /**
   * トピックが topic のプリセット行。選択中の行は名前を入力欄に出す (テキストにならない) ので、
   * 名前ではなくトピックで引く。
   */
  mqttPreset(topic: string): Locator {
    return this.mqttPresets.filter({
      has: this.page.getByText(topic, { exact: true }),
    });
  }

  /** Publish タブのプリセット一覧の追加ボタン。 */
  get addMqttPresetButton(): Locator {
    return this.mqttSection("Messages").getByRole("button", {
      name: "Add preset",
    });
  }

  /** トピックが topic のプリセット行の削除ボタン。確認は挟まない。 */
  deleteMqttPresetButton(topic: string): Locator {
    return this.mqttPreset(topic).getByRole("button", {
      name: "Delete preset",
    });
  }
}
