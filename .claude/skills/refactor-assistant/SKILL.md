---
name: refactor-assistant
description: 既存機能を壊さずにできるリファクタリング候補を調査し、レポートを出力する
argument-hint: "[backend|frontend] [http|mqtt|udp|openapi]"
disable-model-invocation: true
---

このプロジェクト（Wails + Go + SolidJS）を対象に、既存機能を壊さずにリファクタリング可能な箇所を調査してください。

## 実行手順

**推測・記憶・想像による判断は禁止。** 実際のコードを開いて確認してから判断してください。

1. 「対象範囲の決定」に従って調査範囲と出力先を決める
2. 範囲内の調査対象を実際に読む（テスト `*_test.go`・`*.test.ts` は、候補の挙動を守っているテストを探すときに読む）
3. 調査観点に基づいてリファクタリング候補を洗い出す
4. 各候補を「プロジェクトの規約・意図的な設計」と照合し、該当するものは「対応不要」に回す
5. 決めた出力先に結果を出力する

## 対象範囲の決定

全体（テストを除いて Go 約 8,000 行・TS 約 11,500 行）を 1 回で読むと後半の読みが浅くなるため、`$ARGUMENTS` で範囲を絞れる。

| `$ARGUMENTS` | 調査範囲 | 出力先 |
|---|---|---|
| 空 | 下記「調査対象」のすべて | `docs/refactor-report.md` |
| `backend` | 「バックエンド」の表だけ | `docs/refactor-report-backend.md` |
| `frontend` | 「フロントエンド」の表だけ | `docs/refactor-report-frontend.md` |
| `http`・`mqtt`・`udp`・`openapi` のいずれか | 両側のうち、そのプロトコルのパス（下記） | `docs/refactor-report-<proto>.md` |
| `backend http` のような側とプロトコルの組み合わせ | 指定した側の、そのプロトコルのパス | `docs/refactor-report-<側>-<proto>.md` |

- プロトコル指定時の「そのプロトコルのパス」は次のとおり。
  - バックエンド: `internal/{domain,application,infrastructure}/<proto>/`・`internal/adapters/<proto>_handler.go`
  - フロントエンド: `{domain,application,infrastructure}/<proto>/`・`presentation/components/<proto>/`・`presentation/providers/<proto>-provider.tsx`・下表のプロトコル専用ファイル

    | プロトコル | `presentation/components/sidebar/` | その他 |
    |---|---|---|
    | http | `collection-*.tsx`・`tree-item-node.tsx`・`use-tree-drag-drop.ts`・`drag-state.ts`・`tree-ui-context.tsx`・`rename-input.tsx`・`use-long-press-drag.ts` | `presentation/constants/http.ts` |
    | mqtt | `broker-tree.tsx`・`profile-list.tsx` | 無し |
    | udp | `target-tree.tsx`・`profile-list.tsx` | 無し |
    | openapi | `openapi-file-*.tsx`・`use-long-press-drag.ts` | 無し |

    `profile-list.tsx`（mqtt・udp）と `use-long-press-drag.ts`（http・openapi）は 2 つのプロトコルで共有しているため、どちらを指定しても範囲に含める。改善候補を出すときは、もう一方のプロトコルへの影響も確認する。
- 範囲外のファイルは、候補の影響範囲の確認や「既に共通化されたものを再実装していないか」の確認のために読む。範囲外のファイル自体の改善候補はレポートに載せない。範囲外になるのは次のファイル。
  - バックエンドの共通部分: `internal/domain/*.go`・`internal/infrastructure/*.go`・`application/store/`・`internal/adapters/file_dialog.go`（http・openapi で共用）・`internal/adapters/log_handler.go`・`app.go`・`main.go`
  - フロントの共通部分: `shared/`・`config/`・`components/ui/`・`application/{shared,ui}/`・`application/logger.ts`・`presentation/components/shared/`・`presentation/utils/`・`sidebar/index.tsx`・`sidebar/protocol-switcher.tsx` など
- プロトコル間の共通化候補は、範囲外のプロトコルの対応箇所を比較対象として読んだうえで載せてよい。
- 上記以外の値が渡されたら、調査せずに受け付ける値を示して終了する。

## 調査対象

**バックエンド（`internal/`・リポジトリルート）**

| 層 | 調査パス |
|---|---|
| Domain | `internal/domain/*.go`・`internal/domain/{http,mqtt,udp,openapi}/` |
| Application | `internal/application/{http,mqtt,udp,openapi}/`・`internal/application/store/` |
| Infrastructure | `internal/infrastructure/*.go`・`internal/infrastructure/{http,mqtt,udp,openapi}/` |
| Adapters | `internal/adapters/*_handler.go`・`internal/adapters/file_dialog.go` |
| 合成ルート | `app.go`・`main.go` |

**フロントエンド（`frontend/src/`）**

| 層 | 調査パス |
|---|---|
| Domain | `domain/{http,mqtt,udp,openapi,ui}/` |
| Application | `application/{http,mqtt,udp,openapi,shared,ui}/`・`application/logger.ts` |
| Infrastructure | `infrastructure/{http,mqtt,udp,openapi,storage,logger,app}/` |
| Presentation | `presentation/`（`components/`・`providers/`・`constants/`・`utils/`）・`App.tsx` |
| 汎用 UI | `components/ui/` |
| Shared | `shared/`・`config/` |

**調査対象外**: `frontend/wailsjs/` と `frontend/src/shared/wails-events.ts`（生成物）、`internal/testutil/`、`internal/integration/`、`frontend/e2e/`、`tools/`（`gen-events`・`e2e-broker` などの開発用ツール）。ただし、これらも候補の影響範囲としては確認する。

## 前提となる設計（違反する提案をしないこと）

### 依存方向（厳守）

CLAUDE.md の「Architecture」（依存方向・入出力ポートの置き場所・合成ルート・import 制限）と「ドメイン型・RPC・永続化の境界」に従う。加えて次を前提とする。

- openapi は保存用のポートを持たないため `domain/openapi/ports.ts` は無い。
- infrastructure → application の既存 import 2 か所の import 先である `application/openapi/ports.ts`（`ParseResult`）と `application/logger.ts`（`Logger`）は、application に置かれたポートとして現行の設計で扱う。domain への移動などは提案せず、見つけたら「対応不要と判断した箇所」に理由つきで載せる。
- フロントの `application/` が solid-js の signal / store で状態を持つのは現行の設計であり、それ自体は問題にしない。

### 意図的な重複（共通化・削除を提案しない）

以下は同期漏れを防ぐため、または境界を分けるための**意図的な重複**。見つけたら「対応不要と判断した箇所」に理由つきで載せる。

| 重複 | 理由 |
|---|---|
| フロント `domain/<proto>/types.ts` と Go のドメイン型 | Go の型が RPC の配線型の正で、フロント側はユニオン型・型ガードで意味付けする層。生成型→フロント型の変換は `infrastructure/<proto>/client.ts` でスプレッド素通し＋ユニオン検証として行う（openapi には `client.ts` が無く、`infrastructure/openapi/file-io.ts` がファイル I/O と最近使ったファイルの RPC を、`parser.ts` が仕様の解析を受け持つ）。生成型を application / presentation から直接使う案は規約違反 |
| infrastructure の `storedXxx` DTO と Go のドメイン型 | 保存形式を RPC の json タグから独立させるため（CLAUDE.md「ドメイン型・RPC・永続化の境界」）。`testutil.AssertNoTypesFrom` は struct 型だけを検査するが、現状の stored DTO に名前付きの基本型を使う箇所は無いので、使用箇所を探さなくてよい。統合・埋め込みの提案は禁止 |
| adapters に RPC 用 DTO が無いこと | ドメインと同じ形の DTO を adapters に新設する提案は禁止（ドメインに対応物の無いアダプタ固有の入力型は可） |
| フロントとバックの入力検証（例: `frontend/src/domain/udp/types.ts` の数値検証と `internal/domain/udp/encoding.go`） | 正はバックエンド。フロントは送信前の先回り検証で、UX のために持つ |
| フロントのエラー文言定数（例: `RESPONSE_UNAVAILABLE_ERROR` と `ErrResponseUnavailable`） | RPC 越しにはエラー文字列しか渡らないため |
| 名前付き文字列型を RPC のメソッド引数で `string` として受けて内部変換している箇所 | Wails がメソッド引数の名前付き型を生成できない制約への回避策 |

入力検証とエラー文言定数は、削除ではなく「ずれをどう防ぐか」（テスト・コメントでの対応付け）なら提案してよい。

## 調査観点

**Go バックエンド**

- 重複コードの共通化（4 プロトコル間の類似処理。共通の置き場所になり得る `internal/infrastructure/*.go`・`internal/domain/*.go`・`application/store/` に既にあるものを再実装していないか）
- エラーハンドリングの不統一（`fmt.Errorf` の `%w` 付け忘れ、センチネルエラー・`NotFoundError`・`ValidationError` を使うべき所で文字列だけのエラーを返している、`errors.Is`/`errors.As` ではなく文字列比較している、など。`fmt.Errorf("...: %w")` とカスタムエラー型の併用自体は Go の通常の書き方なので問題にしない）
- インターフェース・ポートの不必要な肥大化
- 不要なポインタ渡し・値渡しの混在（RPC とイベントの境界にかかるものは「禁止事項」参照）

**TypeScript フロントエンド**

- `application/` 層での状態管理の重複（4 プロトコル間の類似パターン）
- `presentation/` コンポーネントへのロジック漏れ（application に移せる業務ロジック）
- 型定義の重複（フロント内部での重複定義。Go 型のミラーは「意図的な重複」参照）
- Solid.js のリアクティビティを正しく活用していない箇所

**共通**

- 同じロジックがバックエンド・フロントエンド両方に実装されている箇所（意図的な重複かどうかを必ず判定する）
- 不使用コード（dead code）

## 禁止事項

- 大規模リライト
- フレームワーク変更
- 挙動変更。このプロジェクトでは以下も挙動変更として扱う:
  - **保存形式の変更**（`storedXxx` DTO の json タグ・構造。基準は `internal/infrastructure/*/testdata/*.golden.json`）
  - **設定データの復旧方針の変更**（破損・読み込み失敗時の扱い。CLAUDE.md と `internal/application/store/recovery.go` の表）
  - **RPC のシグネチャ・ドメイン型の json タグ・イベント名の変更**
  - **バインド対象のハンドラメソッドの引数・戻り値やドメイン型のフィールドを、ポインタから値へ、または値からポインタへ変えること**。Wails v2 は `*T` と `T` から同じ TS 型を生成するため、`frontend/wailsjs` の差分には出ない。それでも nil が `null` として渡るかどうかが変わる。「不要なポインタ渡し・値渡しの混在」の候補は、RPC とイベントの境界の内側（application・infrastructure の内部）に限る
  - **ファイルアクセスの方式の変更**（HTTP の `FileReference.Token`、OpenAPI の `checkGranted` による許可リスト。RPC で受け取ったパスを検証なしに読み書きしない。`docs/http-local-file-access-hardening.md`）
- クリーンアーキテクチャの破壊（上記「依存方向」に反する提案）
- 生成物（`frontend/wailsjs/`・`frontend/src/shared/wails-events.ts`）を手で編集する提案

## 候補ごとに確認すること

各候補について次を調べ、「出力形式」の同名の項目に書く。

- **挙動を守っているテスト**: その箇所を検証している既存テスト
- **検証コマンド**: 常に `task format` → `task lint` → `task test`。加えて、並行処理に触れるなら `task go:test:race`、`internal/integration/` が検証する振る舞いに関わるなら `task go:test:integration`、UI の振る舞いに関わるなら `task frontend:test:e2e`
- **付随作業**:
  - バインド対象の Go 構造体・ハンドラメソッドに触れるなら `task wails:generate` を実行する
  - `internal/domain/events.go` のイベント定数に触れるなら `task go:generate:events` を実行する。イベントを追加・移動する場合は `tools/gen-events/main.go` のリストも直す
  - バインド API の実装やイベントの発行を動かすなら、`frontend/e2e/fake-backend/` への影響の有無を書く（fake-backend も `WailsEvents` を参照している）

## 出力形式

出力先に以下の構成で Markdown を出力してください。見出しには調査範囲を書く。

```
# リファクタリング調査レポート（対象: 全体 / backend / frontend / http など）

## 改善候補一覧

| 優先度 | ファイル | 内容 |
|--------|----------|------|
| 高     | ...      | ...  |

## 改善候補の詳細

### [タイトル]
- **該当箇所**: `ファイルパス:行番号`
- **現状**: 何が問題か
- **改善案**: どう直すか
- **期待効果**: 可読性向上 / 重複削減 / パフォーマンス改善 など
- **挙動を守っているテスト**: `xxx_test.go` など（無ければ「無し（先にテストを追加）」）
- **検証コマンド**: `task test` など
- **付随作業**: `task wails:generate`・fake-backend の追従など（無ければ「無し」）

## 対応不要と判断した箇所（理由つき）
```
