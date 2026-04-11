import { clsx } from "clsx";
import { ChevronLeft, ChevronRight, Save } from "lucide-solid";
import { onMount, Show } from "solid-js";
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

  const activeFile = () => filesCtx.getActiveFile();

  return (
    <div class={styles.container}>
      {/* Top bar */}
      <div class={styles.topBar}>
        <span
          class={clsx(styles.fileName, activeFile() && styles.fileNameActive)}
        >
          {activeFile()?.path ??
            "No file opened — use the sidebar to open a file"}
        </span>
        <Show when={activeFile()}>
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
