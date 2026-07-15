/**
 * 配列内の要素を from から to へ移動した新しい配列を返す。
 * インデックスが範囲外の場合は null を返す（呼び出し側は元の配列を維持できる）。
 * 永続化などの副作用は含まない純粋関数。
 */
export function moveItem<T>(
  arr: readonly T[],
  from: number,
  to: number,
): T[] | null {
  if (from < 0 || from >= arr.length || to < 0 || to >= arr.length) return null;
  const next = [...arr];
  const [moved] = next.splice(from, 1);
  next.splice(to, 0, moved);
  return next;
}
