/**
 * setLoading を管理しながら非同期処理を実行するヘルパー。
 * finally で setLoading(false) を保証することでの呼び忘れを防ぐ。
 */
export async function withLoading<T>(
  setLoading: (v: boolean) => void,
  fn: () => Promise<T>,
): Promise<T> {
  setLoading(true);
  try {
    return await fn();
  } finally {
    setLoading(false);
  }
}
