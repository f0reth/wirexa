// ブローカープロファイル編集フォームの入力検証。

/** プロファイルダイアログが編集する、検証対象の入力値。 */
interface ProfileDraftInput {
  name: string;
  host: string;
  /** ポートは入力欄の文字列そのまま（未入力は ""）。 */
  port: string;
}

// 検証を通った値は composeBrokerUrl で broker URL に組み立てて保存し、次にダイアログを開くと
// parseBrokerUrl で読み戻す。読み戻せない値を通すと既定値（mqtt://localhost:1883）に戻るので、
// どちらも parseBrokerUrl の正規表現が読める形に限る。

// Number() は "1e3" や "1883.0"、"0x50"、前後の空白も数値として読むので、数字だけに限る。
export function isValidBrokerPort(port: string): boolean {
  if (!/^\d+$/.test(port)) return false;
  const portNum = Number(port);
  return portNum >= 1 && portNum <= 65535;
}

// コロン（IPv6 リテラルやポート付き）、空白、パスを含むホストは読み戻せない。
function isValidBrokerHost(host: string): boolean {
  return /^[^\s:/]+$/.test(host);
}

export function isValidProfileDraft(input: ProfileDraftInput): boolean {
  return (
    input.name.trim().length > 0 &&
    isValidBrokerHost(input.host) &&
    isValidBrokerPort(input.port)
  );
}
