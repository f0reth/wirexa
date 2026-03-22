import { Save, Send, Trash2 } from "lucide-solid";
import { createSignal, For, Show } from "solid-js";
import { Badge } from "../../../components/ui/badge";
import { Button } from "../../../components/ui/button";
import { Input } from "../../../components/ui/input";
import {
  ResizableHandle,
  ResizablePanel,
  ResizablePanelGroup,
} from "../../../components/ui/resizable";
import { ScrollArea } from "../../../components/ui/scroll-area";
import { Textarea } from "../../../components/ui/textarea";
import type { PublishPreset } from "../../../domain/mqtt/types";
import * as client from "../../../infrastructure/mqtt/client";
import {
  useMqttConnection,
  useMqttPublish,
} from "../../providers/mqtt-provider";
import styles from "./mqtt.module.css";
import { QosSelect } from "./qos-select";

function PresetsPanel(props: {
  publishTopic: () => string;
  publishPayload: () => string;
  publishQos: () => number;
  onLoadPreset: (preset: PublishPreset) => void;
}) {
  const { presets, savePreset, removePreset } = useMqttPublish();
  const [presetName, setPresetName] = createSignal("");

  const handleSavePreset = () => {
    if (!presetName().trim() || !props.publishTopic().trim()) return;
    savePreset({
      name: presetName().trim(),
      topic: props.publishTopic(),
      payload: props.publishPayload(),
      qos: props.publishQos() as 0 | 1 | 2,
    });
    setPresetName("");
  };

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
        <Button
          size="sm"
          onClick={handleSavePreset}
          class={styles.savePresetButton}
        >
          <Save size={16} />
          Save current
        </Button>
      </div>

      <ScrollArea class={styles.subscriptionScrollArea}>
        <div class={styles.listPadding}>
          <Show
            when={presets().length > 0}
            fallback={<p class={styles.emptyText}>No saved presets</p>}
          >
            <div class={styles.itemList}>
              <For each={presets()}>
                {(preset) => (
                  <div class={styles.presetItem}>
                    <button
                      type="button"
                      class={styles.presetItemBody}
                      onClick={() => props.onLoadPreset(preset)}
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

function PublishForm(props: {
  publishTopic: () => string;
  setPublishTopic: (v: string) => void;
  publishPayload: () => string;
  setPublishPayload: (v: string) => void;
  publishQos: () => number;
  setPublishQos: (v: number) => void;
}) {
  const { activeConnection, activeConnectionId } = useMqttConnection();
  const isConnected = () => activeConnection()?.connected ?? false;

  const handlePublish = async () => {
    const connId = activeConnectionId();
    if (
      !connId ||
      !props.publishTopic().trim() ||
      !props.publishPayload().trim()
    )
      return;
    try {
      await client.publish(
        connId,
        props.publishTopic(),
        props.publishPayload(),
        props.publishQos(),
        false,
      );
    } catch (err) {
      console.error("[MQTT] Publish failed:", err);
    }
  };

  return (
    <div class={styles.publishPanel}>
      <div class={styles.sectionHeader}>
        <h3 class={styles.sectionTitle}>Publish</h3>
      </div>
      <div class={styles.publishForm}>
        <div class={styles.publishTopicRow}>
          <Input
            value={props.publishTopic()}
            onInput={(e) => props.setPublishTopic(e.currentTarget.value)}
            placeholder="Topic"
            class={styles.publishTopicInput}
          />
          <QosSelect
            value={props.publishQos()}
            onChange={props.setPublishQos}
          />
        </div>

        <Textarea
          value={props.publishPayload()}
          onInput={(e) => props.setPublishPayload(e.currentTarget.value)}
          placeholder="Message payload"
          class={styles.publishPayload}
        />

        <Button onClick={handlePublish} disabled={!isConnected()}>
          <Send size={16} />
          Publish
        </Button>
      </div>
    </div>
  );
}

export function PublishTab() {
  const [publishTopic, setPublishTopic] = createSignal("");
  const [publishPayload, setPublishPayload] = createSignal("");
  const [publishQos, setPublishQos] = createSignal<number>(0);

  const handleLoadPreset = (preset: PublishPreset) => {
    setPublishTopic(preset.topic);
    setPublishPayload(preset.payload);
    setPublishQos(preset.qos);
  };

  return (
    <ResizablePanelGroup direction="horizontal" class={styles.tabContent}>
      <ResizablePanel defaultSize={30} minSize={20}>
        <PresetsPanel
          publishTopic={publishTopic}
          publishPayload={publishPayload}
          publishQos={publishQos}
          onLoadPreset={handleLoadPreset}
        />
      </ResizablePanel>
      <ResizableHandle withHandle />
      <ResizablePanel defaultSize={70} minSize={40}>
        <PublishForm
          publishTopic={publishTopic}
          setPublishTopic={setPublishTopic}
          publishPayload={publishPayload}
          setPublishPayload={setPublishPayload}
          publishQos={publishQos}
          setPublishQos={setPublishQos}
        />
      </ResizablePanel>
    </ResizablePanelGroup>
  );
}
