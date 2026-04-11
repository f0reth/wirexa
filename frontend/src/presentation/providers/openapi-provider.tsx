import { createContext, type JSX, useContext } from "solid-js";
import {
  createEditorState,
  type EditorState,
} from "../../application/openapi/editor";
import {
  createFilesState,
  type FilesState,
} from "../../application/openapi/files";
import {
  openFilePicker,
  readFile,
  writeFile,
} from "../../infrastructure/openapi/file-io";
import { parseSpec } from "../../infrastructure/openapi/parser";

interface OpenApiFilesContextValue extends FilesState {
  openFile: () => Promise<void>;
  selectFile: (id: string) => Promise<void>;
  saveActiveFile: () => Promise<void>;
}

interface OpenApiEditorContextValue extends EditorState {
  onContentChange: (text: string) => void;
}

const OpenApiFilesContext = createContext<OpenApiFilesContextValue>();
const OpenApiEditorContext = createContext<OpenApiEditorContextValue>();

export function OpenApiProvider(props: { children: JSX.Element }) {
  const filesState = createFilesState();
  const editorState = createEditorState();

  async function openFile(): Promise<void> {
    try {
      const path = await openFilePicker();
      if (!path) return;
      const name = path.split(/[\\/]/).pop() ?? path;
      const content = await readFile(path);
      filesState.addFile(path, name);
      editorState.loadContent(content, parseSpec);
    } catch (err) {
      console.error("Failed to open file:", err);
    }
  }

  async function selectFile(id: string): Promise<void> {
    const file = filesState.files().find((f) => f.id === id);
    if (!file) return;
    try {
      const content = await readFile(file.path);
      filesState.setActiveFileId(id);
      editorState.loadContent(content, parseSpec);
    } catch (err) {
      console.error("Failed to read file:", err);
    }
  }

  async function saveActiveFile(): Promise<void> {
    const file = filesState.getActiveFile();
    if (!file) return;
    try {
      await writeFile(file.path, editorState.editorContent());
    } catch (err) {
      console.error("Failed to save file:", err);
    }
  }

  function onContentChange(text: string): void {
    editorState.updateContent(text, parseSpec);
  }

  return (
    <OpenApiFilesContext.Provider
      value={{ ...filesState, openFile, selectFile, saveActiveFile }}
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
