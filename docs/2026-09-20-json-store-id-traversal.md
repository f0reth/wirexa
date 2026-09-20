# 変更計画書: JSONStore の ID によるディレクトリトラバーサル対策

## 概要

`docs/backend-architecture-review.md` の「2. JSONStore の ID によるディレクトリトラバーサル」(重要度: 重大) に対応する。

現状、`JSONStore.Save` / `JSONStore.Delete` はドメインオブジェクトの ID を検証せずファイルパスへ結合している (`internal/infrastructure/json_store.go:95`, `:101`)。`CachedStore.Save` は ID が空のときだけ UUID を採番し、非空の ID はクライアント由来でもそのまま受理する (`internal/application/store/cached_store.go:73`)。MQTT profile と UDP target の保存 API は RPC 由来の ID をこの経路へ渡すため、`../../../Documents/example` のような ID を与えると設定ストア外へ JSON を書き込め、同じ ID で削除すればストア外のファイル削除にも使える。

レビューの推奨どおり**フル対応**する。

1. **新規 ID は必ず application 層で採番し、クライアント指定の新規 ID を拒否する**
2. **更新・削除は、ロード済みの正規 ID だけを受理する**
3. **`JSONStore` 側でも多層防御として ID 形式を検証し、`filepath.Rel` で最終パスがストア内であることを確認する**

1 に伴い、MQTT の `SaveProfile` RPC は採番後のプロファイルを返す必要がある (現状は `error` のみで、フロントエンドが `crypto.randomUUID()` で ID を作って送っている)。UDP の `SaveTarget` は既に `(UDPTarget, error)` を返し、フロントも空 ID を送っているため RPC 面の変更は不要。

### 現状調査で確認した事実

| 対象                | 新規 ID の出どころ                                       | 対応の要否                                     |
| ------------------- | -------------------------------------------------------- | ---------------------------------------------- |
| MQTT BrokerProfile  | **フロント** (`application/mqtt/profiles.ts:16`)          | フロント・RPC・バックエンドすべて変更が必要     |
| UDP UDPTarget       | バックエンド (フロントは `id: ""` を送る)                  | バックエンドの受理条件のみ変更                 |
| HTTP Collection     | バックエンド (`uuid.NewString()`)、削除は cache 存在確認   | 変更不要 (JSONStore 側の多層防御のみ効く)      |
| sidebar_layout.json / openapi-recents.json | 固定ファイル名                            | 変更不要                                       |

## 変更対象ファイル

### バックエンド

| ファイル                                          | 層             | 変更種別 | 変更内容                                                                                                             |
| ------------------------------------------------- | -------------- | -------- | -------------------------------------------------------------------------------------------------------------------- |
| `internal/domain/id.go`                           | Domain         | 追加     | 永続化ファイル名に使える安全な ID 規則 `ValidateID(id string) error` を定義する (外部依存なし)                        |
| `internal/domain/id_test.go`                      | Domain         | 追加     | ID 規則のテーブルテスト (トラバーサル・区切り文字・予約デバイス名・`__root__` の許可など)                             |
| `internal/infrastructure/json_store.go`           | Infrastructure | 変更     | `resolve(id)` を追加。`Save` / `Delete` で ID を検証し `filepath.Rel` による封じ込めを確認する。`Load` は不正 ID の項目を記録してスキップする |
| `internal/infrastructure/json_store_test.go`      | Infrastructure | 変更     | トラバーサル・絶対パス・予約名・空 ID の攻撃ケースを追加し、ストア外にファイルが生成／削除されないことを検証          |
| `internal/application/store/cached_store.go`      | Application    | 変更     | `Save` で「空 ID → UUID 採番 / 未知の非空 ID → `NotFoundError`」に変更する                                            |
| `internal/application/store/cached_store_test.go` | Application    | 変更     | `Save_KeepsProvidedID` を「既存 ID のときだけ維持」に変更し、未知 ID 拒否のケースを追加                              |
| `internal/domain/mqtt/port.go`                    | Domain         | 変更     | `ProfileUseCase.SaveProfile` の戻り値を `(BrokerProfile, error)` へ変更                                               |
| `internal/application/mqtt/profile_service.go`    | Application    | 変更     | `SaveProfile` が採番後のプロファイルを返すようにする                                                                  |
| `internal/application/mqtt/profile_service_test.go` | Application  | 変更     | 新規は空 ID・戻り値で検証する形へ更新 (後述「影響を受ける既存テスト」)                                                |
| `internal/adapters/mqtt_handler.go`               | Adapters       | 変更     | `SaveProfile` の戻り値を `(mqttdomain.BrokerProfile, error)` にして素通しする                                        |
| `internal/application/udp/target_service_test.go` | Application    | 変更     | `SaveTarget_NewWithID` を「未知 ID は拒否される」テストへ変更                                                         |
| `internal/integration/mqtt_test.go`               | Integration    | 変更     | 新規プロファイル保存を空 ID + 戻り値利用へ更新。トラバーサル ID がハンドラ経由で拒否されることの検証を追加            |
| `internal/integration/udp_test.go`                | Integration    | 変更     | トラバーサル ID がハンドラ経由で拒否され、ストア外にファイルが作られないことの検証を追加                              |

### フロントエンド

| ファイル                                                   | 層             | 変更種別 | 変更内容                                                                                       |
| ---------------------------------------------------------- | -------------- | -------- | ---------------------------------------------------------------------------------------------- |
| `frontend/wailsjs/`                                        | 生成物         | 再生成   | `task wails:generate` (`MQTTHandler.SaveProfile` の戻り値変更)                                  |
| `frontend/src/infrastructure/mqtt/client.ts`               | Infrastructure | 変更     | `saveProfile` の戻り値を `Promise<BrokerProfile>` にする (変換は既存同様の素通し)                |
| `frontend/src/infrastructure/mqtt/client.test.ts`          | Infrastructure | 変更     | `SaveProfile` のモック戻り値と、保存結果を返すことの検証を追加                                  |
| `frontend/src/application/mqtt/profiles.ts`                | Application    | 変更     | `createEmptyProfile()` の `id` を `""` にする。`ProfileApi.saveProfile` / `saveProfile` は保存済みプロファイルを返し、state と order キーには採番後の ID を使う |
| `frontend/src/application/mqtt/profiles.test.ts`           | Application    | 追加     | 空 ID で保存 → サーバ採番 ID が state と order に反映されることを検証                           |
| `frontend/src/application/mqtt/connections.ts`             | Application    | 変更     | `createConnectionsState` の `saveProfile` 引数型を `Promise<BrokerProfile>` へ更新              |
| `frontend/src/presentation/providers/mqtt-provider.tsx`    | Presentation   | 変更     | コンテキストの `saveProfile` 型を更新                                                           |
| `frontend/src/presentation/components/sidebar/broker-tree.tsx` | Presentation | 変更   | `handleProfileSave` / `handleProfileSaveAndConnect` で**保存後の戻り値**を使って接続を作る      |
| `frontend/e2e/fake-backend/install.ts`                     | e2e            | 変更     | `SaveProfile` が保存済みプロファイルを返すようにし、`SaveProfile` / `SaveTarget` で未知の非空 ID を拒否する (Go の意味論に追従) |

## 実装方針

### 1. 安全な ID 規則を domain に置く (`internal/domain/id.go`)

ファイル名として安全かどうかは業務ルールではないが、**「ID は 1 個のファイル名になる」という不変条件**は application (採番) と infrastructure (パス結合) の両方が守る必要がある。規則が 2 か所に割れるのを避けるため、外部依存のない `internal/domain` に置く。`internal/infrastructure/json_store.go` も `internal/application/store/cached_store.go` も既に `internal/domain` を import しており、依存方向 (adapters/application/infrastructure → domain) を壊さない。

```go
// ValidateID は永続化ファイル名として使う ID を検証する。
// 許可するのは ASCII の英数字と "." "_" "-" のみで、パス区切り・ドライブ指定・
// 制御文字・Unicode の別表現をまとめて排除する。
func ValidateID(id string) error
```

拒否する条件:

- 空文字列、128 文字超
- `[A-Za-z0-9._-]` 以外の文字 (`/` `\` `:` NUL 制御文字などがすべてここで落ちる)
- `.` および `..`
- 先頭または末尾が `.`
- Windows の予約デバイス名 (`CON` `PRN` `AUX` `NUL` `COM1`-`COM9` `LPT1`-`LPT9`)。**判定は「最初の `.` より前の部分」に対して大小文字を無視して行う**

予約名の判定を ID 全体の完全一致にしてはならない。Windows は拡張子が付いても予約名として解決するため、ID 内部の `.` を許可したまま完全一致で判定すると `CON.foo` や `LPT1.backup` が検証を通り、最終的なファイル名 `CON.foo.json` も通常ファイルとして扱えない。判定対象は必ず先頭セグメント (`strings.Cut(id, ".")` の前半) とする。

```go
// 例: "CON" "con" "CON.txt" "com1.backup" "LPT1.x.y" はすべて拒否。
//     "console" "con1" "com0" "com10" は先頭セグメント全体が予約名と一致しないので許可。
head, _, _ := strings.Cut(id, ".")
if isWindowsReservedName(head) { // 大小文字を無視して比較する
    return &ValidationError{Field: "id", Message: "..."}
}
```

`COM0` / `COM10` / `console` のように予約名へ前方一致するだけの名前は正規のデバイス名ではないため許可する (先頭セグメント全体の一致で判定すれば自然にそうなる)。

`__root__` (予約コレクション ID) と UUID はこの規則を通る。既存の保存済みデータ (UUID と `__root__`) はすべて適合するため、移行処理は不要。

### 2. JSONStore の多層防御 (`internal/infrastructure/json_store.go`)

`Save` / `Delete` が共通で使う `resolve(id) (string, error)` を追加する。

1. `domain.ValidateID(id)` を通す
2. `filepath.Join(s.dir, id+".json")` を組み立てる
3. `filepath.Rel(s.dir, dest)` の結果が `id + ".json"` と一致することを確認する (一致しなければストア外を指している)

2 段構えにするのは、規則の取りこぼしがあってもパス封じ込めで止めるため。エラーは `*domain.ValidationError` (Field: `"id"`) で返し、RPC 経由でも呼び出し元に意味のあるエラーが届くようにする。

`Load` は `os.ReadDir` の結果を読むためパス自体は安全だが、**ファイルの中身に書かれた ID** はそのまま domain 型とキャッシュのキーになる。ストア外を指す ID を持つファイルが置かれていた場合にキャッシュを汚さないよう、`Load` でも `ValidateID` に通らない項目は記録してスキップする (破損ファイルと違い JSON としては妥当なので `.corrupt` への退避はしない)。これにより「メモリ上の ID はすべて安全な basename」という不変条件が成立し、`CachedStore` の存在確認が防御として意味を持つ。

### 3. ID 採番を application 層に閉じる (`internal/application/store/cached_store.go`)

```go
id := s.getID(item)
switch {
case id == "":
    s.setID(&item, uuid.NewString()) // 新規はサーバで採番する
default:
    if _, ok := s.items[id]; !ok {
        // クライアントが指定した未知の ID は新規作成として受理しない。
        return zero, &cmn.NotFoundError{Resource: s.resource, ID: id}
    }
}
```

これで `Save` は「空 ID = 新規作成 (サーバ採番)」「非空 ID = ロード済みの既存項目の更新」だけになり、`Delete` の既存の存在確認と対称になる。`NotFoundError` を使うのは、意味としては「更新対象が存在しない」であり、`Delete` の挙動と揃うため。

`CachedStore` は MQTT profile と UDP target の両方が使うので、ここ 1 か所で両方の穴が閉じる。HTTP Collection は `CollectionService` が独自キャッシュを持つが、作成は `uuid.NewString()`、削除・更新はキャッシュ存在確認済みなので追加変更は不要 (JSONStore 側の多層防御だけが新たに効く)。

### 4. MQTT SaveProfile の戻り値変更

`ProfileUseCase.SaveProfile` を `(BrokerProfile, error)` へ変更する。UDP `TargetUseCase.SaveTarget` と同じ形になり、「採番はサーバ側、結果はクライアントへ返す」というパターンが 2 プロトコルで揃う。

- `internal/domain/mqtt/port.go`: インターフェース変更
- `internal/application/mqtt/profile_service.go`: `return s.store.Save(profile)` を返すだけ
- `internal/adapters/mqtt_handler.go`: ドメイン型をそのまま返す薄いパススルーを維持する (DTO は新設しない)

`app.go` の配線は変更不要 (サービス追加ではなく既存メソッドのシグネチャ変更のため)。

### 5. フロントエンドを「ID を作らない」形にする

- `createEmptyProfile()` は `id: ""` を返す。`generateId` の import は不要になる
- `createProfilesState.saveProfile` は API の戻り値 (採番済みプロファイル) で state を更新し、`mqtt:profileOrder` の localStorage にも採番後の ID を入れる。ここを戻り値ベースにしないと、新規作成時に `id: ""` のエントリが state と order キーに残る
- `broker-tree.tsx` の保存ハンドラは `const saved = await saveProfile(profile)` として、`createOfflineConnection(saved)` / `handleConnect(saved.id)` に採番後の ID を渡す
- `connections.ts` / `mqtt-provider.tsx` は `saveProfile` の型を更新する (`Promise<BrokerProfile>` は `Promise<void>` に代入できないため、型だけの追従が必要)

## コード生成

- [x] `task wails:generate` — **必要**。`MQTTHandler.SaveProfile` の戻り値が `error` から `(mqttdomain.BrokerProfile, error)` に変わるため、`frontend/wailsjs/go/adapters/MQTTHandler.d.ts` / `.js` が更新される。CI が `git diff --exit-code frontend/wailsjs` で鮮度を検査する
- [ ] `task go:generate:events` — **不要**。イベントの追加・変更はない

## テスト方針

| 系統              | 内容                                                                                                                                                                                       |
| ----------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Go ユニット       | `internal/domain/id_test.go` (新規): ID 規則のテーブルテスト。許可 (`uuid`, `__root__`, `item1`, `console`, `con1`, `com0`, `com10`)、拒否 (`../x`, `..\\x`, `/abs`, `C:\\x`, `..`, `.`, `.hidden`, `trailing.`, 空, 129 文字, `a/b`, NUL 入り)、**予約デバイス名は拡張子付きも拒否** (`CON`, `con`, `CON.txt`, `com1.backup`, `LPT1.x.y`, `nul`, `AUX.json`) |
| Go ユニット       | `internal/infrastructure/json_store_test.go`: `Save` / `Delete` にトラバーサル ID を与え、(1) `*domain.ValidationError` が返る (2) ストアの親ディレクトリにファイルが作られない (3) 事前配置した外部ファイルが削除されない、を検証。`Load` が不正 ID の項目をスキップすることも検証 |
| Go ユニット       | `internal/application/store/cached_store_test.go`: 空 ID → UUID 採番 / 既存 ID → 更新 / 未知の非空 ID → `NotFoundError` かつ repo が呼ばれない                                             |
| Go ユニット       | `internal/application/mqtt/profile_service_test.go`, `internal/application/udp/target_service_test.go`: 新仕様へ更新 (下表)                                                                  |
| Go 統合           | `internal/integration/mqtt_test.go`, `udp_test.go`: 既存 CRUD / round-trip を空 ID + 戻り値の形へ更新。加えて、ハンドラ経由でトラバーサル ID を渡しても設定ディレクトリ外にファイルが作られないことを検証 |
| フロント ユニット | `frontend/src/infrastructure/mqtt/client.test.ts`: `SaveProfile` の戻り値を返すこと。`frontend/src/application/mqtt/profiles.test.ts` (新規): 空 ID で保存したとき、state と `mqtt:profileOrder` にサーバ採番 ID が入ること |
| UI e2e            | `frontend/e2e/ui/mqtt/mqtt.spec.ts`: プロファイル新規作成〜表示のフローが引き続き通ることを確認 (偽バックエンドの採番に追従)。仕様変更に合わせて `frontend/e2e/fake-backend/install.ts` を更新する |
| フルスタック e2e  | `frontend/e2e/integration/common/backend-integration.spec.ts` の MQTT プロファイル操作を Windows ローカルで確認する (CI 非対象)                                                             |

### 影響を受ける既存テスト

| テスト                                        | 現状                            | 変更後                                     |
| --------------------------------------------- | ------------------------------- | ------------------------------------------ |
| `TestCachedStore_Save_KeepsProvidedID`        | 任意の指定 ID を維持            | **既存 ID のときだけ**維持する検証へ        |
| `TestTargetService_SaveTarget_NewWithID`      | 未知 ID `t99` で新規作成できる  | 未知 ID は `NotFoundError` になる検証へ     |
| `TestProfileService_SaveProfile_AddNew`       | 未知 ID `p1` で新規作成         | 空 ID + 戻り値で検証                       |
| `TestProfileService_SaveProfile_RepoError`    | 未知 ID `p1` で repo エラー確認 | 空 ID にして repo まで到達させる           |
| `TestProfileService_SaveProfile_GeneratesIDWhenEmpty` | `GetProfiles` 経由で確認 | 戻り値で直接確認                           |
| `TestProfileService_MultipleProfiles`         | 指定 ID で 3 件作成             | 空 ID で 3 件作成                          |
| `TestMQTT_ProfileCRUD` / `TestMQTT_ProfilePersistenceRoundTrip` | `uuid.NewString()` を送る | 空 ID を送り、戻り値の ID を使う |

`go test` / `go vet` は `main.go` の `//go:embed` のため `frontend/dist/index.html` が必要。

## 副作用・注意事項

- **RPC の振る舞いが変わる**: `SaveProfile` / `SaveTarget` に未知の非空 ID を渡すと `NotFoundError` になる。これは意図した仕様変更で、フロントエンドの正規フロー (新規は空 ID、更新は取得済み ID) はそのまま通る
- **`MQTTHandler.SaveProfile` は破壊的なシグネチャ変更**。`frontend/wailsjs/` を再生成しないとフロントのビルドが通らない
- **MQTT 新規プロファイルの ID がサーバ採番になる**。`mqtt:profileOrder` (localStorage) は保存後の ID を保持するため、戻り値を使う実装にしないと新規プロファイルの並び順が壊れる
- 既存の保存済みファイル (UUID ファイル名、`__root__.json`) はすべて新しい ID 規則に適合するため、**データ移行は不要**
- `JSONStore.Load` が不正 ID の項目をスキップするようになるため、外部で作られた妙な ID のファイルがあると読み込まれなくなる。実運用では該当データは存在しない想定だが、スキップ時にロガーへ記録して追跡可能にする
- HTTP Collection 側の挙動は変わらない (既にサーバ採番 + キャッシュ存在確認)。レビュー項目 3 (コレクション更新のトランザクション性) は本計画のスコープ外

## Git運用

- **ブランチ名**: `fix/json-store-id-traversal`
- **コミット分割方針** (層ごと・日本語):
  1. `feat(domain): 永続化 ID の安全性検証を追加する` — `internal/domain/id.go` + テスト
  2. `fix(infrastructure): JSONStore の ID 検証とパス封じ込めを追加する` — `json_store.go` + テスト (この時点で多層防御が成立)
  3. `fix(application): ID 採番をサーバ側に閉じる` — `cached_store.go` + 各サービステストの更新
  4. `refactor(mqtt): SaveProfile が保存済みプロファイルを返すようにする` — domain port / service / adapter + `task wails:generate` の生成物
  5. `refactor(frontend): MQTT プロファイルの ID 生成をやめてサーバ採番を使う` — フロント各層 + 偽バックエンド + テスト
  6. `test(integration): ID トラバーサルが拒否されることを検証する` — 統合テスト
- **完了後**: `task format` → `task lint` → `task test` がすべて通ることを確認 → `task frontend:test:e2e` → (Windows ローカルで) `task frontend:test:e2e:fullstack` → main へマージ
