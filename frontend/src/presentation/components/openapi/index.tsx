import { clsx } from "clsx";
import { ChevronLeft, ChevronRight, Save } from "lucide-solid";
import { onMount, Show } from "solid-js";
import { runGuarded } from "../../../application/ui/guard";
import {
  ResizableHandle,
  ResizablePanel,
  ResizablePanelGroup,
} from "../../../components/ui/resizable";
import {
  useOpenApiEditor,
  useOpenApiFiles,
} from "../../providers/openapi-provider";
import { EditorPanel } from "./editor-panel";
import styles from "./openapi.module.css";
import { PreviewPanel } from "./preview-panel";

export function OpenApiClient() {
  const editorCtx = useOpenApiEditor();
  const filesCtx = useOpenApiFiles();

  // Ctrl+S でアクティブファイルを保存
  onMount(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && e.key === "s") {
        e.preventDefault();
        filesCtx.saveActiveFile();
      }
    };
    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  });

  const activeDoc = () => filesCtx.activeDoc();
  const displayLabel = () => {
    const doc = activeDoc();
    if (!doc) return "No file opened — use the sidebar to open or paste a spec";
    const base = doc.kind === "file" ? doc.path : `${doc.name} (untitled)`;
    return editorCtx.isDirty() ? `${base} *` : base;
  };

  // OpenAPI ファイルの D&D: HTML5 drop で内容を直接読み込み無題文書として開く。
  // Wails ネイティブ file-drop は使わない（wails:file-drop は JS から偽装可能なため）。
  const handleDragOver = (e: DragEvent) => {
    if (e.dataTransfer?.types.includes("Files")) {
      e.preventDefault();
      e.dataTransfer.dropEffect = "copy";
    }
  };
  const handleDrop = async (e: DragEvent) => {
    const file = e.dataTransfer?.files?.[0];
    if (!file) return;
    e.preventDefault();
    await runGuarded("Failed to open dropped file", async () => {
      const content = await file.text();
      await filesCtx.openDropped(content, file.name);
    });
  };

  return (
    // biome-ignore lint/a11y/noStaticElementInteractions: file drop target
    <div
      class={styles.container}
      onDragOver={handleDragOver}
      onDrop={handleDrop}
    >
      {/* Top bar */}
      <div class={styles.topBar}>
        <span
          class={clsx(styles.fileName, activeDoc() && styles.fileNameActive)}
        >
          {displayLabel()}
        </span>
        <Show when={activeDoc()}>
          <button
            type="button"
            class={styles.toggleBtn}
            onClick={() => filesCtx.saveActiveFile()}
            title="Save (Ctrl+S)"
          >
            <Save size={16} />
          </button>
        </Show>
        <button
          type="button"
          class={clsx(
            styles.toggleBtn,
            editorCtx.isPreviewing() && styles.toggleBtnActive,
          )}
          onClick={() => editorCtx.togglePreview()}
          title={editorCtx.isPreviewing() ? "Hide preview" : "Show preview"}
        >
          {editorCtx.isPreviewing() ? (
            <ChevronRight size={16} />
          ) : (
            <ChevronLeft size={16} />
          )}
        </button>
      </div>

      {/* Main content */}
      <div class={styles.mainContent}>
        <Show
          when={editorCtx.isPreviewing()}
          fallback={
            <div class={styles.editorFullHeight}>
              <EditorPanel />
            </div>
          }
        >
          <ResizablePanelGroup direction="horizontal">
            <ResizablePanel defaultSize={50} minSize={20}>
              <EditorPanel />
            </ResizablePanel>
            <ResizableHandle withHandle />
            <ResizablePanel defaultSize={50} minSize={20}>
              <PreviewPanel />
            </ResizablePanel>
          </ResizablePanelGroup>
        </Show>
      </div>
    </div>
  );
}
