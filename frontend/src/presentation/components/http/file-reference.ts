import type { FileReference } from "../../../domain/http/types";

// ファイル入力欄の placeholder。保存済み・移行済みの参照は token を持たないため、
// 以前のファイル名を示して再選択を促す。
export function filePlaceholder(ref: FileReference | undefined): string {
  return ref?.needsReselect && ref.name
    ? `Reselect file: ${ref.name}`
    : "No file selected";
}
