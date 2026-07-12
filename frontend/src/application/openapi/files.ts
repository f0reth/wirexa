import { createSignal } from "solid-js";
import type { ActiveDoc, OpenApiFile } from "../../domain/openapi/types";
import {
  getRecents,
  moveRecent,
  removeRecent,
} from "../../infrastructure/openapi/file-io";

/**
 * createFilesState は「最近使ったファイル一覧」とアクティブ文書の状態を管理する。
 * recents は Go 側（設定ディレクトリの JSON）を単一の真実とし、localStorage は使わない。
 * Go の許可リストへの登録はダイアログ/保存経由でしか起きないため、JS からは汚染できない。
 */
export function createFilesState() {
  const [files, setFiles] = createSignal<OpenApiFile[]>([]);
  const [activeDoc, setActiveDoc] = createSignal<ActiveDoc>(null);

  async function refreshRecents(): Promise<void> {
    const recents = await getRecents();
    setFiles(
      recents.map((r) => ({
        path: r.path,
        name: r.name,
        order: r.order,
        lastOpenedAt: r.lastOpenedAt,
      })),
    );
  }

  async function removeFile(path: string): Promise<void> {
    await removeRecent(path);
    await refreshRecents();
    const doc = activeDoc();
    if (doc?.kind === "file" && doc.path === path) setActiveDoc(null);
  }

  async function moveFile(path: string, newIndex: number): Promise<void> {
    await moveRecent(path, newIndex);
    await refreshRecents();
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
