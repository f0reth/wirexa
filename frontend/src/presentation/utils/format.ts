/**
 * JSON 文字列をインデント 2 で整形する。
 * パースできない場合は null を返す。呼び出し側が原文フォールバックと
 * シンタックスハイライトの抑止を選び分けられるようにするための契約。
 */
export function formatJson(text: string): string | null {
  try {
    return JSON.stringify(JSON.parse(text), null, 2);
  } catch {
    return null;
  }
}

/**
 * epoch ミリ秒または Date をローカル時刻の `HH:MM:SS.mmm` に整形する。
 * MQTT (`timestamp: Date`) と UDP (`timestamp: number`) の両方を受けるため
 * union を取る。
 */
export function formatTime(value: Date | number): string {
  const d = typeof value === "number" ? new Date(value) : value;
  const hh = String(d.getHours()).padStart(2, "0");
  const mm = String(d.getMinutes()).padStart(2, "0");
  const ss = String(d.getSeconds()).padStart(2, "0");
  const ms = String(d.getMilliseconds()).padStart(3, "0");
  return `${hh}:${mm}:${ss}.${ms}`;
}
