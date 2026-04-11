import { createSignal } from "solid-js";
import type { OpenApiFile } from "../../domain/openapi/types";

const STORAGE_KEY = "wirexa:openapi-files";
const MAX_FILES = 50;

function loadFromStorage(): OpenApiFile[] {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return [];
    return JSON.parse(raw) as OpenApiFile[];
  } catch {
    return [];
  }
}

function saveToStorage(files: OpenApiFile[]): void {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(files));
}

function generateId(): string {
  return crypto.randomUUID();
}

export function createFilesState() {
  const [files, setFiles] = createSignal<OpenApiFile[]>(loadFromStorage());
  const [activeFileId, setActiveFileId] = createSignal<string | null>(null);

  function persist(updated: OpenApiFile[]): void {
    setFiles(updated);
    saveToStorage(updated);
  }

  function addFile(path: string, name: string): OpenApiFile {
    const existing = files().find((f) => f.path === path);
    if (existing) {
      const updated = files().map((f) =>
        f.path === path ? { ...f, lastOpenedAt: new Date().toISOString() } : f,
      );
      persist(updated);
      setActiveFileId(existing.id);
      return existing;
    }

    let updated = [
      ...files(),
      {
        id: generateId(),
        name,
        path,
        order: files().length,
        lastOpenedAt: new Date().toISOString(),
      },
    ];

    // 最大保持件数を超えた場合は lastOpenedAt が古いものを削除
    if (updated.length > MAX_FILES) {
      updated = [...updated]
        .sort(
          (a, b) =>
            new Date(b.lastOpenedAt).getTime() -
            new Date(a.lastOpenedAt).getTime(),
        )
        .slice(0, MAX_FILES);
    }

    // order を振り直す
    updated = updated.map((f, i) => ({ ...f, order: i }));
    persist(updated);

    const added = updated.find((f) => f.path === path);
    if (added) setActiveFileId(added.id);
    return added ?? updated[0];
  }

  function removeFile(id: string): void {
    const updated = files()
      .filter((f) => f.id !== id)
      .map((f, i) => ({ ...f, order: i }));
    persist(updated);
    if (activeFileId() === id) {
      setActiveFileId(updated.length > 0 ? updated[0].id : null);
    }
  }

  function moveFile(id: string, newIndex: number): void {
    const current = [...files()].sort((a, b) => a.order - b.order);
    const idx = current.findIndex((f) => f.id === id);
    if (idx === -1) return;
    const [item] = current.splice(idx, 1);
    current.splice(newIndex, 0, item);
    const updated = current.map((f, i) => ({ ...f, order: i }));
    persist(updated);
  }

  function getActiveFile(): OpenApiFile | null {
    const id = activeFileId();
    if (!id) return null;
    return files().find((f) => f.id === id) ?? null;
  }

  return {
    files,
    activeFileId,
    setActiveFileId,
    addFile,
    removeFile,
    moveFile,
    getActiveFile,
  };
}

export type FilesState = ReturnType<typeof createFilesState>;
