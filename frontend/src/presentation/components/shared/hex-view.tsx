import { createMemo, Show } from "solid-js";
import styles from "./hex-view.module.css";

// 表示するバイト数の上限。超過分は打ち切り、注意書きを出す。
// HTTP は Save、MQTT は Copy で全体を取得できる想定。
const DEFAULT_MAX_BYTES = 64 * 1024;
const BYTES_PER_ROW = 16;

/** base64 文字列をデコードせずにバイト長を推定する（プレビュー表示用）。 */
export function base64ByteLength(base64: string): number {
  const len = base64.length;
  if (len === 0) return 0;
  let padding = 0;
  if (base64[len - 1] === "=") padding++;
  if (base64[len - 2] === "=") padding++;
  return Math.floor((len * 3) / 4) - padding;
}

/** base64 文字列をバイト列にデコードする。不正な base64 の場合は null を返す。 */
function decodeBase64(base64: string): Uint8Array | null {
  try {
    const binary = atob(base64);
    const bytes = new Uint8Array(binary.length);
    for (let i = 0; i < binary.length; i++) {
      bytes[i] = binary.charCodeAt(i);
    }
    return bytes;
  } catch {
    return null;
  }
}

/** バイト列を offset/hex/ASCII の 3 カラムテキストに整形する。 */
function formatHexDump(bytes: Uint8Array): string {
  const lines: string[] = [];
  for (let offset = 0; offset < bytes.length; offset += BYTES_PER_ROW) {
    const chunk = bytes.subarray(offset, offset + BYTES_PER_ROW);
    const offsetCol = offset.toString(16).padStart(8, "0");

    let hexCol = "";
    let asciiCol = "";
    for (let i = 0; i < BYTES_PER_ROW; i++) {
      if (i < chunk.length) {
        const b = chunk[i];
        hexCol += b.toString(16).padStart(2, "0");
        // 印字可能な ASCII 範囲のみそのまま、それ以外は "."
        asciiCol += b >= 0x20 && b <= 0x7e ? String.fromCharCode(b) : ".";
      } else {
        hexCol += "  ";
      }
      hexCol += i === BYTES_PER_ROW / 2 - 1 ? "  " : " ";
    }
    lines.push(`${offsetCol}  ${hexCol.trimEnd()}  ${asciiCol}`);
  }
  return lines.join("\n");
}

/** base64 エンコードされたバイナリを hex ダンプとして表示する共有コンポーネント。 */
export function HexView(props: { base64: string; maxBytes?: number }) {
  const decoded = createMemo(() => decodeBase64(props.base64));

  const view = createMemo(() => {
    const bytes = decoded();
    if (bytes === null) return null;
    const max = props.maxBytes ?? DEFAULT_MAX_BYTES;
    const truncated = bytes.length > max;
    const shown = truncated ? bytes.subarray(0, max) : bytes;
    return {
      total: bytes.length,
      shownCount: shown.length,
      truncated,
      dump: formatHexDump(shown),
    };
  });

  return (
    <Show
      when={view()}
      fallback={
        <div class={styles.hexError}>Failed to decode binary content.</div>
      }
    >
      {(v) => (
        <>
          <div class={styles.hexMeta}>
            Binary content ({v().total.toLocaleString()} bytes)
          </div>
          <Show when={v().truncated}>
            <div class={styles.hexTruncated}>
              ⚠ Showing the first {v().shownCount.toLocaleString()} bytes. Save
              or copy to get the full content.
            </div>
          </Show>
          <pre class={styles.hexDump}>{v().dump}</pre>
        </>
      )}
    </Show>
  );
}
