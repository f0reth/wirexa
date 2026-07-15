import {
  createContext,
  createSignal,
  type JSX,
  onCleanup,
  onMount,
  Show,
  useContext,
} from "solid-js";
import { Portal } from "solid-js/web";
import {
  createEditorState,
  type EditorState,
} from "../../application/openapi/editor";
import {
  createFilesState,
  type FilesState,
} from "../../application/openapi/files";
import { runGuarded } from "../../application/ui/guard";
import { notify } from "../../application/ui/notifications";
import { ConfirmDialog } from "../../components/ui/confirm-dialog";
import type { ActiveDoc } from "../../domain/openapi/types";
import { confirmQuit, onBeforeClose } from "../../infrastructure/app/lifecycle";
import {
  openFilePicker,
  readFile,
  saveFileAs,
  writeFile,
} from "../../infrastructure/openapi/file-io";
import { parseSpec } from "../../infrastructure/openapi/parser";
import { errorMessage } from "../../shared/error";

interface OpenApiFilesContextValue extends FilesState {
  openFile: () => Promise<void>;
  selectFile: (path: string) => Promise<void>;
  saveActiveFile: () => Promise<boolean>;
  newUntitled: () => Promise<void>;
  openDropped: (content: string, name: string) => Promise<void>;
}

interface OpenApiEditorContextValue extends EditorState {
  onContentChange: (text: string) => void;
}

const OpenApiFilesContext = createContext<OpenApiFilesContextValue>();
const OpenApiEditorContext = createContext<OpenApiEditorContextValue>();

function basename(path: string): string {
  return path.split(/[\\/]/).pop() ?? path;
}

type ConfirmChoice = "save" | "discard" | "cancel";

export function OpenApiProvider(props: { children: JSX.Element }) {
  const filesState = createFilesState();
  const editorState = createEditorState();

  // 未保存確認ダイアログの状態。resolve でユーザーの選択を返す。
  const [pendingConfirm, setPendingConfirm] = createSignal<{
    resolve: (choice: ConfirmChoice) => void;
  } | null>(null);

  onMount(() => {
    filesState.refreshRecents().catch((err) => {
      console.error("Failed to load recent files:", err);
    });

    // ウィンドウを閉じようとしたとき、未保存文書があれば確認する。
    const unsubscribe = onBeforeClose(() => {
      void handleBeforeClose();
    });
    onCleanup(unsubscribe);
  });

  // hasUnsavedChanges は「今切替える/閉じると内容が失われるか」を判定する。
  function hasUnsavedChanges(): boolean {
    const doc = filesState.activeDoc();
    if (!doc) return false;
    // 無題文書はメモリ上にしか存在しないため、内容が非空なら常に失われる。
    if (doc.kind === "untitled")
      return editorState.editorContent().trim().length > 0;
    return editorState.isDirty();
  }

  function confirmUnsaved(): Promise<ConfirmChoice> {
    return new Promise((resolve) => setPendingConfirm({ resolve }));
  }

  function resolveConfirm(choice: ConfirmChoice): void {
    const p = pendingConfirm();
    setPendingConfirm(null);
    p?.resolve(choice);
  }

  // guardSwitch は未保存変更があれば確認を挟んでから切替処理を実行する。
  async function guardSwitch(next: () => Promise<void> | void): Promise<void> {
    if (hasUnsavedChanges()) {
      const choice = await confirmUnsaved();
      if (choice === "cancel") return;
      if (choice === "save") {
        const ok = await saveActiveFile();
        if (!ok) return; // 保存ダイアログのキャンセル or 失敗 → 切替中止
      }
    }
    await next();
  }

  async function doOpenFile(): Promise<void> {
    const path = await openFilePicker();
    if (!path) return;
    const content = await readFile(path);
    await filesState.refreshRecents();
    filesState.setActiveDoc({ kind: "file", path, name: basename(path) });
    editorState.loadContent(content, parseSpec);
  }

  async function openFile(): Promise<void> {
    await runGuarded("Failed to open file", () => guardSwitch(doOpenFile));
  }

  async function doSelectFile(path: string): Promise<void> {
    const content = await readFile(path);
    const file = filesState.files().find((f) => f.path === path);
    filesState.setActiveDoc({
      kind: "file",
      path,
      name: file?.name ?? basename(path),
    });
    editorState.loadContent(content, parseSpec);
  }

  async function selectFile(path: string): Promise<void> {
    await runGuarded("Failed to open file", () =>
      guardSwitch(() => doSelectFile(path)),
    );
  }

  async function saveActiveFile(): Promise<boolean> {
    const doc = filesState.activeDoc();
    if (!doc) return false;
    try {
      if (doc.kind === "file") {
        await writeFile(doc.path, editorState.editorContent());
        editorState.markSaved();
        return true;
      }
      // 無題文書: 初回保存はダイアログを 1 回だけ出す。
      const path = await saveFileAs(doc.name, editorState.editorContent());
      if (!path) return false; // キャンセル
      await filesState.refreshRecents();
      filesState.setActiveDoc({ kind: "file", path, name: basename(path) });
      editorState.markSaved();
      return true;
    } catch (err) {
      notify.error("Failed to save file", errorMessage(err));
      return false;
    }
  }

  async function newUntitled(): Promise<void> {
    await guardSwitch(() => {
      filesState.setActiveDoc({ kind: "untitled", name: "untitled.yaml" });
      editorState.loadContent("", parseSpec);
    });
  }

  async function openDropped(content: string, name: string): Promise<void> {
    await guardSwitch(() => {
      filesState.setActiveDoc({ kind: "untitled", name });
      editorState.loadContent(content, parseSpec);
    });
  }

  // handleBeforeClose はウィンドウを閉じる要求に対し、未保存があれば確認する。
  async function handleBeforeClose(): Promise<void> {
    if (!hasUnsavedChanges()) {
      await confirmQuit();
      return;
    }
    const choice = await confirmUnsaved();
    if (choice === "cancel") return; // ウィンドウを維持
    if (choice === "save") {
      const ok = await saveActiveFile();
      if (!ok) return; // 保存できなければ閉じない
    }
    await confirmQuit();
  }

  function onContentChange(text: string): void {
    editorState.updateContent(text, parseSpec);
  }

  const confirmDocName = () => filesState.activeDoc()?.name ?? "this document";

  return (
    <OpenApiFilesContext.Provider
      value={{
        ...filesState,
        openFile,
        selectFile,
        saveActiveFile,
        newUntitled,
        openDropped,
      }}
    >
      <OpenApiEditorContext.Provider
        value={{ ...editorState, onContentChange }}
      >
        {props.children}
        <Show when={pendingConfirm()}>
          <Portal>
            <ConfirmDialog
              title="Unsaved changes"
              message={`You have unsaved changes in "${confirmDocName()}". What would you like to do?`}
              confirmLabel="Save & continue"
              variant="default"
              secondaryLabel="Discard changes"
              secondaryVariant="destructive"
              onConfirm={() => resolveConfirm("save")}
              onSecondary={() => resolveConfirm("discard")}
              onCancel={() => resolveConfirm("cancel")}
            />
          </Portal>
        </Show>
      </OpenApiEditorContext.Provider>
    </OpenApiFilesContext.Provider>
  );
}

export function useOpenApiFiles(): OpenApiFilesContextValue {
  const ctx = useContext(OpenApiFilesContext);
  if (!ctx)
    throw new Error("useOpenApiFiles must be used within OpenApiProvider");
  return ctx;
}

export function useOpenApiEditor(): OpenApiEditorContextValue {
  const ctx = useContext(OpenApiEditorContext);
  if (!ctx)
    throw new Error("useOpenApiEditor must be used within OpenApiProvider");
  return ctx;
}

export type { ActiveDoc };
