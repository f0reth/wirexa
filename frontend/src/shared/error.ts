/** unknown 値から人間可読なエラーメッセージを取り出す。 */
export function errorMessage(err: unknown): string {
  return err instanceof Error ? err.message : String(err);
}
