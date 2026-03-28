import { createStore, reconcile } from "solid-js/store";
import type { UdpTarget } from "../../domain/udp/types";

export interface UdpTargetApi {
  getTargets(): Promise<UdpTarget[]>;
  saveTarget(target: UdpTarget): Promise<UdpTarget>;
  deleteTarget(id: string): Promise<void>;
}

export function createTargetsState(api: UdpTargetApi) {
  const [targets, setTargets] = createStore<UdpTarget[]>([]);

  async function refreshTargets(): Promise<void> {
    const list = await api.getTargets();
    setTargets(reconcile(list));
  }

  refreshTargets();

  return {
    targets,
    refreshTargets,
    saveTarget: async (t: UdpTarget) => {
      const saved = await api.saveTarget(t);
      await refreshTargets();
      return saved;
    },
    deleteTarget: async (id: string) => {
      await api.deleteTarget(id);
      await refreshTargets();
    },
  };
}
