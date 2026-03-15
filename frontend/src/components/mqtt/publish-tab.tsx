import { Save, Send, Trash2 } from "lucide-solid";
import { For, Show } from "solid-js";
import { Badge } from "../ui/badge";
import { Button } from "../ui/button";
import { Input } from "../ui/input";
import {
  ResizableHandle,
  ResizablePanel,
  ResizablePanelGroup,
} from "../ui/resizable";
import { ScrollArea } from "../ui/scroll-area";
import { Textarea } from "../ui/textarea";
import { useMqtt } from "./context";
import styles from "./mqtt.module.css";
import { QosSelect } from "./qos-select";

function PresetsPanel() {
  const {
    presetName,
    setPresetName,
    savedPresets,
    savePreset,
    loadPreset,
    removePreset,
  } = useMqtt();

  return (
    <div class={styles.presetsPanel}>
      <div class={styles.sectionHeader}>
        <h3 class={styles.sectionTitle}>Saved</h3>
      </div>

      <div class={styles.savePresetSection}>
        <Input
          value={presetName()}
          onInput={(e) => setPresetName(e.currentTarget.value)}
          placeholder="Preset name"
          class={styles.presetNameInput}
        />
        <Button size="sm" onClick={savePreset} class={styles.savePresetButton}>
          <Save size={16} />
          Save current
        </Button>
      </div>

      <ScrollArea class={styles.subscriptionScrollArea}>
        <div class={styles.listPadding}>
          <Show
            when={savedPresets().length > 0}
            fallback={<p class={styles.emptyText}>No saved presets</p>}
          >
            <div class={styles.itemList}>
              <For each={savedPresets()}>
                {(preset) => (
                  <div class={styles.presetItem}>
                    <button
                      type="button"
                      class={styles.presetItemBody}
                      onClick={() => loadPreset(preset)}
                    >
                      <span class={styles.presetName}>{preset.name}</span>
                      <span class={styles.presetTopic}>{preset.topic}</span>
                      <Badge variant="secondary">QoS {preset.qos}</Badge>
                    </button>
                    <Button
                      variant="ghost"
                      size="icon"
                      onClick={() => removePreset(preset.id)}
                      class={styles.deleteButton}
                    >
                      <Trash2 size={16} />
                    </Button>
                  </div>
                )}
              </For>
            </div>
          </Show>
        </div>
      </ScrollArea>
    </div>
  );
}

function PublishForm() {
  const {
    publishTopic,
    setPublishTopic,
    publishPayload,
    setPublishPayload,
    publishQos,
    setPublishQos,
    publishMessage,
    isConnected,
  } = useMqtt();

  return (
    <div class={styles.publishPanel}>
      <div class={styles.sectionHeader}>
        <h3 class={styles.sectionTitle}>Publish</h3>
      </div>
      <div class={styles.publishForm}>
        <div class={styles.publishTopicRow}>
          <Input
            value={publishTopic()}
            onInput={(e) => setPublishTopic(e.currentTarget.value)}
            placeholder="Topic"
            class={styles.publishTopicInput}
          />
          <QosSelect value={publishQos()} onChange={setPublishQos} />
        </div>

        <Textarea
          value={publishPayload()}
          onInput={(e) => setPublishPayload(e.currentTarget.value)}
          placeholder="Message payload"
          class={styles.publishPayload}
        />

        <Button onClick={publishMessage} disabled={!isConnected()}>
          <Send size={16} />
          Publish
        </Button>
      </div>
    </div>
  );
}

export function PublishTab() {
  return (
    <ResizablePanelGroup direction="horizontal" class={styles.tabContent}>
      <ResizablePanel defaultSize={30} minSize={20}>
        <PresetsPanel />
      </ResizablePanel>
      <ResizableHandle withHandle />
      <ResizablePanel defaultSize={70} minSize={40}>
        <PublishForm />
      </ResizablePanel>
    </ResizablePanelGroup>
  );
}
