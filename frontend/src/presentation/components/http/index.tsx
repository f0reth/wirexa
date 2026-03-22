import { onCleanup, onMount } from "solid-js";
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
      <RequestBar />
      <ResizablePanelGroup direction="vertical">
        <ResizablePanel defaultSize={50} minSize={20}>
          <RequestEditor />
        </ResizablePanel>
        <ResizableHandle withHandle />
        <ResizablePanel defaultSize={50} minSize={20}>
          <ResponseViewer />
        </ResizablePanel>
      </ResizablePanelGroup>
    </div>
  );
}
