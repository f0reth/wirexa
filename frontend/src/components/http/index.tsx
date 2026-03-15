import {
  ResizableHandle,
  ResizablePanel,
  ResizablePanelGroup,
} from "../ui/resizable";
import styles from "./http.module.css";
import { RequestBar } from "./request-bar";
import { RequestEditor } from "./request-editor";
import { ResponseViewer } from "./response-viewer";

export function HttpClient() {
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
