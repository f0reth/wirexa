import { createSignal, onCleanup, onMount, Show } from "solid-js";
import {
  ResizableHandle,
  ResizablePanel,
  ResizablePanelGroup,
} from "../../../components/ui/resizable";
import { useHttpRequest } from "../../providers/http-provider";
import styles from "./http.module.css";
import { RequestBar } from "./request-bar";
import { RequestEditor } from "./request-editor";
import { ResponseViewer } from "./response-viewer";

export function HttpClient() {
  const { dirty, saveCurrentRequest } = useHttpRequest();
  const [showResponse, setShowResponse] = createSignal(true);

  onMount(() => {
    const handler = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && e.key === "s" && dirty()) {
        e.preventDefault();
        saveCurrentRequest().catch((err) => {
          console.error("Failed to save request:", err);
        });
      }
    };
    document.addEventListener("keydown", handler);
    onCleanup(() => document.removeEventListener("keydown", handler));
  });

  return (
    <div class={styles.container}>
      <RequestBar
        showResponse={showResponse()}
        onToggleResponse={() => setShowResponse((v) => !v)}
      />
      <div class={styles.mainContent}>
        <Show
          when={showResponse()}
          fallback={
            <div class={styles.editorFullHeight}>
              <RequestEditor />
            </div>
          }
        >
          <ResizablePanelGroup direction="horizontal">
            <ResizablePanel defaultSize={50} minSize={20}>
              <RequestEditor />
            </ResizablePanel>
            <ResizableHandle withHandle />
            <ResizablePanel defaultSize={50} minSize={20}>
              <ResponseViewer onClose={() => setShowResponse(false)} />
            </ResizablePanel>
          </ResizablePanelGroup>
        </Show>
      </div>
    </div>
  );
}
