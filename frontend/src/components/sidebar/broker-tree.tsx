import { clsx } from "clsx";
import { Plus, Settings, Trash2 } from "lucide-solid";
import { createMemo, createSignal, For, Show } from "solid-js";
import { Portal } from "solid-js/web";
import { BrokerSettingsDialog } from "../mqtt/broker-settings-dialog";
import { useMqtt } from "../mqtt/context";
import type { BrokerProfile, ConnectionState } from "../mqtt/types";
import { Button } from "../ui/button";
import { ConfirmDialog } from "../ui/confirm-dialog";
import { ScrollArea } from "../ui/scroll-area";
import styles from "./sidebar.module.css";

export function BrokerTree() {
  const {
    profiles,
    connections,
    activeConnectionId,
    handleConnect,
    handleReconnect,
    switchConnection,
    closeConnection,
    saveProfile,
    deleteProfile,
  } = useMqtt();

  const [editingProfile, setEditingProfile] = createSignal<
    BrokerProfile | "new" | null
  >(null);
  const [deletingProfile, setDeletingProfile] = createSignal<{
    id: string;
    name: string;
  } | null>(null);

  // Memoize profileId → ConnectionState index to avoid repeated Map→Array conversions
  const connectionByProfileId = createMemo(() => {
    const map = new Map<string, ConnectionState>();
    for (const conn of connections().values()) {
      map.set(conn.profileId, conn);
    }
    return map;
  });

  const isProfileConnected = (profileId: string) =>
    connectionByProfileId().get(profileId)?.connected ?? false;

  const getConnectionForProfile = (profileId: string) =>
    connectionByProfileId().get(profileId);

  const getConnectionIdForProfile = (profileId: string) =>
    connectionByProfileId().get(profileId)?.connectionId;

  const handleProfileClick = (profileId: string) => {
    const existingConn = getConnectionForProfile(profileId);
    if (existingConn) {
      switchConnection(existingConn.connectionId);
    } else {
      handleConnect(profileId);
    }
  };

  const handleProfileSave = (profile: BrokerProfile) => {
    saveProfile(profile);
    setEditingProfile(null);
  };

  const handleProfileSaveAndConnect = (profile: BrokerProfile) => {
    saveProfile(profile);
    setEditingProfile(null);
    const existingConn = getConnectionForProfile(profile.id);
    if (existingConn) {
      handleReconnect(existingConn.connectionId);
    } else {
      handleConnect(profile.id);
    }
  };

  const handleProfileDelete = (id: string) => {
    const connId = getConnectionIdForProfile(id);
    if (connId) closeConnection(connId);
    deleteProfile(id);
  };

  const isActive = (profileId: string) => {
    const connId = activeConnectionId();
    if (!connId) return false;
    const conn = connections().get(connId);
    return conn?.profileId === profileId;
  };

  return (
    <div class={styles.collectionTree}>
      <div class={styles.collectionHeader}>
        <span class={styles.collectionTitle}>Brokers</span>
        <Button
          variant="ghost"
          size="icon"
          class={styles.collectionAction}
          onClick={() => setEditingProfile("new")}
          title="New Broker"
        >
          <Plus size={14} />
        </Button>
      </div>

      <ScrollArea class={styles.treeScroll}>
        <div class={styles.treeList}>
          <For each={profiles()}>
            {(profile) => (
              <>
                {/* biome-ignore lint/a11y/useSemanticElements: contains nested action buttons; cannot use <button> with nested interactive elements */}
                <div
                  role="button"
                  tabIndex={0}
                  class={clsx(
                    styles.brokerItem,
                    isActive(profile.id) && styles.brokerItemActive,
                  )}
                  onClick={() => handleProfileClick(profile.id)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter" || e.key === " ")
                      handleProfileClick(profile.id);
                  }}
                >
                  <span
                    class={clsx(
                      styles.brokerDot,
                      isProfileConnected(profile.id)
                        ? styles.brokerDotConnected
                        : styles.brokerDotDisconnected,
                    )}
                  />
                  <div class={styles.brokerInfo}>
                    <span class={styles.brokerName}>{profile.name}</span>
                    <span class={styles.brokerUrl}>{profile.broker}</span>
                  </div>
                  <div class={styles.treeNodeActions}>
                    <button
                      type="button"
                      class={styles.treeActionBtn}
                      title="Edit"
                      onClick={(e) => {
                        e.stopPropagation();
                        setEditingProfile(profile);
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
                        setDeletingProfile({
                          id: profile.id,
                          name: profile.name,
                        });
                      }}
                    >
                      <Trash2 size={12} />
                    </button>
                  </div>
                </div>
              </>
            )}
          </For>

          <Show when={profiles().length === 0}>
            <p class={styles.emptyTree}>No brokers yet</p>
          </Show>
        </div>
      </ScrollArea>

      <Show when={editingProfile() !== null}>
        <Portal>
          <BrokerSettingsDialog
            profile={
              editingProfile() === "new"
                ? undefined
                : (editingProfile() as BrokerProfile)
            }
            onSave={handleProfileSave}
            onSaveAndConnect={handleProfileSaveAndConnect}
            onClose={() => setEditingProfile(null)}
          />
        </Portal>
      </Show>

      <Show when={deletingProfile()}>
        {(profile) => (
          <Portal>
            <ConfirmDialog
              title="Delete broker"
              message={`Are you sure you want to delete "${profile().name}"? This action cannot be undone.`}
              onConfirm={() => {
                handleProfileDelete(profile().id);
                setDeletingProfile(null);
              }}
              onCancel={() => setDeletingProfile(null)}
            />
          </Portal>
        )}
      </Show>
    </div>
  );
}
