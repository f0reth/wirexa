# Backend Architecture Review

調査日: 2026-09-09

## 概要

Wirexa の backend は、`domain`、`application`、`infrastructure`、`adapters` を分離し、`app.go` を composition root とする構成になっている。依存性逆転の基本方針は概ね守られており、Logger、Emitter、Repository、ネットワーククライアントをポートとして抽象化している点は良好である。

一方、セキュリティ境界、複数ファイルにまたがる状態整合性、長時間処理のライフサイクル管理には優先度の高い問題がある。本書では、調査時点で確認できた問題を重要度順にまとめる。

## 1. HTTP ファイル送信のローカルファイルアクセスが制限されていない

重要度: **重大**

### 問題

OpenAPI 機能は「Wails RPC は WebView 上の任意の JavaScript から呼び出せる」という前提で、OS のファイルダイアログから得たパスだけを許可する仕組みを持っている。

- [`internal/adapters/openapi_handler.go`](../internal/adapters/openapi_handler.go#L20)
- [`internal/adapters/openapi_handler.go`](../internal/adapters/openapi_handler.go#L101)

しかし HTTP 機能では、RPC から渡されたファイルパスを許可確認なしで読み込んでいる。

- 通常の file body: [`internal/infrastructure/http/net_client.go`](../internal/infrastructure/http/net_client.go#L164)
- multipart の file 行: [`internal/infrastructure/http/form_body.go`](../internal/infrastructure/http/form_body.go#L40)
- レスポンス保存元の一時ファイルパス: [`internal/adapters/http_handler.go`](../internal/adapters/http_handler.go#L73)

### 影響

WebView 側に XSS、依存パッケージ侵害、または不正な RPC 呼び出しが発生した場合、backend の権限で任意のローカルファイルを読み込み、HTTP リクエストを通じて外部へ送信できる。`SaveResponseBody` では呼び出し側が指定したファイルをコピー後に削除するため、任意ファイルの読み取り・削除にもつながる。

### 推奨対応

- HTTP 側にも OpenAPI と同等のパス許可管理を導入する。
- RPC へ生のファイルパスを渡さず、backend が生成した不透明な file token を使用する。
- `SaveResponseBody` はパスではなく request execution ID を受け取り、backend 内部の追跡済み一時ファイルだけを操作する。
- symlink やパス差し替えを考慮し、必要に応じてファイルハンドルを保持する。

## 2. JSONStore の ID によるディレクトリトラバーサル

重要度: **重大**

### 問題

`JSONStore.Save` と `JSONStore.Delete` は、ドメインオブジェクトの ID を検証せずファイルパスへ結合している。

- [`internal/infrastructure/json_store.go`](../internal/infrastructure/json_store.go#L91)

`CachedStore` は ID が空の場合だけ UUID を生成し、空でない ID はそのまま使用する。

- [`internal/application/store/cached_store.go`](../internal/application/store/cached_store.go#L68)

MQTT profile と UDP target の保存 API は、RPC 由来の ID を保存処理へ渡している。UDP target の検証対象は host と port だけで、ID は検証されない。

- [`internal/application/mqtt/profile_service.go`](../internal/application/mqtt/profile_service.go#L36)
- [`internal/application/udp/target_service.go`](../internal/application/udp/target_service.go#L34)
- [`internal/domain/udp/types.go`](../internal/domain/udp/types.go#L60)

### 影響

`../../../Documents/example` のような ID を与えると、設定ストアの外側へ JSON ファイルを書き込める。保存後に同じ ID を削除すれば、ストア外のファイル削除にも利用できる。

### 推奨対応

- 新規 ID は常に application 層で生成し、クライアント指定を許可しない。
- 更新・削除時は、既に読み込まれた正規 ID だけを受理する。
- `JSONStore` 側でも defense in depth として UUID 形式または安全な basename を検証する。
- `filepath.Rel` などを利用し、最終パスがストアディレクトリ内にあることを確認する。

## 3. コレクションとサイドバーレイアウトの更新がトランザクションになっていない

重要度: **高**

### 問題

コレクション、ルートコレクション、サイドバーレイアウトが別ファイルとして永続化され、複数の保存処理が順次実行される。途中で失敗してもロールバックされない。

代表例:

- コレクション作成後にレイアウト保存が失敗すると、API はエラーを返すがコレクションは残る: [`internal/application/http/collection_service.go`](../internal/application/http/collection_service.go#L110)
- コレクション削除後にレイアウト更新が失敗すると、存在しないコレクションへのエントリが残る: [`internal/application/http/collection_service.go`](../internal/application/http/collection_service.go#L134)
- コレクション間移動は source、target の順に保存するため、target の保存失敗で永続状態が分断される: [`internal/application/http/collection_service.go`](../internal/application/http/collection_service.go#L287)
- sidebar への移動では source、root、layout の3つを順次更新する: [`internal/application/http/collection_service.go`](../internal/application/http/collection_service.go#L397)

さらに、多くの操作ではキャッシュ上のオブジェクトを先に変更してから `Save` している。`Save` が失敗してもメモリ上の変更は元に戻らない。

- [`internal/application/http/collection_service.go`](../internal/application/http/collection_service.go#L219)
- [`internal/application/http/collection_service.go`](../internal/application/http/collection_service.go#L237)
- [`internal/application/http/collection_service.go`](../internal/application/http/collection_service.go#L261)
- [`internal/application/http/collection_service.go`](../internal/application/http/collection_service.go#L437)

### 影響

- API のエラー結果と実際の状態が一致しない。
- メモリ、collection JSON、root collection JSON、sidebar layout JSON が互いに異なる状態になる。
- 再起動前後で表示される状態が変わる。
- sidebar layout が空でない場合は起動時に再構築されないため、不整合が継続する。

### 推奨対応

- キャッシュを直接変更せず、ディープコピー上で変更する。
- 必要な永続化がすべて成功してからキャッシュを差し替える。
- コレクションとレイアウトを同じ aggregate／保存単位へまとめることを検討する。
- 複数ファイルを維持する場合は journal、generation、manifest、または unit of work を導入する。
- 起動時に sidebar entry と実データの突合・修復を行う。
- 保存失敗時に状態が変わらないことを検証する regression test を追加する。

## 4. HTTP リクエストの永続 ID と実行 ID が混同されている

重要度: **高**

### 問題

`HTTPRequestService` は、実行中リクエストのキャンセル関数を保存済みリクエストの `req.ID` で管理している。

- [`internal/application/http/request_service.go`](../internal/application/http/request_service.go#L32)

同じ保存済みリクエストを並行送信すると、後発のキャンセル関数が先発を上書きする。その後、先発リクエストの `defer` が map のエントリを削除すると、後発リクエストをキャンセルできなくなる。空 ID も許可されているため、未保存リクエスト同士でも同じ問題が起きる。

レスポンスの一時ファイルも `req.ID` をキーとしており、同一 ID の新しいレスポンスが以前の一時ファイルを削除する。

- [`internal/infrastructure/http/net_client.go`](../internal/infrastructure/http/net_client.go#L346)

また、リクエストは `context.Background()` から開始され、アプリケーションの context と結び付いていない。shutdown は MQTT、UDP、一時ファイルの掃除を行うが、実行中 HTTP リクエストをキャンセルしない。

- [`internal/application/http/request_service.go`](../internal/application/http/request_service.go#L39)
- [`app.go`](../app.go#L204)

### 影響

- ユーザーが意図したリクエストと異なる実行がキャンセルされる。
- キャンセル不能なリクエストが残る。
- 並行レスポンスの一時ファイルが失われる。
- shutdown 後に HTTP 処理が継続し、一時ファイルが再作成される可能性がある。

### 推奨対応

- 送信ごとに一意な execution ID を発行する。
- キャンセル、一時ファイル、レスポンスを execution ID で一貫して関連付ける。
- 保存済み HTTPRequest の ID は編集対象の識別だけに使用する。
- application のルート context を `HTTPRequestService` に注入し、shutdown 時に全実行をキャンセルして完了を待つ。

## 5. MQTT 接続ライフサイクルの競合

重要度: **高**

### 問題

`withConn` のコメントは「ロックを保持したまま接続を取得し、関数を呼ぶ」と説明しているが、実装は関数呼び出し前に read lock を解放している。

- [`internal/application/mqtt/service.go`](../internal/application/mqtt/service.go#L127)

このため、接続取得後、Publish／Subscribe／Unsubscribe の実行前または実行中に `Disconnect` が同じ接続を削除・切断できる。

Connect は goroutine で非同期実行される一方、Shutdown は最大5秒しか待たず、その後 map を空にして現在見えている client を切断する。

- [`internal/application/mqtt/service.go`](../internal/application/mqtt/service.go#L55)
- [`internal/application/mqtt/service.go`](../internal/application/mqtt/service.go#L229)

待機時間を超えた Connect が後から成功すると、管理 map から消えた接続や shutdown 後のイベント発行が発生し得る。

### 影響

- 切断済み client に対する Publish／Subscribe。
- ユーザーが切断した後に connected イベントが届く。
- application が追跡できない MQTT 接続が残る。
- shutdown 後に callback が Wails emitter を呼ぶ。

### 推奨対応

- 接続ごとに `connecting`、`connected`、`disconnecting`、`closed` の状態を持たせる。
- Connect 操作をキャンセル可能にする。
- client 操作と Disconnect を同じ per-connection lock または command loop で直列化する。
- service に shutdown 状態を設け、新規 Connect と callback を拒否する。
- Disconnect と Connect、Publish と Disconnect、Shutdown と Connect の競合テストを追加する。

## 6. コレクションキャッシュの可変参照がロック外へ公開される

重要度: **中**

### 問題

`GetCollections` は `Collection` 自体だけを浅くコピーしており、内部の `[]*TreeItem`、`HTTPRequest`、map、slice はキャッシュと共有される。

- [`internal/application/http/collection_service.go`](../internal/application/http/collection_service.go#L82)

`GetRootItems` はキャッシュ内のポインタースライスを直接返している。

- [`internal/application/http/collection_service.go`](../internal/application/http/collection_service.go#L99)

### 影響

- Wails が戻り値を JSON 化している間に別 RPC が更新すると、データ競合になり得る。
- application 層の外部からキャッシュ内容を変更できる。
- mutex がキャッシュを完全には保護していない。

既存の concurrent test は `GetCollections` の戻り値を走査・シリアライズしていないため、この競合を検出しない。

### 推奨対応

- ロック保持中に返却用 DTO へディープコピーする。
- application からポインターを返さず、読み取り専用 snapshot を返す。
- 実際の JSON marshal と更新を並行実行する race test を追加する。

## 7. OpenAPI 機能が application 層を迂回している

重要度: **中**

### 問題

OpenAPI の Wails adapter が次の責務を直接持っている。

- OS ファイルダイアログ
- パスのアクセス制御
- ファイル読み書き
- recent file のビジネスルール
- recent file の JSON 永続化

該当箇所:

- [`internal/adapters/openapi_handler.go`](../internal/adapters/openapi_handler.go#L20)
- [`internal/adapters/openapi_recents.go`](../internal/adapters/openapi_recents.go#L26)

adapter が `internal/infrastructure` を直接 import しており、HTTP、MQTT、UDP で採用している application service／port の構造と一致しない。

### 影響

- OS ダイアログなしでユースケースをテストしにくい。
- 別 UI や CLI から OpenAPI 操作を再利用できない。
- アクセス制御と永続化の変更が Wails adapter に波及する。
- adapter の責務が肥大化する。

### 推奨対応

- OpenAPI application service を追加する。
- `OpenAPIFileAccess`、`RecentRepository` などの output port を定義する。
- OS ダイアログだけを Wails adapter に残す。
- native file I/O と recent JSON store を infrastructure へ移す。

## 8. 再生成可能なサイドバーレイアウトの破損で起動できなくなる

重要度: **中**

### 問題

`SidebarLayoutRepository.Load` は JSON parse error をそのまま返す。

- [`internal/infrastructure/http/sidebar_layout_repository.go`](../internal/infrastructure/http/sidebar_layout_repository.go#L25)

`NewCollectionService` は sidebar layout の読み込み失敗を初期化失敗として返すため、アプリケーション全体が起動しない。

- [`internal/application/http/collection_service.go`](../internal/application/http/collection_service.go#L62)

一方、collection の破損ファイルは隔離して起動を継続し、OpenAPI recents の破損はログを残して空状態から開始する。復旧方針が統一されていない。

### 影響

本体データが正常でも、再生成可能な UI レイアウトファイル1件の破損だけでアプリケーションが利用不能になる。

### 推奨対応

- 破損した layout を隔離する。
- collection と root item から layout を再生成する。
- 起動時に stale entry、duplicate entry、存在しない ID を除去する。
- 設定データを「必須」「再生成可能」「best effort」に分類し、復旧方針を統一する。

### 対応状況

対応済み（2026-09-23）。

- 破損した `sidebar_layout.json` は `.corrupt` へ退避し、コレクションと `__root__` 直下アイテムから再生成して起動を継続する。破損以外の読み込み失敗でも起動は止めない。
- stale / duplicate / 存在しない ID の除去は項目 3 の対応（`reconcileSidebarLayout`）で実装済み。
- 設定データの分類と復旧方針は `CLAUDE.md` と `internal/application/store/recovery.go` にまとめ、共通のセンチネル（`ErrCorruptData`）・ポート（`Quarantiner`）・ヘルパー（`ReadJSONFile`、`store.LoadSingleFile`）で各リポジトリを揃えた。
- 併せて、読み込めなかった `__root__.json` を空の root で上書きしないようにした。

## 9. ドメインモデルと RPC／インフラ DTO の境界が曖昧

重要度: **中〜低**

### 問題

`internal/domain` の型が Wails RPC の JSON DTO、永続化形式、HTTP transport 設定を兼ねている。例えば `HTTPResponse.TempFilePath` は adapter が設定する infrastructure 固有の情報であることがコメントにも明記されている。

- [`internal/domain/http/types.go`](../internal/domain/http/types.go#L203)

また、use case の入力ポートも domain package に置かれている。

- [`internal/domain/http/port.go`](../internal/domain/http/port.go#L25)
- [`internal/domain/mqtt/port.go`](../internal/domain/mqtt/port.go#L4)
- [`internal/domain/udp/port.go`](../internal/domain/udp/port.go#L20)

### 影響

- RPC、永続化、application、domain の変更理由が同じ型へ集中する。
- Wails のバインディング制約が domain model に漏れる。
- infrastructure 固有フィールドが他のユースケースにも見える。

### 推奨対応

- RPC request／response DTO を adapters 側へ分離する。
- use case の入力ポートを application package に置く。
- domain には不変条件と業務概念を表す型を残す。
- 永続化マイグレーションが必要な場合は persistence DTO と domain model の mapper を用意する。

## その他の観察事項

### アップロードファイルを全量メモリへ読み込む

通常の file body は `os.ReadFile`、multipart は `bytes.Buffer` と `os.ReadFile` を使うため、大きなファイルの送信時にファイルサイズ以上のメモリを消費する。

- [`internal/infrastructure/http/net_client.go`](../internal/infrastructure/http/net_client.go#L164)
- [`internal/infrastructure/http/form_body.go`](../internal/infrastructure/http/form_body.go#L19)

送信ボディを streaming reader として組み立てることが望ましい。

### 予約済み root collection の不変条件が保護されていない

`RootCollectionID` は特別な用途を持つが、`DeleteCollection` や `RenameCollection` で拒否されない。

- [`internal/domain/http/types.go`](../internal/domain/http/types.go#L249)
- [`internal/application/http/collection_service.go`](../internal/application/http/collection_service.go#L134)

RPC を直接呼ぶと root collection を削除できるため、application 層で予約 ID の更新・削除を拒否すべきである。

## 良好な点

- `app.go` が composition root として依存関係を明示的に組み立てている。
- application 層が永続化やネットワークの具象実装ではなく port に依存している。
- Logger と Emitter が抽象化され、application service のテストで差し替え可能である。
- Repository 実装に compile-time interface assertion がある。
- HTTP、MQTT、UDP の application service にまとまったユニットテストがある。
- JSON 書き込み時に同一ディレクトリ内の一時ファイルと rename を使用しており、部分書き込みへの対策がある。

## 推奨する対応順序

1. HTTP ファイルアクセスを token／許可リストで保護する。
2. JSONStore の ID と最終パスを検証する。
3. コレクション更新を copy-on-write と整合性のある commit 単位へ変更する。
4. HTTP に execution ID とアプリケーションライフサイクル context を導入する。
5. MQTT 接続を明示的な状態機械とキャンセルで管理する。
6. コレクション返却値を immutable snapshot／DTO にする。
7. OpenAPI use case を application 層へ分離する。
8. sidebar layout の隔離・再生成・整合性修復を実装する。
9. domain、RPC DTO、persistence DTO の境界を段階的に整理する。

## 検証結果

調査時に以下を実行した。

### 成功

application、domain、adapters、および HTTP サブパッケージを除く infrastructure のテストは成功した。

```text
go test ./internal/application/... ./internal/domain/... ./internal/adapters/... ./internal/infrastructure
```

### 完走しなかった検証

`go test ./...`、`go vet ./...`、`golangci-lint run` は `frontend/dist` が存在せず、`main.go` の `//go:embed all:frontend/dist` が解決できないため完走しなかった。

HTTP infrastructure のテストと race test は、macOS で一時ディレクトリを隔離する helper が `TMPDIR` ではなく `TMP`／`TEMP` を設定しているため、次の3テストが失敗した。

- `TestSweepStaleTempFiles`
- `TestNetClient_Truncated_TempFileHoldsFullBody`
- `TestNetClient_AbsoluteLimit_CapsBody`

該当 helper:

- [`internal/infrastructure/http/net_client_cleanup_test.go`](../internal/infrastructure/http/net_client_cleanup_test.go#L15)

失敗した race test の範囲では race detector によるデータ競合報告はなかったが、上記テスト失敗により suite 全体としては成功していない。

## 結論

現在のレイヤー構造は全面的な再設計を必要とする状態ではない。composition root、application service、output port の基本構成を維持したまま改善できる。

ただし、HTTP ファイルアクセスと JSONStore のパス生成は native 権限境界に関わるため最優先で対応すべきである。続いて、コレクション永続化と非同期ネットワーク処理を「失敗しても整合性を失わない」「shutdown 後に処理を残さない」設計へ変更する必要がある。
