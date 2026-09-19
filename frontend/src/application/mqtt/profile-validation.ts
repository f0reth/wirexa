// ブローカープロファイル編集フォームの入力検証。

/** プロファイルダイアログが編集する、検証対象の入力値。 */
export interface ProfileDraftInput {
  name: string;
  host: string;
  /** ポートは入力欄の文字列そのまま（未入力は ""）。 */
  port: string;
}

export function isValidBrokerPort(port: string): boolean {
  const portNum = Number(port);
  return Number.isInteger(portNum) && portNum >= 1 && portNum <= 65535;
}

export function isValidProfileDraft(input: ProfileDraftInput): boolean {
  return (
    input.name.trim().length > 0 &&
    input.host.trim().length > 0 &&
    isValidBrokerPort(input.port)
  );
}
