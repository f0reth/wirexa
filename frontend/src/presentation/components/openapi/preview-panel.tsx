import "rapidoc";
import { createEffect, onMount, Show } from "solid-js";
import { useOpenApiEditor } from "../../providers/openapi-provider";
import styles from "./openapi.module.css";

interface RapiDocElement extends HTMLElement {
  loadSpec(spec: object): void;
}

// RapiDoc の Web Component 型拡張
declare module "solid-js" {
  namespace JSX {
    interface IntrinsicElements {
      "rapi-doc": {
        ref?: RapiDocElement | ((el: RapiDocElement) => void);
        id?: string;
        "render-style"?: string;
        theme?: string;
        "allow-try"?: string;
        "show-header"?: string;
        "primary-color"?: string;
        "bg-color"?: string;
        "text-color"?: string;
      };
    }
  }
}

export function PreviewPanel() {
  let rapiDocRef: RapiDocElement | undefined;
  const editorCtx = useOpenApiEditor();

  onMount(() => {
    const spec = editorCtx.parsedSpec();
    if (spec && rapiDocRef) {
      rapiDocRef.loadSpec(spec);
    }
  });

  // parsedSpec が更新されるたびに RapiDoc へ反映
  createEffect(() => {
    const spec = editorCtx.parsedSpec();
    if (!rapiDocRef || !spec) return;
    rapiDocRef.loadSpec(spec);
  });

  return (
    <div class={styles.previewPanel}>
      <Show
        when={editorCtx.parsedSpec()}
        fallback={
          <div class={styles.previewEmpty}>
            <span>No valid OpenAPI spec</span>
          </div>
        }
      >
        <div class={styles.rapiDocWrap}>
          <rapi-doc
            ref={rapiDocRef}
            render-style="read"
            theme="dark"
            allow-try="false"
            show-header="false"
            primary-color="#6366f1"
          />
        </div>
      </Show>
    </div>
  );
}
