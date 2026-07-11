import { createEffect, onCleanup, onMount, Show } from "solid-js";
import { SwaggerUIBundle } from "swagger-ui-dist";
import "swagger-ui-dist/swagger-ui.css";
import { useOpenApiEditor } from "../../providers/openapi-provider";
import styles from "./openapi.module.css";
import "./swagger-ui-dark.css";

function SwaggerPreview(props: { spec: object }) {
  let containerRef: HTMLDivElement | undefined;

  onMount(() => {
    const ui = SwaggerUIBundle({
      domNode: containerRef,
      spec: props.spec,
      presets: [SwaggerUIBundle.presets.apis], // no Standalone topbar
      supportedSubmitMethods: [], // read-only preview: no "Try it out"
      tryItOutEnabled: false,
      docExpansion: "list",
    });

    // Reflect spec edits into the rendered reference.
    createEffect(() => {
      ui.specActions.updateSpec(JSON.stringify(props.spec));
    });

    // swagger-ui has no public destroy(); Solid removes the parent div on
    // unmount, so just detach the rendered subtree to free its React root.
    onCleanup(() => containerRef?.replaceChildren());
  });

  return (
    <div
      ref={containerRef}
      data-testid="openapi-preview"
      class={styles.swaggerWrap}
    />
  );
}

export function PreviewPanel() {
  const editorCtx = useOpenApiEditor();

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
        {(spec) => <SwaggerPreview spec={spec()} />}
      </Show>
    </div>
  );
}
