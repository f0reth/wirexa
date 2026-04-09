import { createSignal, Show } from "solid-js";
import {
  ResizableHandle,
  ResizablePanel,
  ResizablePanelGroup,
} from "../../../components/ui/resizable";
import styles from "./http.module.css";
import { RequestBar } from "./request-bar";
import { RequestEditor } from "./request-editor";
import { ResponseViewer } from "./response-viewer";

export function HttpClient() {
  const [showResponse, setShowResponse] = createSignal(true);

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
