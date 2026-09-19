import { clsx } from "clsx";
import { GripVertical, Plus, Send, Trash2 } from "lucide-solid";
import { createSignal, For, Show } from "solid-js";
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
    selectPreset,
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
                          if (!isSelected()) selectPreset(preset.id);
                        }}
                        onKeyDown={(e) => {
                          if (
                            (e.key === "Enter" || e.key === " ") &&
                            !isSelected()
                          )
                            selectPreset(preset.id);
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

function PublishForm() {
  const { activeConnection } = useMqttConnection();
  const { publish, draft, updateDraft } = useMqttPublish();
  const isConnected = () => {
    const conn = activeConnection();
    return conn?.type === "online" && conn.connected;
  };

  const handlePublish = async () => {
    const { topic, payload, qos, retain } = draft();
    if (!topic.trim()) return;
    // retain 付きの空ペイロードは retained メッセージの削除を意味するので許可する。
    if (!retain && !payload.trim()) return;
    await publish(topic, payload, qos, retain);
  };

  return (
    <div class={styles.publishPanel}>
      <div class={base.sectionHeader}>
        <h3 class={base.sectionTitle}>Publish</h3>
      </div>
      <div class={styles.publishForm}>
        <div class={styles.publishTopicRow}>
          <Input
            value={draft().topic}
            onInput={(e) => updateDraft({ topic: e.currentTarget.value })}
            placeholder="Topic"
            class={styles.publishTopicInput}
          />
          <QosSelect
            value={draft().qos}
            onChange={(qos) => updateDraft({ qos: qos as 0 | 1 | 2 })}
          />
          <label class={styles.retainCheckboxLabel} for="publish-retain">
            <input
              id="publish-retain"
              type="checkbox"
              checked={draft().retain}
              onChange={(e) => updateDraft({ retain: e.currentTarget.checked })}
            />
            Retain
          </label>
        </div>

        <Textarea
          value={draft().payload}
          onInput={(e) => updateDraft({ payload: e.currentTarget.value })}
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
  const { addPreset } = useMqttPublish();

  return (
    <ResizablePanelGroup direction="horizontal" class={base.tabContent}>
      <ResizablePanel defaultSize={30} minSize={20}>
        <PresetsPanel addPreset={addPreset} />
      </ResizablePanel>
      <ResizableHandle withHandle />
      <ResizablePanel defaultSize={70} minSize={40}>
        <PublishForm />
      </ResizablePanel>
    </ResizablePanelGroup>
  );
}
