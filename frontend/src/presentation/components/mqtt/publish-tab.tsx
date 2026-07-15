import { clsx } from "clsx";
import { GripVertical, Plus, Send, Trash2 } from "lucide-solid";
import { batch, createEffect, createSignal, For, on, Show } from "solid-js";
import { runGuarded } from "../../../application/ui/guard";
import { Badge } from "../../../components/ui/badge";
import { Button } from "../../../components/ui/button";
import { Input } from "../../../components/ui/input";
import {
  createListReorder,
  InsertionZone,
} from "../../../components/ui/list-reorder";
import reorderStyles from "../../../components/ui/list-reorder.module.css";
import {
  ResizableHandle,
  ResizablePanel,
  ResizablePanelGroup,
} from "../../../components/ui/resizable";
import { ScrollArea } from "../../../components/ui/scroll-area";
import { Textarea } from "../../../components/ui/textarea";
import {
  useMqttConnection,
  useMqttPublish,
} from "../../providers/mqtt-provider";
import base from "./mqtt.module.css";
import styles from "./publish.module.css";
import { QosSelect } from "./qos-select";

function PresetsPanel(props: { addPreset: () => void }) {
  const {
    presets,
    removePreset,
    updatePreset,
    reorderPresets,
    selectedPresetId,
    setSelectedPresetId,
  } = useMqttPublish();

  const { draggingIndex, dropIndicatorIndex, itemHandlers } =
    createListReorder(reorderPresets);

  return (
    <div class={styles.presetsPanel}>
      <div class={base.sectionHeader}>
        <h3 class={base.sectionTitle}>Messages</h3>
        <Button variant="ghost" size="icon" onClick={() => props.addPreset()}>
          <Plus size={16} />
        </Button>
      </div>

      <ScrollArea class={base.subscriptionScrollArea}>
        <div class={base.listPadding}>
          <Show
            when={presets().length > 0}
            fallback={<p class={base.emptyText}>No saved presets</p>}
          >
            <div class={base.itemList}>
              <For each={presets()}>
                {(preset, index) => {
                  const isSelected = () => selectedPresetId() === preset.id;
                  const [editingName, setEditingName] = createSignal(
                    preset.name,
                  );

                  const commitName = () => {
                    const name = editingName().trim();
                    if (name && name !== preset.name) {
                      updatePreset(preset.id, { name });
                    }
                  };

                  return (
                    <>
                      <InsertionZone
                        visible={draggingIndex() !== null}
                        active={dropIndicatorIndex() === index()}
                      />
                      {/* biome-ignore lint/a11y/useSemanticElements: contains nested interactive elements (delete button, name input); button cannot contain button */}
                      <div
                        class={clsx(
                          styles.presetItem,
                          isSelected() && styles.presetItemSelected,
                          draggingIndex() === index() &&
                            styles.presetItemDragging,
                        )}
                        role="button"
                        tabIndex={0}
                        {...itemHandlers(index)}
                        onClick={() => {
                          if (draggingIndex() !== null) return;
                          if (!isSelected()) setSelectedPresetId(preset.id);
                        }}
                        onKeyDown={(e) => {
                          if (
                            (e.key === "Enter" || e.key === " ") &&
                            !isSelected()
                          )
                            setSelectedPresetId(preset.id);
                        }}
                      >
                        <GripVertical
                          size={14}
                          class={reorderStyles.dragHandle}
                        />
                        <div class={styles.presetItemBody}>
                          <Show
                            when={isSelected()}
                            fallback={
                              <span class={styles.presetName}>
                                {preset.name}
                              </span>
                            }
                          >
                            <Input
                              value={editingName()}
                              onInput={(e) =>
                                setEditingName(e.currentTarget.value)
                              }
                              onBlur={commitName}
                              onKeyDown={(e) => {
                                if (e.key === "Enter") {
                                  e.preventDefault();
                                  e.currentTarget.blur();
                                } else if (e.key === "Escape") {
                                  e.preventDefault();
                                  setEditingName(preset.name);
                                  e.currentTarget.blur();
                                }
                              }}
                              onClick={(e) => e.stopPropagation()}
                              class={styles.presetNameInlineInput}
                            />
                          </Show>
                          <span class={styles.presetTopic}>{preset.topic}</span>
                          <Badge variant="secondary">QoS {preset.qos}</Badge>
                          <Show when={preset.retain}>
                            <Badge variant="outline">Retained</Badge>
                          </Show>
                        </div>
                        <Button
                          variant="ghost"
                          size="icon"
                          onClick={(e) => {
                            e.stopPropagation();
                            removePreset(preset.id);
                          }}
                          class={base.deleteButton}
                        >
                          <Trash2 size={16} />
                        </Button>
                      </div>
                    </>
                  );
                }}
              </For>
              <InsertionZone
                visible={draggingIndex() !== null}
                active={dropIndicatorIndex() === presets().length}
              />
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
  publishRetain: () => boolean;
  setPublishRetain: (v: boolean) => void;
}) {
  const { activeConnection } = useMqttConnection();
  const { publish } = useMqttPublish();
  const isConnected = () => {
    const conn = activeConnection();
    return conn?.type === "online" && conn.connected;
  };

  const handlePublish = async () => {
    if (!props.publishTopic().trim()) return;
    // retain 付きの空ペイロードは retained メッセージの削除を意味するので許可する。
    if (!props.publishRetain() && !props.publishPayload().trim()) return;
    await runGuarded("Failed to publish message", () =>
      publish(
        props.publishTopic(),
        props.publishPayload(),
        props.publishQos(),
        props.publishRetain(),
      ),
    );
  };

  return (
    <div class={styles.publishPanel}>
      <div class={base.sectionHeader}>
        <h3 class={base.sectionTitle}>Publish</h3>
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
          <label class={styles.retainCheckboxLabel} for="publish-retain">
            <input
              id="publish-retain"
              type="checkbox"
              checked={props.publishRetain()}
              onChange={(e) => props.setPublishRetain(e.currentTarget.checked)}
            />
            Retain
          </label>
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
  const [publishRetain, setPublishRetain] = createSignal(false);

  const { presets, addPreset, updatePreset, selectedPresetId } =
    useMqttPublish();

  createEffect(
    on(
      selectedPresetId,
      (id) => {
        if (!id) return;
        const preset = presets().find((p) => p.id === id);
        if (!preset) return;
        batch(() => {
          setPublishTopic(preset.topic);
          setPublishPayload(preset.payload);
          setPublishQos(preset.qos);
          setPublishRetain(preset.retain);
        });
      },
      { defer: true },
    ),
  );

  createEffect(
    on(
      [publishTopic, publishPayload, publishQos, publishRetain],
      ([topic, payload, qos, retain]) => {
        const id = selectedPresetId();
        if (!id) return;
        const current = presets().find((p) => p.id === id);
        if (
          current &&
          current.topic === topic &&
          current.payload === payload &&
          current.qos === qos &&
          current.retain === retain
        )
          return;
        updatePreset(id, { topic, payload, qos: qos as 0 | 1 | 2, retain });
      },
      { defer: true },
    ),
  );

  return (
    <ResizablePanelGroup direction="horizontal" class={base.tabContent}>
      <ResizablePanel defaultSize={30} minSize={20}>
        <PresetsPanel addPreset={addPreset} />
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
          publishRetain={publishRetain}
          setPublishRetain={setPublishRetain}
        />
      </ResizablePanel>
    </ResizablePanelGroup>
  );
}
