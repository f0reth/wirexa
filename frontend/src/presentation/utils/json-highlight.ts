function escapeHtml(str: string): string {
  return str.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
}

const JSON_TOKEN_RE =
  /("(?:\\u[a-fA-F0-9]{4}|\\[^u]|[^\\"])*"(?:\s*:)?|true|false|null|-?\d+(?:\.\d*)?(?:[eE][+-]?\d+)?)/g;

function getTokenClass(token: string): string {
  if (token.startsWith('"')) {
    return token.trimEnd().endsWith(":") ? "json-key" : "json-string";
  }
  if (token === "true" || token === "false") return "json-boolean";
  if (token === "null") return "json-null";
  return "json-number";
}

/**
 * フォーマット済み JSON 文字列に構文ハイライト用の span を付与した HTML 文字列を返す。
 * regex を走らせるだけで JSON.parse は呼ばない（呼び出し側で整形済みを渡す前提）。
 */
export function highlightJson(formatted: string): string {
  let result = "";
  let lastIndex = 0;
  for (const match of formatted.matchAll(JSON_TOKEN_RE)) {
    const start = match.index;
    result += escapeHtml(formatted.slice(lastIndex, start));
    const token = match[0];
    result += `<span class="${getTokenClass(token)}">${escapeHtml(token)}</span>`;
    lastIndex = start + token.length;
  }
  result += escapeHtml(formatted.slice(lastIndex));
  return result;
}
