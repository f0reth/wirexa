export function applyOrder<T extends { id: string }>(
  items: T[],
  order: string[],
): T[] {
  const orderMap = new Map(order.map((id, i) => [id, i]));
  return [...items].sort((a, b) => {
    const ai = orderMap.get(a.id) ?? Number.POSITIVE_INFINITY;
    const bi = orderMap.get(b.id) ?? Number.POSITIVE_INFINITY;
    return ai - bi;
  });
}
