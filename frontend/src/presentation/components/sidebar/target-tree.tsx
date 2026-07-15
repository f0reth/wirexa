import { createSignal, onMount, Show } from "solid-js";
import { Portal } from "solid-js/web";
import { notify } from "../../../application/ui/notifications";
import { Button } from "../../../components/ui/button";
import { createFocusTrap } from "../../../components/ui/focus-trap";
import { Input } from "../../../components/ui/input";
import type { UdpTarget } from "../../../domain/udp/types";
import { errorMessage } from "../../../shared/error";
import { useUdpSend, useUdpTargets } from "../../providers/udp-provider";
import { ProfileList } from "./profile-list";
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
  let dialogRef: HTMLDivElement | undefined;

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

  const { onKeyDown } = createFocusTrap(
    () => dialogRef,
    () => props.onClose(),
  );

  return (
    <div class={styles.dialogOverlay}>
      <div
        ref={dialogRef}
        class={styles.dialog}
        role="dialog"
        aria-modal="true"
        onKeyDown={onKeyDown}
      >
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

  const handleSave = async (t: UdpTarget) => {
    await saveTarget(t);
    setEditingTarget(null);
  };

  return (
    <>
      <ProfileList
        title="Targets"
        addLabel="New Target"
        emptyMessage="No targets yet"
        items={targets}
        onAdd={() => setEditingTarget("new")}
        onItemClick={(t) => loadTarget(t)}
        onEdit={(t) => setEditingTarget(t)}
        onDelete={(t) => {
          deleteTarget(t.id).catch((err: unknown) => {
            notify.error("Failed to delete target", errorMessage(err));
          });
        }}
        onReorder={reorderTargets}
        editAriaLabel="Edit target"
        deleteAriaLabel="Delete target"
        deleteTitle="Delete target"
        renderContent={(target) => (
          <div class={styles.listInfo}>
            <span class={styles.listName}>{target.name}</span>
            <span class={styles.listSub}>
              {target.host}:{target.port}
            </span>
          </div>
        )}
      />

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
    </>
  );
}
