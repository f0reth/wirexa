import { createSignal } from "solid-js";
import type { ActiveDoc, OpenApiFile } from "../../domain/openapi/types";
import type { Notifier } from "../../domain/ui/ports";
import { runGuarded } from "../ui/guard";

export interface OpenApiRecentsApi {
  getRecents(): Promise<OpenApiFile[]>;
  removeRecent(path: string): Promise<void>;
  moveRecent(path: string, index: number): Promise<void>;
}

/**
 * createFilesState は「最近使ったファイル一覧」とアクティブ文書の状態を管理する。
 * recents は Go 側（設定ディレクトリの JSON）を単一の真実とし、localStorage は使わない。
 * Go の許可リストへの登録はダイアログ/保存経由でしか起きないため、JS からは汚染できない。
 */
export function createFilesState(api: OpenApiRecentsApi, notifier: Notifier) {
  const [files, setFiles] = createSignal<OpenApiFile[]>([]);
  const [activeDoc, setActiveDoc] = createSignal<ActiveDoc>(null);

  async function refreshRecents(): Promise<void> {
    setFiles(await api.getRecents());
  }

  async function removeFile(path: string): Promise<void> {
    await api.removeRecent(path);
    await refreshRecents();
    const doc = activeDoc();
    if (doc?.kind === "file" && doc.path === path) setActiveDoc(null);
  }

  async function moveFile(path: string, newIndex: number): Promise<void> {
    await runGuarded(notifier, "Failed to reorder file", async () => {
      await api.moveRecent(path, newIndex);
      await refreshRecents();
    });
  }

  return {
    files,
    activeDoc,
    setActiveDoc,
    refreshRecents,
    removeFile,
    moveFile,
  };
}

export type FilesState = ReturnType<typeof createFilesState>;
