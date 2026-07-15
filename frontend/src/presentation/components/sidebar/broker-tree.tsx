import { clsx } from "clsx";
import { createMemo, createSignal, Show } from "solid-js";
import { Portal } from "solid-js/web";
import type {
  BrokerProfile,
  ConnectionState,
} from "../../../domain/mqtt/types";
import { useMqttConnection } from "../../providers/mqtt-provider";
import { BrokerSettingsDialog } from "../mqtt/broker-settings-dialog";
import { ProfileList } from "./profile-list";
import styles from "./sidebar.module.css";

export function BrokerTree() {
  const {
    profiles,
    connections,
    activeConnectionId,
    createOfflineConnection,
    handleConnect,
    handleReconnect,
    switchConnection,
    closeConnection,
    saveProfile,
    deleteProfile,
    reorderProfiles,
  } = useMqttConnection();

  const [editingProfile, setEditingProfile] = createSignal<
    BrokerProfile | "new" | null
  >(null);

  // Memoize profileId → ConnectionState index to avoid repeated object→Array conversions
  const connectionByProfileId = createMemo(() => {
    const map = new Map<string, ConnectionState>();
    for (const conn of Object.values(connections)) {
      map.set(conn.profileId, conn);
    }
    return map;
  });

  const isProfileConnected = (profileId: string) => {
    const conn = connectionByProfileId().get(profileId);
    return conn?.type === "online" && conn.connected;
  };

  const getConnectionForProfile = (profileId: string) =>
    connectionByProfileId().get(profileId);

  const getConnectionIdForProfile = (profileId: string) =>
    connectionByProfileId().get(profileId)?.connectionId;

  const handleProfileClick = (profileId: string) => {
    const existingConn = getConnectionForProfile(profileId);
    if (existingConn) {
      switchConnection(existingConn.connectionId);
    } else {
      const profile = profiles().find((p) => p.id === profileId);
      if (profile) createOfflineConnection(profile);
    }
  };

  const handleProfileSave = async (profile: BrokerProfile) => {
    await saveProfile(profile);
    setEditingProfile(null);
    createOfflineConnection(profile);
  };

  const handleProfileSaveAndConnect = async (profile: BrokerProfile) => {
    await saveProfile(profile);
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
    const conn = connections[connId];
    return conn?.profileId === profileId;
  };

  return (
    <>
      <ProfileList
        title="Brokers"
        addLabel="New Broker"
        emptyMessage="No brokers yet"
        items={profiles()}
        onAdd={() => setEditingProfile("new")}
        onItemClick={(p) => handleProfileClick(p.id)}
        onEdit={(p) => setEditingProfile(p)}
        onDelete={(p) => handleProfileDelete(p.id)}
        onReorder={reorderProfiles}
        isActive={(p) => isActive(p.id)}
        editAriaLabel="Edit broker"
        deleteAriaLabel="Delete broker"
        deleteTitle="Delete broker"
        renderContent={(profile) => (
          <>
            <span
              class={clsx(
                styles.statusDot,
                isProfileConnected(profile.id)
                  ? styles.statusDotConnected
                  : styles.statusDotDisconnected,
              )}
              title={
                isProfileConnected(profile.id) ? "Connected" : "Disconnected"
              }
              aria-hidden="true"
            />
            <div class={styles.listInfo}>
              <span class={styles.listName}>{profile.name}</span>
              <span class={styles.listSub}>{profile.broker}</span>
            </div>
          </>
        )}
      />

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
    </>
  );
}
