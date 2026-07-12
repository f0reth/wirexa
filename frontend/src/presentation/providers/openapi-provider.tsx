import { createContext, type JSX, onMount, useContext } from "solid-js";
import {
  createEditorState,
  type EditorState,
} from "../../application/openapi/editor";
import {
  createFilesState,
  type FilesState,
} from "../../application/openapi/files";
import type { ActiveDoc } from "../../domain/openapi/types";
import {
  openFilePicker,
  readFile,
  saveFileAs,
  writeFile,
} from "../../infrastructure/openapi/file-io";
import { parseSpec } from "../../infrastructure/openapi/parser";

interface OpenApiFilesContextValue extends FilesState {
  openFile: () => Promise<void>;
  selectFile: (path: string) => Promise<void>;
  saveActiveFile: () => Promise<void>;
  newUntitled: () => void;
  openDropped: (content: string, name: string) => void;
}

interface OpenApiEditorContextValue extends EditorState {
  onContentChange: (text: string) => void;
}

const OpenApiFilesContext = createContext<OpenApiFilesContextValue>();
const OpenApiEditorContext = createContext<OpenApiEditorContextValue>();

function basename(path: string): string {
  return path.split(/[\\/]/).pop() ?? path;
}

export function OpenApiProvider(props: { children: JSX.Element }) {
  const filesState = createFilesState();
  const editorState = createEditorState();

  onMount(() => {
    filesState.refreshRecents().catch((err) => {
      console.error("Failed to load recent files:", err);
    });
  });

  async function openFile(): Promise<void> {
    try {
      const path = await openFilePicker();
      if (!path) return;
      const content = await readFile(path);
      await filesState.refreshRecents();
      filesState.setActiveDoc({ kind: "file", path, name: basename(path) });
      editorState.loadContent(content, parseSpec);
    } catch (err) {
      console.error("Failed to open file:", err);
    }
  }

  async function selectFile(path: string): Promise<void> {
    try {
      const content = await readFile(path);
      const file = filesState.files().find((f) => f.path === path);
      filesState.setActiveDoc({
        kind: "file",
        path,
        name: file?.name ?? basename(path),
      });
      editorState.loadContent(content, parseSpec);
    } catch (err) {
      console.error("Failed to read file:", err);
    }
  }

  async function saveActiveFile(): Promise<void> {
    const doc = filesState.activeDoc();
    if (!doc) return;
    try {
      if (doc.kind === "file") {
        await writeFile(doc.path, editorState.editorContent());
        return;
      }
      // 無題文書: 初回保存はダイアログを 1 回だけ出す。
      const path = await saveFileAs(doc.name, editorState.editorContent());
      if (!path) return; // キャンセル
      await filesState.refreshRecents();
      filesState.setActiveDoc({ kind: "file", path, name: basename(path) });
    } catch (err) {
      console.error("Failed to save file:", err);
    }
  }

  function newUntitled(): void {
    filesState.setActiveDoc({ kind: "untitled", name: "untitled.yaml" });
    editorState.loadContent("", parseSpec);
  }

  function openDropped(content: string, name: string): void {
    filesState.setActiveDoc({ kind: "untitled", name });
    editorState.loadContent(content, parseSpec);
  }

  function onContentChange(text: string): void {
    editorState.updateContent(text, parseSpec);
  }

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
