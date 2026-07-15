import {
  GetRecents,
  MoveRecent,
  OpenFilePicker,
  ReadFile,
  RemoveRecent,
  SaveFileAs,
  WriteFile,
} from "../../../wailsjs/go/adapters/OpenAPIHandler";
import type { adapters } from "../../../wailsjs/go/models";

export async function openFilePicker(): Promise<string> {
  return OpenFilePicker();
}

export async function readFile(path: string): Promise<string> {
  return ReadFile(path);
}

export async function writeFile(path: string, content: string): Promise<void> {
  return WriteFile(path, content);
}

/**
 * saveFileAs はネイティブ保存ダイアログを開き、選択先へ content を書き込む。
 * 保存パスを返す（キャンセル時は空文字）。無題文書の初回保存に使う。
 */
export async function saveFileAs(
  defaultName: string,
  content: string,
): Promise<string> {
  return SaveFileAs(defaultName, content);
}

export async function getRecents(): Promise<adapters.OpenAPIRecent[]> {
  return GetRecents();
}

export async function removeRecent(path: string): Promise<void> {
  return RemoveRecent(path);
}

export async function moveRecent(path: string, index: number): Promise<void> {
  return MoveRecent(path, index);
}
