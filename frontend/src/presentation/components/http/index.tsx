import { createEffect, createSignal, Show } from "solid-js";
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
  const { loading } = useHttpRequest();
  const [showResponse, setShowResponse] = createSignal(true);
  const [savedResponseSize, setSavedResponseSize] = createSignal(0.5);

  createEffect(() => {
    if (loading()) {
      setShowResponse(true);
    }
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
            <ResizablePanel
              defaultSize={(1 - savedResponseSize()) * 100}
              minSize={20}
            >
              <RequestEditor />
            </ResizablePanel>
            <ResizableHandle withHandle />
            <ResizablePanel
              defaultSize={savedResponseSize() * 100}
              minSize={20}
              onResize={setSavedResponseSize}
            >
              <ResponseViewer />
            </ResizablePanel>
          </ResizablePanelGroup>
        </Show>
      </div>
    </div>
  );
}
