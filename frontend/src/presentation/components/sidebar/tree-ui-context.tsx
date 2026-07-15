import { createContext, type JSX, useContext } from "solid-js";
import type { TreeItem } from "../../../domain/http/types";

/**
 * ツリー（コレクション / フォルダ / リクエスト）の描画に必要な
 * rename UI 状態とアクションハンドラをまとめて配布するコンテキスト。
 * CollectionTree が値を構築し、CollectionNode / TreeItemNode が参照する
 * ことで、再帰ツリーへの prop drilling を解消する。
 */
export interface TreeUiContextValue {
  activeRequestId: () => string | null;
  renamingItemId: () => string | null;
  setRenamingItemId: (id: string | null) => void;
  renamingCollectionId: () => string | null;
  setRenamingCollectionId: (id: string | null) => void;
  onAddFolder: (collectionId: string, parentId: string) => void;
  onAddRequest: (collectionId: string, parentId: string) => void;
  onDeleteItem: (
    collectionId: string,
    itemId: string,
    name: string,
    type: string,
  ) => void;
  onDeleteCollection: (id: string, name: string) => void;
  onSelectRequest: (item: TreeItem, collectionId: string) => void;
  onRenameItem: (collectionId: string, itemId: string, name: string) => void;
  onRenameCollection: (id: string, name: string) => void;
}

const TreeUiContext = createContext<TreeUiContextValue>();

export function TreeUiProvider(props: {
  value: TreeUiContextValue;
  children: JSX.Element;
}) {
  return (
    <TreeUiContext.Provider value={props.value}>
      {props.children}
    </TreeUiContext.Provider>
  );
}

export function useTreeUi(): TreeUiContextValue {
  const ctx = useContext(TreeUiContext);
  if (!ctx) {
    throw new Error("useTreeUi must be used within TreeUiProvider");
  }
  return ctx;
}
