import { createStore, reconcile } from "solid-js/store";
import type { UdpTarget } from "../../domain/udp/types";
import {
  loadFromStorage,
  saveToStorage,
} from "../../infrastructure/storage/local-storage";

const TARGET_ORDER_KEY = "udp:targetOrder";

export interface UdpTargetApi {
  getTargets(): Promise<UdpTarget[]>;
  saveTarget(target: UdpTarget): Promise<UdpTarget>;
  deleteTarget(id: string): Promise<void>;
}

function applyOrder(targets: UdpTarget[], order: string[]): UdpTarget[] {
  const orderMap = new Map(order.map((id, i) => [id, i]));
  return [...targets].sort((a, b) => {
    const ai = orderMap.get(a.id) ?? Number.POSITIVE_INFINITY;
    const bi = orderMap.get(b.id) ?? Number.POSITIVE_INFINITY;
    return ai - bi;
  });
}

export function createTargetsState(api: UdpTargetApi) {
  const [targets, setTargets] = createStore<UdpTarget[]>([]);

  async function refreshTargets(): Promise<void> {
    const list = await api.getTargets();
    const order = loadFromStorage<string[]>(TARGET_ORDER_KEY, []);
    setTargets(reconcile(applyOrder(list, order)));
  }

  function reorderTargets(fromIndex: number, toIndex: number): void {
    const next = [...targets];
    if (
      fromIndex < 0 ||
      fromIndex >= next.length ||
      toIndex < 0 ||
      toIndex >= next.length
    )
      return;
    const [item] = next.splice(fromIndex, 1);
    next.splice(toIndex, 0, item);
    saveToStorage(
      TARGET_ORDER_KEY,
      next.map((t) => t.id),
    );
    setTargets(reconcile(next));
  }

  refreshTargets();

  return {
    targets,
    refreshTargets,
    reorderTargets,
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
