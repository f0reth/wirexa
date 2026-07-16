const LONG_PRESS_MS = 250;
const DRAG_THRESHOLD_PX = 5;

/**
 * 長押し（LONG_PRESS_MS）またはマウス移動 DRAG_THRESHOLD_PX 超でドラッグ開始する
 * mousedown ハンドラを作る。開始時にセットするドラッグ状態は呼び出し側ごとに
 * 異なるため onActivate に委譲する。
 *
 * suppressRef はドラッグ後の mouseup で click が誤発火するのを抑えるためのもので、
 * 立てるのはここ、降ろすのは呼び出し側の onClick。
 *
 * ドロップ検出は sidebar/use-tree-drag-drop.ts（ツリー）と
 * openapi-file-tree.tsx（OpenAPI ファイル一覧）が各々担う。
 */
export function makeLongPressDragHandlers(
  suppressRef: { suppress: boolean },
  onActivate: (x: number, y: number) => void,
) {
  const handleMouseDown = (e: MouseEvent) => {
    if (e.button !== 0) return;
    const startX = e.clientX;
    const startY = e.clientY;

    const activate = (x: number, y: number) => {
      suppressRef.suppress = true;
      onActivate(x, y);
      document.removeEventListener("mousemove", handleMove);
      document.removeEventListener("mouseup", handleUp);
    };

    const handleMove = (me: MouseEvent) => {
      const dx = me.clientX - startX;
      const dy = me.clientY - startY;
      if (Math.sqrt(dx * dx + dy * dy) > DRAG_THRESHOLD_PX) {
        clearTimeout(timer);
        activate(me.clientX, me.clientY);
      }
    };

    const handleUp = () => {
      clearTimeout(timer);
      document.removeEventListener("mousemove", handleMove);
      document.removeEventListener("mouseup", handleUp);
    };

    const timer = setTimeout(() => {
      document.removeEventListener("mousemove", handleMove);
      document.removeEventListener("mouseup", handleUp);
      activate(startX, startY);
    }, LONG_PRESS_MS);

    document.addEventListener("mousemove", handleMove);
    document.addEventListener("mouseup", handleUp);
  };

  return { handleMouseDown };
}
