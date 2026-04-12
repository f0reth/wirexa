import { clsx } from "clsx";
import { ChevronDown, ChevronUp, Plus, Settings, Trash2 } from "lucide-solid";
import { createSignal, For, onMount, Show } from "solid-js";
import { Portal } from "solid-js/web";
import { Button } from "../../../components/ui/button";
import { ConfirmDialog } from "../../../components/ui/confirm-dialog";
import { Input } from "../../../components/ui/input";
import { ScrollArea } from "../../../components/ui/scroll-area";
import type { UdpTarget } from "../../../domain/udp/types";
import { useUdpSend, useUdpTargets } from "../../providers/udp-provider";
import styles from "./sidebar.module.css";

interface TargetFormState {
  id: string;
  name: string;
  host: string;
  port: string;
}

function defaultForm(target?: UdpTarget): TargetFormState {
  return {
    id: target?.id ?? "",
    name: target?.name ?? "",
    host: target?.host ?? "",
    port: target ? String(target.port) : "",
  };
}

interface TargetDialogProps {
  initial?: UdpTarget;
  onSave: (t: UdpTarget) => void;
  onClose: () => void;
}

function TargetDialog(props: TargetDialogProps) {
  const [form, setForm] = createSignal<TargetFormState>(
    defaultForm(props.initial),
  );

  const update = (field: keyof TargetFormState, value: string) =>
    setForm((prev) => ({ ...prev, [field]: value }));

  const handleSave = () => {
    const f = form();
    props.onSave({
      id: f.id,
      name: f.name,
      host: f.host,
      port: Number(f.port),
    });
  };

  return (
    <div class={styles.dialogOverlay}>
      <div class={styles.dialog}>
        <h3 class={styles.dialogTitle}>
          {props.initial ? "Edit Target" : "New Target"}
        </h3>
        <div class={styles.dialogForm}>
          {/* biome-ignore lint/a11y/noLabelWithoutControl: custom Input component is not recognized as a form control */}
          <label class={styles.dialogLabel}>
            Name
            <Input
              value={form().name}
              onInput={(e) => update("name", e.currentTarget.value)}
              placeholder="My Target"
            />
          </label>
          {/* biome-ignore lint/a11y/noLabelWithoutControl: custom Input component is not recognized as a form control */}
          <label class={styles.dialogLabel}>
            Host
            <Input
              value={form().host}
              onInput={(e) => update("host", e.currentTarget.value)}
              placeholder="127.0.0.1"
            />
          </label>
          {/* biome-ignore lint/a11y/noLabelWithoutControl: custom Input component is not recognized as a form control */}
          <label class={styles.dialogLabel}>
            Port
            <Input
              type="number"
              min={1}
              max={65535}
              value={form().port}
              onInput={(e) => update("port", e.currentTarget.value)}
              placeholder="12345"
            />
          </label>
        </div>
        <div class={styles.dialogActions}>
          <Button variant="ghost" onClick={props.onClose}>
            Cancel
          </Button>
          <Button onClick={handleSave}>Save</Button>
        </div>
      </div>
    </div>
  );
}

export function TargetTree() {
  const { targets, saveTarget, deleteTarget, refreshTargets, reorderTargets } =
    useUdpTargets();
  const { loadTarget } = useUdpSend();

  onMount(() => {
    refreshTargets();
  });

  const [editingTarget, setEditingTarget] = createSignal<
    UdpTarget | "new" | null
  >(null);
  const [deletingTarget, setDeletingTarget] = createSignal<{
    id: string;
    name: string;
  } | null>(null);

  const handleSave = async (t: UdpTarget) => {
    await saveTarget(t);
    setEditingTarget(null);
  };

  return (
    <div class={styles.collectionTree}>
      <div class={styles.collectionHeader}>
        <span class={styles.collectionTitle}>Targets</span>
        <Button
          variant="ghost"
          size="icon"
          class={styles.collectionAction}
          onClick={() => setEditingTarget("new")}
          title="New Target"
        >
          <Plus size={14} />
        </Button>
      </div>

      <ScrollArea class={styles.treeScroll}>
        <div class={styles.treeList}>
          <For each={targets}>
            {(target, index) => (
              <>
                {/* biome-ignore lint/a11y/useSemanticElements: contains nested action buttons; cannot use <button> with nested interactive elements */}
                <div
                  role="button"
                  tabIndex={0}
                  class={clsx(styles.brokerItem)}
                  onClick={() => loadTarget(target)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter" || e.key === " ") loadTarget(target);
                  }}
                >
                  <div class={styles.brokerInfo}>
                    <span class={styles.brokerName}>{target.name}</span>
                    <span class={styles.brokerUrl}>
                      {target.host}:{target.port}
                    </span>
                  </div>
                  <div class={styles.treeNodeActions}>
                    <button
                      type="button"
                      class={styles.treeActionBtn}
                      title="Move up"
                      disabled={index() === 0}
                      onClick={(e) => {
                        e.stopPropagation();
                        reorderTargets(index(), index() - 1);
                      }}
                    >
                      <ChevronUp size={12} />
                    </button>
                    <button
                      type="button"
                      class={styles.treeActionBtn}
                      title="Move down"
                      disabled={index() === targets.length - 1}
                      onClick={(e) => {
                        e.stopPropagation();
                        reorderTargets(index(), index() + 1);
                      }}
                    >
                      <ChevronDown size={12} />
                    </button>
                    <button
                      type="button"
                      class={styles.treeActionBtn}
                      title="Edit"
                      onClick={(e) => {
                        e.stopPropagation();
                        setEditingTarget(target);
                      }}
                    >
                      <Settings size={12} />
                    </button>
                    <button
                      type="button"
                      class={clsx(
                        styles.treeActionBtn,
                        styles.treeActionBtnDanger,
                      )}
                      title="Delete"
                      onClick={(e) => {
                        e.stopPropagation();
                        setDeletingTarget({ id: target.id, name: target.name });
                      }}
                    >
                      <Trash2 size={12} />
                    </button>
                  </div>
                </div>
              </>
            )}
          </For>

          <Show when={targets.length === 0}>
            <p class={styles.emptyTree}>No targets yet</p>
          </Show>
        </div>
      </ScrollArea>

      <Show when={editingTarget() !== null}>
        <Portal>
          <TargetDialog
            initial={
              editingTarget() === "new"
                ? undefined
                : (editingTarget() as UdpTarget)
            }
            onSave={handleSave}
            onClose={() => setEditingTarget(null)}
          />
        </Portal>
      </Show>

      <Show when={deletingTarget()}>
        {(t) => (
          <Portal>
            <ConfirmDialog
              title="Delete target"
              message={`Are you sure you want to delete "${t().name}"? This action cannot be undone.`}
              onConfirm={() => {
                deleteTarget(t().id).catch((err: unknown) => {
                  console.error("Failed to delete target:", err);
                });
                setDeletingTarget(null);
              }}
              onCancel={() => setDeletingTarget(null)}
            />
          </Portal>
        )}
      </Show>
    </div>
  );
}
