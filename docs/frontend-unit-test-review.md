# フロントエンド単体テストレビュー

生成日時: 2026-09-27
対象: frontend/src/ 配下の単体テスト（`*.test.ts` 29 ファイルと、同ディレクトリの本体コード）

> 行番号はすべてレビュー時点のもの。「本体」は `*.test.ts` を除く同名ファイルを指す。
> 境界値の挙動のうち「Node で確認済み」と書いたものは、本体と同じ式を Node で評価して確かめた。

---

## サマリー

| モジュール | テストケース数 | 不足ケース数 | 優先度(高/中/低) |
|---|---|---|---|
| domain/http/types | 11（+ it.each 3） | 5 | 中 |
| domain/mqtt/topic | 45 | 1 | 低 |
| domain/udp/types | 28（+ it.each 1） | 4 | 中 |
| application/http/collections | 12 | 6 | 高 |
| application/http/request | 24 | 12 | 高 |
| application/mqtt/broker-url | 7 | 2 | 低 |
| application/mqtt/profile-validation | 8 | 2 | 高 |
| application/mqtt/messages | 11 | 1 | 中 |
| application/mqtt/connections | 6 | 13 | 高 |
| application/mqtt/presets | 6 | 4 | 中 |
| application/mqtt/profiles | 5 | 3 | 中 |
| application/openapi/editor | 6 | 4 | 中 |
| application/openapi/files | 5 | 2 | 中 |
| application/udp/field-validation | 28 | 2 | 低 |
| application/udp/receive | 2 | 5 | 高 |
| application/udp/targets | 5 | 2 | 中 |
| application/ui/guard | 6 | 1 | 低 |
| application/ui/notifications | 9 | 4 | 中 |
| components/ui/copy-button | 4 | 2 | 低 |
| presentation/utils/format | 9 | 1 | 低 |
| presentation/components/udp/field-display | 6 | 1 | 低 |
| presentation/components/sidebar/use-long-press-drag | 7 | 3 | 低 |
| shared/array | 7 | 1 | 低 |
| infrastructure/http/client | 76 | 5 | 中 |
| infrastructure/mqtt/client | 23（+ it.each 2） | 1 | 低 |
| infrastructure/mqtt/events | 5（+ it.each 1） | 1 | 低 |
| infrastructure/openapi/file-io | 5 | 2 | 中 |
| infrastructure/storage/local-storage | 43 | 2 | 中 |
| infrastructure/udp/client | 26（+ it.each 1） | 2 | 中 |
| **合計** | **435（+ it.each 8）** | **94** | |

> **優先度の定義**: そのモジュール内の個別不足ケースのうち最も高い優先度を記載する。
> 不足ケース数は本文の「対象」の件数。1 件に複数の関数や分岐をまとめたものがあるので、未テストの関数の数とは一致しない。
> テストケース数は `it(...)` ブロックの数。`it.each` はブロック数を括弧内に別記した（展開後の件数ではない）。

---

## モジュール別詳細

### `frontend/src/domain/http/types.ts`

#### 対象関数・定数

| 名前 | ファイル:行 | 分岐/バリエーション数 | テスト済み? |
|---|---|---|---|
| fileSelectionState | types.ts:36 | 5（undefined / token / hint / needsReselect&&name / それ以外） | 部分的（request.test.ts 経由で hint・reselect のみ） |
| hasUnconfirmedFile | types.ts:48 | 3（file / form-data / その他）＋行条件 4 | 部分的（request.test.ts 経由） |
| isFormBodyType | types.ts:103 | 2 | 部分的（request.test.ts の formBodyType 経由） |
| isResponseUnavailableError | types.ts:178 | 2 | 部分的（request.test.ts 経由で完全一致の文言のみ） |
| isHttpMethod | types.ts:235 | 2 | 済 |
| isBodyType | types.ts:239 | 2 | 済 |
| isFormRowKind | types.ts:252 | 2 | 部分的（http/client.test.ts の未知 kind フォールバック経由のみ） |
| isAuthType | types.ts:262 | 2 | 済 |
| HTTP_METHODS / BODY_TYPES / AUTH_TYPES | types.ts:219 ほか | - | 済（it.each の入力として使用） |
| FORM_ROW_KINDS | types.ts:250 | - | 未使用（types.test.ts は import していない） |

#### 不足テストケース

##### [観点 A] 正常系の網羅

- **対象**: `fileSelectionState`
- **不足内容**: 直接のテストが無い。5 つの戻り分岐のうち、token と hint が両方ある場合に `"selected"` が優先されること、`needsReselect: true` でも `name` が空なら `"none"` になることが未検証
- **根拠コード**: `types.ts:40-43` は token → hint → needsReselect&&name の順に判定する。`name` 無しの reselect 参照は `"none"` になり、`hasUnconfirmedFile` でも止まらずに送信される
- **推奨テスト名**: `it.each([...])("classifies %j as %s", ...)` で 5 分岐を表形式に並べる
- **優先度**: 中

##### [観点 B] 入力値の境界

- **対象**: `isResponseUnavailableError`
- **不足内容**: 部分一致（前後に文言が付く）と大文字小文字違いが未テスト
- **根拠コード**: `types.ts:179` は `includes` なので `"save: response body unavailable"` は true、`"Response body unavailable"` は false になる。Go 側のエラーがラップされた場合の判定がこれに依存する
- **推奨テスト名**: `it("matches the backend message even when wrapped", ...)` / `it("is case-sensitive", ...)`
- **優先度**: 低

##### [観点 D] 組み合わせの境界

- **対象**: `hasUnconfirmedFile`
- **不足内容**: 次の組み合わせが未テスト。(1) `form-urlencoded` に kind=file の未確定行がある（`form-data` 以外は見ない）、(2) kind を file → text に戻したが `file` 参照が残っている行（止めない）、(3) `type: "file"` で `file` が undefined（止めない）、(4) `formData` が undefined
- **根拠コード**: `types.ts:53-63`。`FormRow.file` は kind を往復しても消えない設計（`types.ts:73` のコメント）なので、(2) は実際に起きる
- **推奨テスト名**: `it("ignores file references on rows whose kind is not file", ...)` / `it("only checks form-data rows", ...)`
- **優先度**: 中

##### [観点 H] 型ガード・バリデーション関数の境界

- **対象**: `isFormRowKind`
- **不足内容**: 直接のテストが無い（有効値・大文字・空文字・部分一致）。ほかの 3 つの型ガードは定数配列を `it.each` に渡しているが、`FORM_ROW_KINDS` だけはテストから使われていない
- **根拠コード**: `types.ts:252-254`。infrastructure の `fromWailsFormRow`（`http/client.ts:100`）がこの判定で未知 kind を text に寄せる
- **推奨テスト名**: `it.each(FORM_ROW_KINDS)("returns true for %s", ...)` と `it("returns false for FILE / '' / fil", ...)`（`isHttpMethod` などと同じ形にそろえる）
- **優先度**: 低

- **対象**: `isFormBodyType`
- **不足内容**: 全 body type に対する true/false の表が無い。`in` 演算子はプロトタイプチェーンも見るため、`"toString"` などでも true になる
- **根拠コード**: `types.ts:104` の `v in FORM_PAIR_FIELDS`。現状の呼び出し元は `isBodyType` で検証済みの値しか渡さないので実害は無いが、ガードの厳密さはテストで固定されていない
- **推奨テスト名**: `it.each(BODY_TYPES)("returns %s only for form bodies", ...)`
- **優先度**: 低

- 観点 C・E・F・G・I・J: 該当なし（純粋関数のみで、状態・例外・非同期・Wails 呼び出しを持たない）

---

### `frontend/src/domain/mqtt/topic.ts`

#### 対象関数・定数

| 名前 | ファイル:行 | 分岐/バリエーション数 | テスト済み? |
|---|---|---|---|
| compilePattern | topic.ts:1 | 1 | 済 |
| topicMatchesParts | topic.ts:5 | 4（`#` / 長さ不足 / `+` or 一致 / 長さ一致） | 済 |
| topicMatches | topic.ts:18 | 1 | 済 |

#### 不足テストケース

##### [観点 B] 入力値の境界

- **対象**: `topicMatches`
- **不足内容**: 途中に空レベルを含むトピック（`"a//b"` と `"a/+/b"`）、`"+"` と空文字トピックの照合が未テスト。末尾の空レベル（`"sensors/+"` 対 `"sensors/"`、`topic.test.ts:216`）と `"#"` 対空文字（`topic.test.ts:212`）は確認済み
- **根拠コード**: `topic.ts:19` は `split("/")` なので空レベルは `""` 要素になり、`topic.ts:12` の `+` はそれに一致する
- **推奨テスト名**: `it("matches an empty middle level with +", ...)` / `it("matches an empty topic with +", ...)`
- **優先度**: 低

- 観点 A・D・H・I: 不足なし（完全一致・`+`・`#`・複合パターン・長さ違いまで網羅されている）
- 観点 C・E・F・G・J: 該当なし（純粋関数）

---

### `frontend/src/domain/udp/types.ts`

#### 対象関数・定数

| 名前 | ファイル:行 | 分岐/バリエーション数 | テスト済み? |
|---|---|---|---|
| isValidNumericFieldValue | types.ts:63 | 空文字 + 10 型 + default | 部分的 |
| isPayloadEncoding | types.ts:153 | 2 | 済 |
| PAYLOAD_ENCODINGS / FIELD_TYPES / FIELD_TYPE_SIZES | types.ts:9, 25, 45 | - | 部分的（PAYLOAD_ENCODINGS のみ it.each で使用） |

#### 不足テストケース

##### [観点 B] 入力値の境界

- **対象**: `isValidNumericFieldValue`（float32 / float64）
- **不足内容**: 空白のみ `" "`、16 進 `"0x10"`、前後空白付き `" 1.5 "` が未テスト。いずれも true になる（Node で確認済み）
- **根拠コード**: 空文字の早期 return は `value === ""` だけ（`types.ts:67`）。float 系は `Number(value)`（`types.ts:108, 113`）で判定するため、`Number(" ") === 0`、`Number("0x10") === 16` を有効値として通す。送信時に backend 側の数値変換が同じ解釈をする保証は無い
- **推奨テスト名**: `it("float32: rejects whitespace-only input", ...)` など。現状の挙動を仕様とするなら、その旨を固定するテスト
- **優先度**: 中

- **対象**: `isValidNumericFieldValue`（int16 / int32 / int64）
- **不足内容**: 下限の 1 つ外側（`"-32769"`、`"-2147483649"`、`"-9223372036854775809"`）が未テスト。上限側と int8 の下限だけがテストされている
- **根拠コード**: `types.ts:91, 96, 102` の `n >= 下限` 比較。int64 は BigInt 経路（`types.ts:101`）で、他と比較方法が違う
- **推奨テスト名**: `it.each([["int16","-32769"],["int32","-2147483649"],["int64","-9223372036854775809"]])("%s: rejects below the minimum", ...)`
- **優先度**: 中

- **対象**: `isValidNumericFieldValue`（uint16 / uint32 / uint64）
- **不足内容**: uint16・uint32 の負値 `"-1"`、uint64 の小数 `"1.5"` と `"+5"` が未テスト
- **根拠コード**: `types.ts:72, 74, 76` の正規表現 `^\d+$`
- **推奨テスト名**: 既存の `"uint type + prefix rejection"` に uint64 と負値を追加
- **優先度**: 低

- **対象**: `isValidNumericFieldValue`（float32）
- **不足内容**: 上限ちょうど（`"3.4028234663852886e38"`）と正側の上限超過のすぐ外側が未テスト（`3.5e38` と `-3.4e38` のみ）
- **根拠コード**: `types.ts:110` の `Math.abs(n) <= 3.4028234663852886e38`
- **推奨テスト名**: `it("float32: accepts exactly FLT_MAX", ...)`
- **優先度**: 低

- 観点 H: 不足なし（`isPayloadEncoding` は `it.each(PAYLOAD_ENCODINGS)` で定数配列から生成しており、大文字・部分一致・空白も確認済み）
- 観点 E: 該当なし（`types.ts:79, 103` の BigInt の catch は直前の正規表現で到達しない）
- 観点 C・D・F・G・I・J: 該当なし（純粋関数）

---

### `frontend/src/application/http/collections.ts`

#### 対象関数・定数

| 名前 | ファイル:行 | 分岐/バリエーション数 | テスト済み? |
|---|---|---|---|
| findRequestById | collections.ts:18 | 5（root / collection 無し / 再帰 / request 無し / 不一致） | 部分的 |
| createCollectionsState | collections.ts:87 | - | 部分的 |
| └ isExpanded / setExpanded | collections.ts:100, 104 | 2 | 済 |
| └ pruneExpandedIds | collections.ts:109 | 3（再帰 / stale 無し / stale 有り） | 部分的 |
| └ refreshCollections | collections.ts:141 | 1 | 部分的（prune のみ検証） |
| └ createCollection / addFolder / addRequest | collections.ts:150, 172, 184 | 成功 / 失敗 | 部分的（createCollection 失敗・addRequest 成功のみ） |
| └ deleteCollection / renameCollection / renameItem / deleteItem / moveItem / moveItemToSidebar | collections.ts:158-256 | 成功 / 失敗 | 部分的（deleteItem 失敗のみ） |
| └ moveSidebarEntry | collections.ts:236 | 成功 / 失敗 | 部分的（失敗のみ） |
| └ patchRequest | collections.ts:258 | 2（__root__ / 通常）＋再帰 | 未テスト |

#### 不足テストケース

##### [観点 A] 正常系の網羅

- **対象**: `patchRequest`
- **不足内容**: テストが無い。`__root__` 側と通常コレクション側の両分岐、フォルダ内のネストしたリクエストの更新、`name` を backend の値のまま保持することが未検証
- **根拠コード**: `collections.ts:258-288`。`name` の保持は `collections.ts:264, 278`。`request.ts:338` の `afterSave` から呼ばれ、保存後にサイドバーのツリーと編集中の内容を一致させる役割を持つ
- **推奨テスト名**: `it("patches a nested request but keeps its backend name", ...)` / `it("patches a request directly under the sidebar root", ...)`
- **優先度**: 高

- **対象**: `refreshCollections`
- **不足内容**: 成功後に `collections` / `rootItems` / `sidebarLayout` へ API の値が入ることと、`collectionsLoaded()` が true になることが未検証
- **根拠コード**: `collections.ts:141-148`
- **推奨テスト名**: `it("loads collections, root items and layout and marks them loaded", ...)`
- **優先度**: 中

- **対象**: 変更系操作（`deleteCollection`・`renameCollection`・`addFolder`・`renameItem`・`moveItem`・`moveItemToSidebar`・`moveSidebarEntry`）
- **不足内容**: API に渡す引数と、成功後の再読み込みが未検証。`moveSidebarEntry` だけはレイアウトしか読み直さない（コレクションは読み直さない）違いも固定されていない
- **根拠コード**: `collections.ts:158-256`。再読み込みの範囲の違いは `collections.ts:243`（`refreshSidebarLayout`）と他の `refreshCollections`
- **推奨テスト名**: `it.each([...])("%s calls the api and refreshes", ...)` / `it("moveSidebarEntry only refreshes the layout", ...)`
- **優先度**: 中

##### [観点 B] 入力値の境界

- **対象**: `findRequestById`
- **不足内容**: (1) `__root__` 側で同じ id のフォルダがある、(2) `type: "request"` だが `request` フィールドが無い要素（探索を続けて子を見る）、が未テスト
- **根拠コード**: `collections.ts:26`（root 側の type 判定）、`collections.ts:36`（`&& item.request`）
- **推奨テスト名**: `it("ignores a root folder with the same id", ...)`
- **優先度**: 低

##### [観点 C] 状態の境界

- **対象**: `pruneExpandedIds`
- **不足内容**: 2 階層以上のフォルダ（再帰）の id が stale と誤判定されないことが未テスト。既存テストは 1 階層のフォルダだけ
- **根拠コード**: `collections.ts:111-116` の `walk` の再帰
- **推奨テスト名**: `it("keeps expanded ids of deeply nested folders", ...)`
- **優先度**: 低

##### [観点 E] 異常系の境界

- **対象**: `createCollection` など戻り値を持つ操作 / `refreshCollections`
- **不足内容**: (1) API は成功したが直後の `refreshCollections` が失敗した場合、作成済みなのに「Failed to create collection」を通知して reject すること、(2) `refreshCollections` 自体はガードされておらず reject がそのまま伝わること、が未テスト
- **根拠コード**: `collections.ts:151-155`（API 成功後の `await refreshCollections()` も `notifyOnError` の中）、`collections.ts:141`（ガード無し）
- **推奨テスト名**: `it("reports failure when the refresh after create fails", ...)` / `it("propagates a getCollections rejection from refreshCollections", ...)`
- **優先度**: 中

- 観点 D・F・G・H: 該当なし（組み合わせ入力・並列 Promise・Wails 直接呼び出し・型ガードを持たない）
- 観点 I・J: 不足なし（展開状態は `stored()` まで確認し、各テストで API・ストレージを作り直している）

---

### `frontend/src/application/http/request.ts`

#### 対象関数・定数

| 名前 | ファイル:行 | 分岐/バリエーション数 | テスト済み? |
|---|---|---|---|
| createRequestState | request.ts:81 | - | 部分的 |
| └ bodyContent / setBodyContent | request.ts:127, 136 | 3（file / json 雛形 / その他） | 部分的（file 種別が未テスト） |
| └ formBodyType / formPairs / setFormPairs | request.ts:148-167 | 2 | 済 |
| └ replaceResponse | request.ts:172 | 4（prev 無し / 同一 / 非切り詰め / saveState≠idle） | 部分的 |
| └ sendRequest | request.ts:190 | 3（未確定ファイル / 成功 / 失敗） | 部分的（失敗が未テスト） |
| └ pickFile | request.ts:244 | 2 | 未テスト |
| └ cancelRequest | request.ts:253 | 2 | 済 |
| └ saveResponseBody | request.ts:261 | 6（cur 無し / バイナリ / saved / キャンセル / unavailable / その他エラー） | 部分的 |
| └ loadRequest | request.ts:282 | 3（保存 / 切替判定 / 既定値補完） | 部分的 |
| └ newRequest | request.ts:302 | 1 | 部分的（response のクリアのみ） |
| └ saveCurrentRequest / clearSaveError | request.ts:319, 375 | 3（id 無し / 成功 / 失敗） | 未テスト |
| createAutoSaveEffect | request.ts:387 | 3（id 無し / デバウンス / 失敗通知） | 未テスト |

#### 不足テストケース

##### [観点 A] 正常系の網羅

- **対象**: `createAutoSaveEffect`
- **不足内容**: テストが無い。(1) 編集後 `debounceMs` で `saveCurrentRequest` が 1 回呼ばれる、(2) デバウンス中の連続編集は最後の 1 回にまとまる、(3) `activeRequestId` が null なら保存しない、(4) 保存失敗時に `key: "http-autosave"` 付きで通知する、が未検証
- **根拠コード**: `request.ts:393-416`（`403` の早期 return、`405-413` の version 判定、`409-411` の通知、`415` の onCleanup）
- **推奨テスト名**: `it("saves once after the debounce window", ...)` / `it("does not save an unsaved (new) request", ...)` / `it("notifies auto-save failures with a dedup key", ...)`（`vi.useFakeTimers()` を使う）
- **優先度**: 高

- **対象**: `saveCurrentRequest` / `clearSaveError`
- **不足内容**: 直接のテストが無い。(1) id か collectionId が無ければ `updateRequest` を呼ばない、(2) 成功時に `saveError` を null にして `afterSave(colId, req)` を呼ぶ、(3) 失敗時に `saveError` にメッセージを入れて再送出する、が未検証
- **根拠コード**: `request.ts:319-344`（`322`、`337-338`、`339-343`）
- **推奨テスト名**: `it("records saveError and rethrows when updateRequest fails", ...)` / `it("calls afterSave with the saved request", ...)`
- **優先度**: 高

- **対象**: `pickFile`
- **不足内容**: テストが無い。成功時に参照を返すことと、失敗時に `"Failed to open file picker"` を通知して undefined を返すことが未検証
- **根拠コード**: `request.ts:244-251`
- **推奨テスト名**: `it("notifies and returns undefined when the picker fails", ...)`
- **優先度**: 中

##### [観点 B] 入力値の境界

- **対象**: `loadRequest` / `bodyContent` / `setBodyContent`
- **不足内容**: `settings` / `doc` が undefined のリクエストを読んだときの既定値補完、`type: "file"` での `bodyContent()` が `""` になり `setBodyContent` が何もしないこと、が未テスト
- **根拠コード**: `request.ts:296-297`（`?? { ...DEFAULT_SETTINGS }`、`?? ""`）、`request.ts:123, 138`
- **推奨テスト名**: `it("fills default settings and doc for legacy requests", ...)` / `it("ignores body content writes for file bodies", ...)`
- **優先度**: 低

##### [観点 C] 状態の境界

- **対象**: `replaceResponse`
- **不足内容**: `responseSaveState` が `"unavailable"` のときに破棄しないこと、`discardResponseBody` が reject しても logger に記録するだけで未処理の reject にならないこと、が未テスト
- **根拠コード**: `request.ts:178`（`=== "idle"`）、`request.ts:180-184`（`.catch` で logger.error）
- **推奨テスト名**: `it("does not discard a body the backend already reclaimed", ...)` / `it("logs a failed discard", ...)`
- **優先度**: 低

##### [観点 D] 組み合わせの境界

- **対象**: `loadRequest`
- **不足内容**: 別のリクエストへ切り替えるとき、切り替え前のリクエスト（前の id・collectionId・編集内容）で `updateRequest` が呼ばれることが未検証。同じ id で collectionId だけ違う場合にレスポンスを破棄する分岐も未テスト
- **根拠コード**: `request.ts:283`（`saveCurrentRequest` の signal 読み取りは最初の `await` より前にあるので、切り替え前の値で保存される）、`request.ts:287` の `||`
- **推奨テスト名**: `it("saves the previous request before switching", ...)` / `it("clears the response when only the collection differs", ...)`
- **優先度**: 中

##### [観点 E] 異常系の境界

- **対象**: `sendRequest`
- **不足内容**: `api.sendRequest` が reject したときの経路が未テスト。`statusCode: 0` とエラー文言を持つレスポンスが表示される、Error 以外の値は `String(err)` になる、`loading()` が false に戻る、が未検証。テスト用の `makeApi` の `sendRequest` は resolve しかしない
- **根拠コード**: `request.ts:227-240`（catch と finally）。テスト側は `request.test.ts:71-77`
- **推奨テスト名**: `it("shows an error response and stops loading when sending fails", ...)`
- **優先度**: 高

- **対象**: `saveResponseBody`
- **不足内容**: (1) 切り詰めレスポンスで `unavailable` 以外のエラー → `"Failed to save response"` を通知、(2) 非切り詰め（バイナリ）の保存失敗は、文言が unavailable でも通知する、(3) 保存中に表示レスポンスが入れ替わったら状態を更新しない、(4) レスポンスが無ければ何もしない、が未テスト
- **根拠コード**: `request.ts:263`、`271`（`current() === cur`）、`274-278`（`resp.bodyTruncated &&` で分岐）
- **推奨テスト名**: `it("notifies non-reclaim save errors", ...)` / `it("does not mark saved when the response changed during the save", ...)`
- **優先度**: 中

- **対象**: `loadRequest` / `newRequest`
- **不足内容**: 切り替え前の保存が失敗したときに `"Failed to save request"` を通知することが未テスト
- **根拠コード**: `request.ts:283-285, 303-305`
- **推奨テスト名**: `it("notifies when saving the previous request fails on switch", ...)`
- **優先度**: 中

##### [観点 F] 非同期・時間の境界

- **対象**: `sendRequest` × `loadRequest` / `newRequest`
- **不足内容**: 送信中に別のリクエストへ切り替えた後で応答が返ったときの挙動が未テスト。現状はアクティブなリクエストに関係なく `replaceResponse` するので、切り替え先に前のリクエストの応答が表示される
- **根拠コード**: `request.ts:220`（応答時にアクティブ id を確認していない）と、`request.ts:286-289` の「別のリクエストへ切り替えたら前のレスポンスは表示しない」という意図。意図どおりかどうかの判断を含めてテストで固定したい
- **推奨テスト名**: `it("does not show a late response after switching to another request", ...)`
- **優先度**: 中

- **対象**: `cancelRequest`
- **不足内容**: 複数の実行中リクエストのうち一部のキャンセルが reject した場合（`Promise.all` が reject し、呼び出し側に伝わる）が未テスト
- **根拠コード**: `request.ts:256`
- **推奨テスト名**: `it("rejects when one of the cancellations fails", ...)`
- **優先度**: 低

##### [観点 I] 戻り値・副作用の検証

- **対象**: `sendRequest`
- **不足内容**: backend に渡すリクエストのうち `id` と `body.file.token` しか検証していない。`name: ""`・method・url・headers・params・auth・settings・doc が signal の値どおりに組み立てられることが未検証
- **根拠コード**: `request.ts:207-219`
- **推奨テスト名**: `it("sends the current editor state", ...)` で `sent[0]` を `toEqual`
- **優先度**: 中

- 観点 G・H: 該当なし（Wails を直接呼ばず、型ガードも持たない）
- 観点 J: 不足なし（`sendRequest` は resolve のタイミングを制御できる偽 API で、各テストで作り直している）。ただし `noopLogger`（`request.test.ts:17`）は呼び出しを記録しないため、logger への出力は検証できない

---

### `frontend/src/application/mqtt/broker-url.ts`

#### 対象関数・定数

| 名前 | ファイル:行 | 分岐/バリエーション数 | テスト済み? |
|---|---|---|---|
| defaultPort | broker-url.ts:9 | 6（5 スキーム + 既定） | 済 |
| parseBrokerUrl | broker-url.ts:13 | 3（不一致 / ポート有 / ポート無） | 部分的 |
| composeBrokerUrl | broker-url.ts:27 | 1 | 済 |

#### 不足テストケース

##### [観点 B] 入力値の境界

- **対象**: `parseBrokerUrl`
- **不足内容**: 範囲外ポート（`"mqtt://h:99999"`、`":0"`）がそのまま返ること、パスを含む URL（`"mqtt://host/path"` → host が `"host/path"`）、末尾コロンだけの `"mqtt://h:"` がフォールバックになること、が未テスト（前 2 つは Node で確認済み）
- **根拠コード**: `broker-url.ts:18` の正規表現（ポートは `\d+`、ホストは `[^:]+`）
- **推奨テスト名**: `it("does not validate the port range", ...)` / `it("treats a path as part of the host", ...)`
- **優先度**: 低

##### [観点 D] 組み合わせの境界

- **対象**: `parseBrokerUrl`
- **不足内容**: 大文字スキーム（`"MQTT://host:1883"`）がフォールバックになることが未テスト
- **根拠コード**: `broker-url.ts:18` は大文字小文字を区別する
- **推奨テスト名**: `it("falls back for an upper-case scheme", ...)`
- **優先度**: 低

- 観点 C・E・F・G・H・I・J: 該当なし（純粋関数。不一致は例外ではなくフォールバック値を返す）

---

### `frontend/src/application/mqtt/profile-validation.ts`

#### 対象関数・定数

| 名前 | ファイル:行 | 分岐/バリエーション数 | テスト済み? |
|---|---|---|---|
| isValidBrokerPort | profile-validation.ts:11 | 3（整数 / 下限 / 上限） | 部分的 |
| isValidProfileDraft | profile-validation.ts:16 | 3（name / host / port） | 済 |

#### 不足テストケース

##### [観点 B] 入力値の境界

- **対象**: `isValidBrokerPort`
- **不足内容**: `Number()` の変換で通ってしまう入力が未テスト。「検証関数が受け入れる値」と「画面から入力できる値」は範囲が違うので分けて記す
  - **検証関数が受け入れる値**: `"1e3"`、`"1883.0"`、`"0x50"`、`" 1883 "` はいずれも true になる（Node で確認済み）
  - **画面から入力できる値**: ポート欄は `type="number"`（`broker-settings-dialog.tsx:135`）。HTML 仕様の値の正規化では、浮動小数点数として正しい `"1e3"` と `"1883.0"` は `value` にそのまま残り、`"0x50"` と `" 1883 "` は空文字になって `isValidBrokerPort` で弾かれる。したがって画面から問題を起こせるのは前の 2 つ（WebView2 での実機確認はしていない）
- **根拠コード**: `profile-validation.ts:12-13`。ダイアログはこの判定を通ったポート文字列をそのまま `composeBrokerUrl` に渡して保存する（`presentation/components/mqtt/broker-settings-dialog.tsx:57-60`）。`"mqtt://host:1e3"` と `"mqtt://host:1883.0"` は `parseBrokerUrl` の `\d+`（`broker-url.ts:18`）に合わないため、次にダイアログを開くと `mqtt://localhost:1883` として表示され、そのまま保存するとブローカー設定が書き換わる
- **推奨テスト名**: `it("rejects exponent and decimal port strings", ...)`、または `it("round-trips a valid port through compose/parse", ...)`。検証関数は画面以外からも呼べるので、hex・前後空白も同じテストで弾いておく
- **優先度**: 高（画面から到達できる `"1e3"` / `"1883.0"` が根拠。hex・前後空白だけなら低）

##### [観点 D] 組み合わせの境界

- **対象**: `isValidProfileDraft`
- **不足内容**: host にコロン（IPv6 リテラル `"::1"`）や前後の空白（`" host "`）を含む場合も true になることが未テスト。空白チェックは `trim()` だけで、保存値はトリムされない
- **根拠コード**: `profile-validation.ts:19` は `trim().length > 0` のみ。`broker-url.test.ts:42` で `[::1]` は parse できないことが確認されている
- **推奨テスト名**: `it("rejects hosts that parseBrokerUrl cannot read back", ...)`
- **優先度**: 中

- 観点 C・E・F・G・H・I・J: 該当なし（純粋関数）

---

### `frontend/src/application/mqtt/messages.ts`

#### 対象関数・定数

| 名前 | ファイル:行 | 分岐/バリエーション数 | テスト済み? |
|---|---|---|---|
| collectFilterTopics | messages.ts:14 | 2（ワイルドカード有無） | 済 |
| filterMessagesByTopic | messages.ts:35 | 3（空 / ワイルドカード / 完全一致） | 済 |
| createMessagesState | messages.ts:46 | 5（追従 effect / 3 setter / 接続無し） | 未テスト |

#### 不足テストケース

##### [観点 A] 正常系の網羅

- **対象**: `createMessagesState`
- **不足内容**: テストが無い。(1) `autoFollow` が true のとき末尾メッセージを `selectedMessage` にする、(2) 既に末尾を選択中なら `updateConnection` を呼ばない、(3) アクティブ接続が無いと各 setter が何もしない、(4) `setAutoFollow` に関数を渡すと現在値から計算する、(5) `clearMessages` が `messages` と `selectedMessage` を両方消す、が未検証
- **根拠コード**: `messages.ts:58-72`（`66` の untrack 比較）、`75-76`、`83`、`90-94`
- **推奨テスト名**: `it("follows the newest message while autoFollow is on", ...)` / `it("clears the selection together with the messages", ...)`
- **優先度**: 中

- 観点 B・D・H・I: 不足なし（空購読・`#`・`+`・重複排除・参照同一性まで確認済み）
- 観点 C・E・F・G・J: 該当なし（純粋関数部分は状態・例外・非同期を持たない。`createMessagesState` の状態は観点 A にまとめた）

---

### `frontend/src/application/mqtt/connections.ts`

#### 対象関数・定数

| 名前 | ファイル:行 | 分岐/バリエーション数 | テスト済み? |
|---|---|---|---|
| createConnectionsState | connections.ts:123 | - | 部分的 |
| └ restore | connections.ts:312 | 6（1 回限り / 取得失敗 / online 復元 / offline 補完 / 最後のプロファイル online・offline） | 部分的（取得失敗が未テスト） |
| └ flushMessages（mqtt:message） | connections.ts:160, 240 | 8（グループ化 / scan / 一致 / muted / 不一致 / topics 上限 / messages 上限 / buffer 上限） | 未テスト |
| └ mqtt:connected / disconnected / connection-lost / connection-failed | connections.ts:250-296 | 各 2（online / offline） | 未テスト |
| └ 最後のプロファイルの永続化 effect | connections.ts:360 | 2 | 未テスト |
| └ updateConnectionBroker | connections.ts:369 | 3 | 未テスト |
| └ createOfflineConnection | connections.ts:380 | 1 | 未テスト |
| └ handleConnect | connections.ts:393 | 3（プロファイル無し / 成功 / 失敗） | 部分的（失敗のみ） |
| └ handleDisconnect | connections.ts:425 | 3 | 未テスト |
| └ handleReconnect | connections.ts:444 | 6 | 未テスト |
| └ closeConnection | connections.ts:486 | 3 | 未テスト |
| └ switchConnection / updateConnection | connections.ts:506, 147 | 2 | 未テスト |

#### 不足テストケース

##### [観点 A] 正常系の網羅

- **対象**: `mqtt:message` の受信とバッチ反映（`flushMessages`）
- **不足内容**: テストが無い。(1) 接続ごとにまとめて反映する、(2) 完全一致または `patternParts` に一致する購読のメッセージだけ追加する、(3) `muted` の購読は追加しない、(4) `payloadBase64` 未指定は false、`timestamp` は Date に変換する、が未検証
- **根拠コード**: `connections.ts:160-237`（`191-196`、`201`、`203`）、`240-248`。テストの `noopEvent`（`connections.test.ts:14`）はハンドラを捨てるため、この経路に到達できない。node 環境には `requestAnimationFrame` が無いので、`vi.stubGlobal` で差し替えが必要（`connections.ts:246`）
- **推奨テスト名**: `it("appends only messages matching an unmuted subscription", ...)`
- **優先度**: 高

- **対象**: `handleReconnect`
- **不足内容**: テストが無い。(1) online なら先に disconnect し、その失敗は無視する、(2) 新しい接続 id に購読・メッセージを引き継いで旧キーを消す、(3) アクティブならアクティブ id も移す、(4) 購読ごとに再 subscribe し、失敗はトピック名付きで通知して続行する、(5) connect 失敗で `"Failed to reconnect"`、が未検証
- **根拠コード**: `connections.ts:444-484`
- **推奨テスト名**: `it("moves state to the new connection id and re-subscribes", ...)` / `it("keeps re-subscribing after one topic fails", ...)`
- **優先度**: 高

- **対象**: 接続ライフサイクルイベント（connected / disconnected / connection-lost / connection-failed）
- **不足内容**: online の接続だけ `connected` を更新し offline は変えないこと、lost・failed で `key: connectionId` 付きの通知を出すこと、disconnected・failed で `isScanning` を false にすること、が未検証
- **根拠コード**: `connections.ts:250-296`
- **推奨テスト名**: `it("marks the connection lost and notifies once per connection", ...)`
- **優先度**: 中

- **対象**: `handleConnect`（成功時）
- **不足内容**: 同じプロファイルの offline タブを消して新しい接続 id で online タブを作り、アクティブにすることが未検証。存在しない profileId では API を呼ばないことも未テスト
- **根拠コード**: `connections.ts:394-411`
- **推奨テスト名**: `it("replaces the offline tab with the new online connection", ...)`
- **優先度**: 中

- **対象**: `handleDisconnect`
- **不足内容**: 引数省略時にアクティブ接続を使うこと、disconnect が失敗しても通知したうえで `connected: false` にすることが未検証
- **根拠コード**: `connections.ts:426`、`431-441`
- **推奨テスト名**: `it("marks the connection disconnected even when the backend call fails", ...)`
- **優先度**: 中

- **対象**: `closeConnection`
- **不足内容**: online かつ接続中のときだけ disconnect を呼ぶこと、閉じたのがアクティブなら残りの先頭（無ければ null）をアクティブにすることが未検証
- **根拠コード**: `connections.ts:488`、`500-503`
- **推奨テスト名**: `it("activates the first remaining tab after closing the active one", ...)`
- **優先度**: 中

##### [観点 B] 入力値の境界

- **対象**: `flushMessages` の上限処理
- **不足内容**: `maxMessages` ちょうど・超過時に古い順に捨てること、`isScanning` 中のトピック一覧が `maxTopics` を超えたら古い順に捨てて Set からも消すこと、受信バッファが 5000 件で頭打ちになること、が未テスト
- **根拠コード**: `connections.ts:182-188`、`209-218`、`221-226`、`241`
- **推奨テスト名**: `it("keeps at most maxMessages, dropping the oldest", ...)` / `it("caps broker topics while scanning", ...)`
- **優先度**: 高

##### [観点 C] 状態の境界

- **対象**: 最後のプロファイルの永続化 effect
- **不足内容**: アクティブ接続があれば `saveLastProfileId(profileId)`、無くなれば `removeLastProfileId()` を呼ぶことが未検証
- **根拠コード**: `connections.ts:360-367`
- **推奨テスト名**: `it("forgets the last profile when no tab is active", ...)`
- **優先度**: 低

- **対象**: `createOfflineConnection`
- **不足内容**: 同じプロファイルの既存エントリ（online を含む）を消してから offline タブを作ることが未検証
- **根拠コード**: `connections.ts:384-386`
- **推奨テスト名**: `it("replaces existing tabs of the same profile", ...)`
- **優先度**: 低

##### [観点 D] 組み合わせの境界

- **対象**: `updateConnectionBroker`
- **不足内容**: online かつ接続中なら何もしない、offline または未接続の online なら profile を更新して `saveProfile` を呼ぶ、の組み合わせが未テスト。`saveProfile` の Promise は await も catch もしていない
- **根拠コード**: `connections.ts:371`、`377`
- **推奨テスト名**: `it("does not change the broker of a connected tab", ...)`
- **優先度**: 中

- **対象**: `flushMessages` の購読の照合（重なる購読と `muted`）
- **不足内容**: 1 つのメッセージに複数の購読が一致する場合が未テスト。`find` で最初に一致した購読だけを見て `muted` を判定するため、先に並ぶ購読が muted だと、後ろに muted でない購読（例: muted の `sensors/#` と通常の `sensors/temp`）が一致しても追加されない。意図した仕様か不具合かも決まっていない
- **根拠コード**: `connections.ts:191-196`（`state.subscriptions.find(...)` と `matchingSub && !matchingSub.muted`）。観点 A と同じく、`noopEvent` ではこの経路に到達できない
- **推奨テスト名**: `it("keeps a message when any matching subscription is unmuted", ...)`（仕様を決めてから追加する。現状の挙動を固定するなら `it("uses the first matching subscription's muted flag", ...)`）
- **優先度**: 中

##### [観点 E] 異常系の境界

- **対象**: `restore`
- **不足内容**: `getConnections` が reject しても logger に記録し、全プロファイルを offline タブとして復元することが未テスト。保存済みプロファイル id がどこにも無い場合にアクティブが null のままになることも未テスト
- **根拠コード**: `connections.ts:317-321`、`349-356`
- **推奨テスト名**: `it("falls back to offline tabs when getConnections fails", ...)`
- **優先度**: 中

##### [観点 I] 戻り値・副作用の検証

- **対象**: テストダブル全般
- **不足内容**: `makePersistence` と `noopEvent` が呼び出しを記録しない（`connections.test.ts:14, 49-55`）ため、永続化とイベント購読解除（`onCleanup`）の副作用をどのテストも確認できない
- **根拠コード**: `connections.ts:298-304`（5 つのイベント購読を解除）、`360-367`
- **推奨テスト名**: `vi.fn` を使うダブルに差し替え、`it("unsubscribes every event on dispose", ...)` を追加
- **優先度**: 中

- 観点 F: 観点 A・B にまとめた（`requestAnimationFrame` によるバッチ反映）
- 観点 G・H: 該当なし（API はポート経由で注入され、型ガードを持たない）
- 観点 J: 不足なし（各テストで state・API を作り直し、`dispose` している）

---

### `frontend/src/application/mqtt/presets.ts`

#### 対象関数・定数

| 名前 | ファイル:行 | 分岐/バリエーション数 | テスト済み? |
|---|---|---|---|
| createPresetsState | presets.ts:21 | - | 部分的 |
| └ selectPreset | presets.ts:40 | 2（存在 / 不在） | 部分的 |
| └ updateDraft | presets.ts:49 | 2（選択有無） | 済 |
| └ savePreset | presets.ts:55 | 1 | 未テスト |
| └ updatePreset | presets.ts:67 | 1 | 部分的（updateDraft 経由） |
| └ removePreset | presets.ts:78 | 2（選択中 / 非選択） | 部分的 |
| └ addPreset | presets.ts:87 | 2（name 有無） | 部分的 |
| └ reorderPresets | presets.ts:107 | 2（範囲内 / 範囲外） | 未テスト |

#### 不足テストケース

##### [観点 A] 正常系の網羅

- **対象**: `savePreset`
- **不足内容**: テストが無い。採番した id で末尾に追加し、そのプリセットを選択して draft に読み込み、ストレージへ保存することが未検証
- **根拠コード**: `presets.ts:55-65`
- **推奨テスト名**: `it("saves a new preset, selects it and loads it into the draft", ...)`
- **優先度**: 中

##### [観点 B] 入力値の境界

- **対象**: `addPreset` / `reorderPresets`
- **不足内容**: name 省略時の `"no name"`、範囲外インデックスで並べ替えず保存もしないこと、が未テスト
- **根拠コード**: `presets.ts:91`、`presets.ts:110`
- **推奨テスト名**: `it("names an added preset 'no name' by default", ...)` / `it("ignores out-of-range reorder", ...)`
- **優先度**: 低

##### [観点 D] 組み合わせの境界

- **対象**: `selectPreset` × `updateDraft`
- **不足内容**: 存在しない id を選択すると `selectedPresetId` にはその id が入り、draft は変わらないこと、その後の `updateDraft` が何も更新しないのに `storage.save` を呼ぶこと、が未テスト
- **根拠コード**: `presets.ts:41-42`（id の存在確認前に選択状態を更新）、`presets.ts:52, 71-75`
- **推奨テスト名**: `it("does not select a missing preset", ...)`（現状の挙動を仕様とするかの判断を含む）
- **優先度**: 低

##### [観点 I] 戻り値・副作用の検証

- **対象**: `storage.save`
- **不足内容**: `save` を `vi.fn` にしている（`presets.test.ts:11`）が、どのテストも呼び出しや保存内容を確認していない。追加・更新・削除のたびに永続化されることが未検証
- **根拠コード**: `presets.ts:60, 73, 81, 99, 111`
- **推奨テスト名**: 既存テストに `expect(storage.save).toHaveBeenLastCalledWith(...)` を追加
- **優先度**: 中

- 観点 C: 不足なし（初期状態・上書き・選択中の削除を確認済み）
- 観点 E・F・G・H・J: 該当なし（同期処理のみで、例外・Wails・型ガードを持たない）

---

### `frontend/src/application/mqtt/profiles.ts`

#### 対象関数・定数

| 名前 | ファイル:行 | 分岐/バリエーション数 | テスト済み? |
|---|---|---|---|
| createEmptyProfile | profiles.ts:8 | 1 | 部分的（id のみ） |
| createProfilesState | profiles.ts:27 | - | 部分的 |
| └ loadProfiles | profiles.ts:33 | 1 | 済 |
| └ saveProfile | profiles.ts:41 | 2（新規 / 更新） | 済 |
| └ deleteProfile | profiles.ts:55 | 1 | 済 |
| └ reorderProfiles | profiles.ts:64 | 2（範囲内 / 範囲外） | 部分的 |

#### 不足テストケース

##### [観点 B] 入力値の境界

- **対象**: `reorderProfiles`
- **不足内容**: 範囲外インデックスで並べ替えず保存もしないことが未テスト（udp/targets には同じテストがある）
- **根拠コード**: `profiles.ts:67`
- **推奨テスト名**: `it("does not save when the indices are out of range", ...)`
- **優先度**: 低

##### [観点 E] 異常系の境界

- **対象**: `saveProfile` / `deleteProfile` / `loadProfiles`
- **不足内容**: API が reject したとき、state と並び順を変えずに reject を伝えることが未テスト（ガードは無い）
- **根拠コード**: `profiles.ts:34, 42, 56`（いずれも state 更新より前に await）
- **推奨テスト名**: `it("leaves the list untouched when saving fails", ...)`
- **優先度**: 中

##### [観点 I] 戻り値・副作用の検証

- **対象**: `createEmptyProfile`
- **不足内容**: id 以外（`broker: "mqtt://localhost:1883"` など）が未検証
- **根拠コード**: `profiles.ts:9-17`
- **推奨テスト名**: `expect(createEmptyProfile()).toEqual({...})`
- **優先度**: 低

- 観点 C: 不足なし（空・上書き・削除後を確認済み）
- 観点 D・F・G・H・J: 該当なし

---

### `frontend/src/application/openapi/editor.ts`

#### 対象関数・定数

| 名前 | ファイル:行 | 分岐/バリエーション数 | テスト済み? |
|---|---|---|---|
| createEditorState | editor.ts:7 | - | 部分的 |
| └ updateContent | editor.ts:19 | 3（デバウンス / ok / ng） | 部分的（dirty のみ） |
| └ loadContent | editor.ts:34 | 3（タイマー取消 / ok / ng） | 部分的（dirty のみ） |
| └ markSaved / isDirty | editor.ts:52, 15 | 1 | 済 |
| └ togglePreview | editor.ts:56 | 1 | 未テスト |

#### 不足テストケース

##### [観点 A] 正常系の網羅

- **対象**: `loadContent` / `updateContent` のパース結果
- **不足内容**: `ok: true` で `parsedSpec` に spec が入り `parseErrors` が空になること、`ok: false` で `parsedSpec` が null になり errors が入ること、が未検証。全テストが常に成功する `okParse` を使い、`parsedSpec()` / `parseErrors()` を一度も見ていない
- **根拠コード**: `editor.ts:24-30`、`41-48`
- **推奨テスト名**: `it("exposes parse errors and clears the spec on failure", ...)`
- **優先度**: 中

- **対象**: `togglePreview` / `isPreviewing`
- **不足内容**: 初期値 true と切り替えが未テスト
- **根拠コード**: `editor.ts:13, 56-58`
- **推奨テスト名**: `it("starts in preview mode and toggles", ...)`
- **優先度**: 低

##### [観点 F] 非同期・時間の境界

- **対象**: `updateContent` のデバウンス / `loadContent` による取消
- **不足内容**: (1) 500ms 経過まで `onParse` を呼ばない、(2) 連続編集では最後のテキストで 1 回だけパースする、(3) デバウンス中に `loadContent` すると保留中のパースが取り消され、古い内容の結果で上書きされない、が未テスト
- **根拠コード**: `editor.ts:21-22`（前のタイマーを消す）、`editor.ts:35-38`（loadContent がタイマーを消す）
- **推奨テスト名**: `it("parses only the last edit after the debounce", ...)` / `it("cancels a pending parse when a file is loaded", ...)`
- **優先度**: 中

##### [観点 J] テスト構造の整合性

- **対象**: `editor.test.ts:19-55`
- **不足内容**: 実タイマーのまま `updateContent` を呼ぶテストが、500ms 後に発火するタイマーを残して終わる。現状は捨てた state に対して `okParse` が呼ばれるだけで結果に影響しないが、パース関数に spy を入れると別テストの呼び出しが混ざる
- **根拠コード**: `editor.ts:22` の `setTimeout`
- **推奨テスト名**: ファイル全体で `beforeEach(() => vi.useFakeTimers())` にそろえる
- **優先度**: 低

- 観点 C: 不足なし（読み込み・編集・保存・再読み込みの dirty 遷移を確認済み）
- 観点 B・D・E・G・H・I: 該当なし（パース関数は注入され、本体は例外を投げない）

---

### `frontend/src/application/openapi/files.ts`

#### 対象関数・定数

| 名前 | ファイル:行 | 分岐/バリエーション数 | テスト済み? |
|---|---|---|---|
| createFilesState | files.ts:17 | - | 部分的 |
| └ refreshRecents | files.ts:21 | 1 | 済 |
| └ removeFile | files.ts:25 | 2（アクティブ一致 / 不一致） | 済 |
| └ moveFile | files.ts:32 | 2（成功 / 失敗） | 済 |

#### 不足テストケース

##### [観点 D] 組み合わせの境界

- **対象**: `removeFile`
- **不足内容**: アクティブ文書が `kind: "untitled"` のときは消さないことが未テスト
- **根拠コード**: `files.ts:29` の `doc?.kind === "file"`
- **推奨テスト名**: `it("keeps an untitled active document", ...)`
- **優先度**: 低

##### [観点 E] 異常系の境界

- **対象**: `removeFile` / `refreshRecents`
- **不足内容**: `moveFile` と違いガードされていない。`removeRecent` が reject するとアクティブ文書を消さずに reject を伝えること、`getRecents` の reject が伝わること、が未テスト
- **根拠コード**: `files.ts:22, 26-27`（`runGuarded` 無し）と `files.ts:33`（`runGuarded` 有り）の違い
- **推奨テスト名**: `it("propagates a removeRecent failure without clearing the active doc", ...)`
- **優先度**: 中

- 観点 C・I: 不足なし（削除後の一覧とアクティブ文書を確認済み）
- 観点 B・F・G・H・J: 該当なし

---

### `frontend/src/application/udp/field-validation.ts`

#### 対象関数・定数

| 名前 | ファイル:行 | 分岐/バリエーション数 | テスト済み? |
|---|---|---|---|
| isValidAscii | field-validation.ts:35 | 2 | 済 |
| isValidHex | field-validation.ts:39 | 3（奇数長 / 非 hex / 正常） | 済 |
| hexByteCount | field-validation.ts:45 | 1 | 済 |
| isVarLengthFieldType | field-validation.ts:50 | 2 | 済 |
| isValidFieldValue | field-validation.ts:54 | 3 | 済 |
| fieldByteCount | field-validation.ts:60 | 2 | 済 |
| fieldByteCountStatus | field-validation.ts:65 | 3 | 済 |
| fieldByteCountInfo | field-validation.ts:75 | 6 | 済 |
| fieldValueKind | field-validation.ts:98 | 5 | 済 |
| totalFieldBytes | field-validation.ts:107 | 2 | 済 |

#### 不足テストケース

##### [観点 D] 組み合わせの境界

- **対象**: `fieldByteCountStatus`
- **不足内容**: bytes 型で値のバイト数が `length` とちょうど等しい場合（`ok`）が未テスト。string 型では `"abcd"`/4 を確認済み
- **根拠コード**: `field-validation.ts:69` の `>`
- **推奨テスト名**: `it("reports ok when a bytes value exactly fills its length", ...)`
- **優先度**: 低

##### [観点 H] 型ガード・バリデーション関数の境界

- **対象**: `fieldValueKind`
- **不足内容**: 入力をハードコードで並べており、uint16・uint32・int8・int16 が既定の `"integer"` になることは確認していない。`FIELD_TYPES` に型が増えても追従しない
- **根拠コード**: `field-validation.ts:103` の既定 return と `domain/udp/types.ts:25` の `FIELD_TYPES`
- **推奨テスト名**: `it.each(FIELD_TYPES)("maps %s to a value kind", ...)` で期待値表を持つ
- **優先度**: 低

- 観点 A・B・I: 不足なし（空文字・空白・奇数長・サロゲートペア・`\u007f` 境界まで確認済み）
- 観点 C・E・F・G・J: 該当なし（純粋関数）

---

### `frontend/src/application/udp/receive.ts`

#### 対象関数・定数

| 名前 | ファイル:行 | 分岐/バリエーション数 | テスト済み? |
|---|---|---|---|
| createUdpReceiveState | receive.ts:20 | - | 部分的 |
| └ onMessage 購読 | receive.ts:29 | 2（先頭追加 / 上限） | 未テスト |
| └ refreshListeners | receive.ts:36 | 2（成功 / 失敗） | 部分的（成功のみ） |
| └ startListen | receive.ts:43 | 2（成功 / 失敗） | 未テスト |
| └ stopListen | receive.ts:58 | 2（成功 / 失敗） | 未テスト |
| └ clearMessages | receive.ts:65 | 1 | 未テスト |

#### 不足テストケース

##### [観点 A] 正常系の網羅

- **対象**: `startListen`
- **不足内容**: テストが無い。`listenPort()` と `listenEncoding()` を API に渡す、成功でセッションを追加する、失敗で `error()` にメッセージを入れて `"Failed to start listening"` を通知する、どちらでも最後に `loading()` を false に戻す、が未検証
- **根拠コード**: `receive.ts:43-56`
- **推奨テスト名**: `it("adds the new session and clears loading", ...)` / `it("records the error and notifies when listening fails", ...)`
- **優先度**: 高

- **対象**: `stopListen`
- **不足内容**: 成功で該当セッションだけ消す、失敗で通知してセッションを残す、が未検証
- **根拠コード**: `receive.ts:58-63`
- **推奨テスト名**: `it("removes only the stopped session", ...)`
- **優先度**: 中

- **対象**: `clearMessages` / 購読解除
- **不足内容**: `clearMessages` と、dispose 時に `onMessage` の解除関数が呼ばれることが未検証
- **根拠コード**: `receive.ts:32, 65-67`
- **推奨テスト名**: `it("unsubscribes from messages on dispose", ...)`
- **優先度**: 低

##### [観点 B] 入力値の境界

- **対象**: `onMessage` の受信処理
- **不足内容**: 新しいメッセージを先頭に追加すること、`UDP_MAX_MESSAGES`（500）を超えたら古いものから捨てることが未テスト。テストの `onMessage`（`receive.test.ts:28`）はコールバックを捨てるので、この経路に到達できない
- **根拠コード**: `receive.ts:29-31`、`config/limits.ts` の `UDP_MAX_MESSAGES = 500`
- **推奨テスト名**: `it("keeps the newest UDP_MAX_MESSAGES messages, newest first", ...)`
- **優先度**: 高

##### [観点 E] 異常系の境界

- **対象**: `refreshListeners`
- **不足内容**: `getListeners` が reject したときに `"Failed to restore listeners"` を通知し、sessions を空のままにすることが未テスト
- **根拠コード**: `receive.ts:37-40`
- **推奨テスト名**: `it("notifies when restoring listeners fails", ...)`
- **優先度**: 中

- 観点 C: 初期状態（空）は確認済み。削除後の参照は観点 A の `stopListen` に含めた
- 観点 D・G・H: 該当なし
- 観点 F: 不足なし（初期化時の fire-and-forget を `setTimeout(0)` で待ってから検証している）
- 観点 I・J: 不足なし

---

### `frontend/src/application/udp/targets.ts`

#### 対象関数・定数

| 名前 | ファイル:行 | 分岐/バリエーション数 | テスト済み? |
|---|---|---|---|
| createTargetsState | targets.ts:15 | - | 部分的 |
| └ refreshTargets | targets.ts:22 | 1 | 済 |
| └ reorderTargets | targets.ts:28 | 2 | 済 |
| └ saveTarget | targets.ts:40 | 2（成功 / 失敗） | 部分的（失敗のみ） |
| └ deleteTarget | targets.ts:46 | 2（成功 / 失敗） | 部分的（失敗のみ） |

#### 不足テストケース

##### [観点 A] 正常系の網羅

- **対象**: `saveTarget` / `deleteTarget`
- **不足内容**: 成功時に保存結果を返すこと、成功後に一覧を読み直すことが未検証
- **根拠コード**: `targets.ts:41-45`、`47-50`
- **推奨テスト名**: `it("returns the saved target and refreshes the list", ...)`
- **優先度**: 中

##### [観点 E] 異常系の境界

- **対象**: `refreshTargets`
- **不足内容**: ガードされておらず `getTargets` の reject がそのまま伝わること、`saveTarget` で API 成功後の再読み込みが失敗しても通知・reject になることが未テスト
- **根拠コード**: `targets.ts:23`、`targets.ts:43`
- **推奨テスト名**: `it("propagates getTargets failures", ...)`
- **優先度**: 低

- 観点 B・C・I: 不足なし（範囲外インデックス・並び順の保存・通知文言を確認済み）
- 観点 D・F・G・H・J: 該当なし

---

### `frontend/src/application/ui/guard.ts`

#### 対象関数・定数

| 名前 | ファイル:行 | 分岐/バリエーション数 | テスト済み? |
|---|---|---|---|
| runGuarded | guard.ts:13 | 3（成功 / Error / 非 Error） | 済 |
| notifyOnError | guard.ts:31 | 3（成功 / Error / 非 Error） | 部分的 |

#### 不足テストケース

##### [観点 E] 異常系の境界

- **対象**: `notifyOnError`
- **不足内容**: Error 以外を throw したときに `String(err)` で通知し、元の値をそのまま再送出する（包み直さない）ことが未テスト
- **根拠コード**: `guard.ts:39-40`
- **推奨テスト名**: `it("rethrows the original non-Error value", ...)`（`rejects.toBe(value)`）
- **優先度**: 低

- 観点 A・I: 不足なし
- 観点 B・C・D・F・G・H・J: 該当なし

---

### `frontend/src/application/ui/notifications.ts`

#### 対象関数・定数

| 名前 | ファイル:行 | 分岐/バリエーション数 | テスト済み? |
|---|---|---|---|
| createNotificationStore | notifications.ts:40 | - | 部分的 |
| └ push（notify.*） | notifications.ts:71 | 5（key 重複 / 上限 / error 時間 / 非 error 時間 / 時間 0） | 部分的 |
| └ dismiss | notifications.ts:52 | 2 | 済 |
| └ clear | notifications.ts:64 | 1 | 部分的 |
| notificationStore / notify | notifications.ts:135-136 | - | 対象外（シングルトン） |

#### 不足テストケース

##### [観点 B] 入力値の境界

- **対象**: `createNotificationStore` の既定値
- **不足内容**: オプション省略時の `autoDismissMs = 4000`、`errorDismissMs = 8000`、`max = 5` が未テスト
- **根拠コード**: `notifications.ts:43-45`
- **推奨テスト名**: `it("uses 4s / 8s / 5 as defaults", ...)`
- **優先度**: 低

##### [観点 C] 状態の境界

- **対象**: key の解放
- **不足内容**: key 付き通知が (1) 上限超過で押し出されたとき、(2) 自動消滅したとき、に key が解放され同じ key で再通知できることが未テスト。dismiss による解放だけが確認されている
- **根拠コード**: `notifications.ts:100-102`（押し出し時）、`notifications.ts:112`（タイマー → `dismiss`）。解放漏れがあると、その key の通知（MQTT の connection-lost など）が以後出なくなる
- **推奨テスト名**: `it("releases the key of a notification dropped by max", ...)` / `it("releases the key after auto-dismiss", ...)`
- **優先度**: 中

- **対象**: `clear`
- **不足内容**: clear 後に同じ key で通知できること、保留中のタイマーが残らないこと（`vi.getTimerCount()`）が未テスト
- **根拠コード**: `notifications.ts:65-67`
- **推奨テスト名**: `it("releases keys and timers on clear", ...)`
- **優先度**: 低

##### [観点 I] 戻り値・副作用の検証

- **対象**: `notify.*`
- **不足内容**: `toMatchObject` で一部フィールドのみ検証しており、`createdAt` が未検証
- **根拠コード**: `notifications.ts:84`
- **推奨テスト名**: `vi.setSystemTime` を使い `toEqual` で全フィールドを確認
- **優先度**: 低

- 観点 F: 不足なし（fake timers で非 error・error・0 の各時間を確認済み）
- 観点 D・E・G・H・J: 該当なし

---

### `frontend/src/components/ui/copy-button.ts`

#### 対象関数・定数

| 名前 | ファイル:行 | 分岐/バリエーション数 | テスト済み? |
|---|---|---|---|
| createCopyButton | copy-button.ts:20 | 3（単一 / キー付き / 再コピー） | 部分的 |

#### 不足テストケース

##### [観点 E] 異常系の境界

- **対象**: `copy`
- **不足内容**: `navigator.clipboard.writeText` が reject しても「コピー済み」表示になり、reject が処理されないことが未テスト
- **根拠コード**: `copy-button.ts:26`（戻り値の Promise を扱っていない）
- **推奨テスト名**: `it("does not mark copied when the clipboard write fails", ...)`（現状の挙動を仕様とするかの判断を含む）
- **優先度**: 低

##### [観点 F] 非同期・時間の境界

- **対象**: `copy`
- **不足内容**: 同じキーをリセット前に再コピーするとタイマーが延長されることが未テスト
- **根拠コード**: `copy-button.ts:28-29`
- **推奨テスト名**: `it("restarts the reset timer on a repeated copy", ...)`
- **優先度**: 低

- 観点 A・C・I・J: 不足なし（キーごとの状態、onCleanup でのタイマー破棄、`vi.unstubAllGlobals` まで確認済み）
- 観点 B・D・G・H: 該当なし

---

### `frontend/src/presentation/utils/format.ts`

#### 対象関数・定数

| 名前 | ファイル:行 | 分岐/バリエーション数 | テスト済み? |
|---|---|---|---|
| formatJson | format.ts:6 | 2 | 済 |
| formatTime | format.ts:19 | 2（number / Date） | 済 |

#### 不足テストケース

##### [観点 B] 入力値の境界

- **対象**: `formatJson` / `formatTime`
- **不足内容**: 空白のみ `"  "`（null）、`"null"`（`"null"`）、不正な Date（`"NaN:NaN:NaN.NaN"` になる）が未テスト
- **根拠コード**: `format.ts:8`、`format.ts:20-25`（Invalid Date の判定が無い）
- **推奨テスト名**: `it("returns null for whitespace-only input", ...)` / `it("formats an invalid date as ...", ...)`
- **優先度**: 低

- 観点 A・E・I: 不足なし（パース失敗・0 埋めまで確認済み）
- 観点 C・D・F・G・H・J: 該当なし

---

### `frontend/src/presentation/components/udp/field-display.ts`

#### 対象関数・定数

| 名前 | ファイル:行 | 分岐/バリエーション数 | テスト済み? |
|---|---|---|---|
| FIELD_VALUE_LABELS | field-display.ts:10 | 5 | 部分的（3/5） |
| FIELD_VALUE_PLACEHOLDERS | field-display.ts:18 | 5 | 部分的（4/5） |
| FIELD_VALUE_INPUT_TYPES | field-display.ts:27 | 5 | 済 |
| byteCountLabel | field-display.ts:38 | 5 | 済 |

#### 不足テストケース

##### [観点 I] 戻り値・副作用の検証

- **対象**: `FIELD_VALUE_LABELS` / `FIELD_VALUE_PLACEHOLDERS`
- **不足内容**: `"wide-integer"`・`"float"` のラベル、`"wide-integer"` の placeholder が未検証
- **根拠コード**: `field-display.ts:14-15, 22`
- **推奨テスト名**: `expect(FIELD_VALUE_LABELS).toEqual({...})` で全キーを確認
- **優先度**: 低

- 観点 B・C・D・E・F・G・H・J: 該当なし（定数と網羅的な switch のみ）

---

### `frontend/src/presentation/components/sidebar/use-long-press-drag.ts`

#### 対象関数・定数

| 名前 | ファイル:行 | 分岐/バリエーション数 | テスト済み? |
|---|---|---|---|
| makeLongPressDragHandlers | use-long-press-drag.ts:15 | 4（非左ボタン / 長押し / 移動 / 早期 mouseup） | 済 |

#### 不足テストケース

##### [観点 B] 入力値の境界

- **対象**: `handleMove`
- **不足内容**: 移動距離がちょうど 5px（例: (3, 4)）では開始しないことが未テスト。既存は約 4.24px と 6px のみ
- **根拠コード**: `use-long-press-drag.ts:34` の `> DRAG_THRESHOLD_PX`
- **推奨テスト名**: `it("does not activate at exactly the threshold distance", ...)`
- **優先度**: 低

##### [観点 C] 状態の境界

- **対象**: 移動で開始した後のリスナー
- **不足内容**: 移動で開始した後に document のリスナーが外れていることが未テスト（長押しで開始した場合は確認済み）
- **根拠コード**: `use-long-press-drag.ts:27-28`
- **推奨テスト名**: `it("leaves no document listeners behind after a move activation", ...)`
- **優先度**: 低

##### [観点 J] テスト構造の整合性

- **対象**: `use-long-press-drag.test.ts:66-76`（"does not activate while the move stays within the threshold"）
- **不足内容**: このテストは mouseup を送らずに終わるので、document に mousemove / mouseup のリスナーが残る。現在の宣言順では、直後のテスト（`:78`）の `dispatchUp()` がこのリスナーも外すため、後続テストに影響は出ていない。ただし後始末を次のテストの操作に頼っており、テストの並べ替え・`it.only`・シャッフル実行では、残ったリスナーが後続の `dispatchMove(500, 500)`（`:122, :137`）で前のテストの `onActivate` を呼びうる
- **根拠コード**: `use-long-press-drag.ts:52-53` の `addEventListener`（外すのは `handleUp` か activate だけ）
- **推奨テスト名**: 該当テストの最後で `dispatchUp()` する、または `afterEach` で mouseup を送る
- **優先度**: 低

- 観点 A・F: 不足なし（fake timers で 249ms / 250ms の境界を確認済み）
- 観点 D・E・G・H・I: 該当なし

---

### `frontend/src/shared/array.ts`

#### 対象関数・定数

| 名前 | ファイル:行 | 分岐/バリエーション数 | テスト済み? |
|---|---|---|---|
| moveItem | array.ts:6 | 2（範囲外 / 移動） | 済 |

#### 不足テストケース

##### [観点 B] 入力値の境界

- **対象**: `moveItem`
- **不足内容**: 末尾インデックス（`to = length - 1`）への移動が有効であることが未テスト。範囲外側（`to = length`）は確認済み
- **根拠コード**: `array.ts:11` の `to >= arr.length`
- **推奨テスト名**: `it("moves an item to the last index", ...)`
- **優先度**: 低

- 観点 A・I: 不足なし（非破壊性まで確認済み）
- 観点 C・D・E・F・G・H・J: 該当なし

---

### `frontend/src/infrastructure/http/client.ts`

#### 対象関数・定数

| 名前 | ファイル:行 | 分岐/バリエーション数 | テスト済み? |
|---|---|---|---|
| sendRequest | client.ts:171 | 1 | 済 |
| openFilePicker | client.ts:187 | 2（token 有 / 無） | 部分的 |
| getCollections / getRootItems / createCollection / addFolder / addRequest | client.ts:199-241 | マッピング経由 | 済 |
| updateRequest | client.ts:243 | 1 | 部分的（引数は toMatchObject） |
| cancelRequest / deleteCollection / renameCollection / renameItem / deleteItem / moveItem / moveSidebarEntry / moveItemToSidebar / saveResponseBody / saveResponseBinary | client.ts:179-323 | 1 | 済 |
| discardResponseBody | client.ts:313 | 1 | 部分的（reject 未テスト） |
| getSidebarLayout | client.ts:281 | 1（as キャスト） | 済 |
| （内部）toWailsRequest | client.ts:44 | 1 | 部分的 |
| （内部）fromWailsRequestSettings | client.ts:53 | 3（null / proxyMode 3 通り / 各 `??`） | 部分的 |
| （内部）fromWailsRequestAuth | client.ts:73 | 3 | 部分的 |
| （内部）fromWailsFileReference | client.ts:83 | 3 | 部分的 |
| （内部）fromWailsFormRow / fromWailsTreeItem | client.ts:97, 149 | 各 2-3 | 済 |
| （内部）fromWailsRequestBody | client.ts:106 | 2（throw / 変換） | 部分的 |
| （内部）fromWailsHttpRequest | client.ts:121 | 2（throw / 変換） | 部分的 |
| （内部）fromWailsHttpResponse | client.ts:139 | 4 つの `??` | 部分的 |

#### 不足テストケース

##### [観点 B] 入力値の境界

- **対象**: `fromWailsRequestSettings`
- **不足内容**: settings はあるが一部フィールドが欠けている場合の既定値（`timeoutSec: 0`、`proxyMode: "system"`、`insecureSkipVerify: false`、`maxResponseBodyMB: 0`）が未テスト。これは settings が null のときの `DEFAULT_SETTINGS`（`timeoutSec: 120`、`proxyMode: "none"`、`insecureSkipVerify: true`）と値が違う
- **根拠コード**: `client.ts:56`（null → DEFAULT_SETTINGS）と `client.ts:61-69`（フィールド単位の `??`）、`domain/http/types.ts:134-141`
- **推奨テスト名**: `it("fills missing setting fields individually", ...)` で現状の既定値を固定する（DEFAULT_SETTINGS に合わせるべきかの判断を含む）
- **優先度**: 中

- **対象**: `fromWailsRequestAuth` / `fromWailsRequestBody` / `fromWailsFileReference` / `fromWailsHttpResponse`
- **不足内容**: auth が null（`type: "none"` と空文字）、`body.contents` が null（`{}`）、token だけで name の無いファイル参照、レスポンスの `headers` が null（`{}`）が未テスト
- **根拠コード**: `client.ts:75-78`、`114`、`86`、`142`
- **推奨テスト名**: `it("normalizes null auth / contents / headers", ...)`
- **優先度**: 低

##### [観点 E] 異常系の境界

- **対象**: `openFilePicker` / `discardResponseBody`
- **不足内容**: `OpenFilePicker` が null を返したとき（`selected?.token`）に undefined を返すこと、`OpenFilePicker` と `DiscardResponseBody` の reject が伝わることが未テスト
- **根拠コード**: `client.ts:191`、`client.ts:190, 314`
- **推奨テスト名**: `it("returns undefined when the backend returns null", ...)` / `it("propagates rejection from the backend", ...)`
- **優先度**: 低

##### [観点 G] Wails バインディング層の検証

- **対象**: `fromWailsHttpRequest`（getCollections / getRootItems 経由）
- **不足内容**: 変換後のリクエストを `toEqual` で全体比較するテストが無い。`doc ?? ""` と、settings null 時の `timeoutSec` / `insecureSkipVerify` などが未検証（proxyMode だけ確認している）
- **根拠コード**: `client.ts:127-136`（`135` の doc）。テスト側は `client.test.ts:498-511` が proxyMode のみ、`client.test.ts:561-579` が method と url のみ
- **推奨テスト名**: `it("maps a stored request field by field", ...)` で `toEqual`
- **優先度**: 中

- **対象**: `toWailsRequest`（sendRequest / addRequest / updateRequest）
- **不足内容**: `httpdomain.HTTPRequest.createFrom` は本物が動く（`wailsjs/go/models` はモックしていない）のに、送信値を全体で比較していない。特に、frontend 専用の `FileReference.hint` が RPC に載らないこと（`domain/http/types.ts:23-24` の前提）が未検証。`addRequest` は `objectContaining({ method })`、`updateRequest` は `toMatchObject` のみ
- **根拠コード**: `client.ts:44-46`、`wailsjs/go/models.ts:132-138`（FileReference の constructor が既知フィールドだけ写す）
- **推奨テスト名**: `it("does not send the file hint to the backend", ...)` / `it("sends the request as created by createFrom", ...)`
- **優先度**: 中

- 観点 A: 不足なし（全 export に正常系がある）
- 観点 H: 該当なし（型ガードは domain 側で観点 H として扱った）
- 観点 I: 観点 G にまとめた
- 観点 J: 不足なし（`beforeEach(vi.clearAllMocks)`、各テストでモック値を設定）
- モックで未検証: `getSidebarLayout` は `as` キャストで未知の kind を通す（`client.ts:284`）。テスト（`client.test.ts:970-976`）はこの挙動を記録しているが、本体に検証が無いので、presentation 側で未知 kind がどう扱われるかは単体テストの範囲外

---

### `frontend/src/infrastructure/mqtt/client.ts`

#### 対象関数・定数

| 名前 | ファイル:行 | 分岐/バリエーション数 | テスト済み? |
|---|---|---|---|
| connect | client.ts:16 | 1 | 済 |
| disconnect / subscribe / unsubscribe / publish / deleteProfile | client.ts:28-79 | 1 | 済 |
| getConnections / getProfiles | client.ts:57, 61 | 1（as キャスト） | 済 |
| saveProfile | client.ts:65 | 1 | 済 |

#### 不足テストケース

##### [観点 G] Wails バインディング層の検証

- **対象**: `getConnections` / `getProfiles`
- **不足内容**: 変換せず `as` キャストで返すため、Wails 生成型とドメイン型の差（例: `subscriptions` が null）はテストで検出できない。`subscriptions: null` の要素が来ると `connections.ts:334` の `.map` で落ちる
- **根拠コード**: `client.ts:58, 62`。Go 側は `internal/application/mqtt/service.go:408` で非 nil のスライスを作るので、現状の優先度は低い
- **推奨テスト名**: 変換関数を置くなら `it("normalizes null subscriptions", ...)`。置かないなら「モックで未検証」として扱う
- **優先度**: 低

- 観点 A・E・I・J: 不足なし（全関数で引数の完全一致・reject・戻り値を確認済み）
- 観点 B・C・D・F・H: 該当なし

---

### `frontend/src/infrastructure/mqtt/events.ts`

#### 対象関数・定数

| 名前 | ファイル:行 | 分岐/バリエーション数 | テスト済み? |
|---|---|---|---|
| onMqttEvent | events.ts:7 | 1 | 済 |

#### 不足テストケース

##### [観点 H] 型ガード・バリデーション関数の境界

- **対象**: `events.test.ts:11-17` の `MQTT_EVENTS`
- **不足内容**: イベント名をハードコードしており、生成された `WailsEvents`（`shared/wails-events.ts`）から作っていない。Go 側でイベントを足しても `it.each` が追従しない
- **根拠コード**: `shared/wails-events.ts:3-18`
- **推奨テスト名**: `Object.values(WailsEvents).filter((e) => e.startsWith("mqtt:"))` から `it.each` を作る
- **優先度**: 低

- 観点 A・G・I・J: 不足なし
- 観点 B・C・D・E・F: 該当なし

---

### `frontend/src/infrastructure/openapi/file-io.ts`

#### 対象関数・定数

| 名前 | ファイル:行 | 分岐/バリエーション数 | テスト済み? |
|---|---|---|---|
| openFilePicker | file-io.ts:22 | 1 | 未テスト |
| readFile | file-io.ts:26 | 1 | 未テスト |
| writeFile | file-io.ts:30 | 1 | 未テスト |
| saveFileAs | file-io.ts:38 | 1 | 未テスト |
| getRecents | file-io.ts:45 | 1 | 済 |
| removeRecent | file-io.ts:50 | 1 | 部分的（reject 未テスト） |
| moveRecent | file-io.ts:54 | 1 | 部分的（reject 未テスト） |

#### 不足テストケース

##### [観点 A] 正常系の網羅

- **対象**: `openFilePicker` / `readFile` / `writeFile` / `saveFileAs`
- **不足内容**: 7 関数中 4 つにテストが無い。引数の受け渡し（`writeFile(path, content)`、`saveFileAs(defaultName, content)`）と戻り値（キャンセル時の空文字）が未検証。テストファイルはモックを定義しているが import していない（`file-io.test.ts:14`）
- **根拠コード**: `file-io.ts:22-43`
- **推奨テスト名**: `it("passes the path and content to WriteFile", ...)` / `it("returns '' when the save dialog is canceled", ...)`
- **優先度**: 中

##### [観点 E] 異常系の境界

- **対象**: `removeRecent` / `moveRecent`
- **不足内容**: backend の reject が伝わることが未テスト（`application/openapi/files.ts` の `moveFile` は reject を前提に通知している）
- **根拠コード**: `file-io.ts:51, 55`
- **推奨テスト名**: `it("propagates rejection from the backend", ...)`
- **優先度**: 低

- 観点 G: 観点 A にまとめた（テストのある 3 関数のうち、`getRecents` は戻り値を `toEqual` で、`removeRecent` / `moveRecent` はバインディングへの引数を `toHaveBeenCalledWith` で確認済み。後の 2 つは戻り値を持たない）
- 観点 B・C・D・F・H・I・J: 該当なし

---

### `frontend/src/infrastructure/storage/local-storage.ts`

#### 対象関数・定数

| 名前 | ファイル:行 | 分岐/バリエーション数 | テスト済み? |
|---|---|---|---|
| createLastProfileStorage | local-storage.ts:19 | 3 | 済 |
| createPresetsStorage | local-storage.ts:31 | 2（retain 補完） | 済 |
| createThemeStorage / createActiveRequestStorage / createExpandedFoldersStorage / createProfileOrderStorage / createTargetOrderStorage | local-storage.ts:42-84 | 2-3 | 済 |
| loadFromStorage | local-storage.ts:86 | 3（null / 正常 / 壊れた JSON） | 部分的 |
| saveToStorage | local-storage.ts:100 | 2 | 済 |
| removeFromStorage | local-storage.ts:110 | 2 | 済 |

#### 不足テストケース

##### [観点 B] 入力値の境界

- **対象**: 各 `createXxxStorage().load`
- **不足内容**: JSON としては正しいが形が違う値が未テスト。`"mqtt:presets"` が `"null"` や `"{}"` だと `.map` で TypeError になり、プリセット state の生成（`application/mqtt/presets.ts:22`）で落ちる。並び順が配列でない場合は `applyOrder` の `order.map`（`application/shared/order.ts:5`）で落ちる。テーマは `"blue"` などの不正値もそのまま返る
- **根拠コード**: `local-storage.ts:34-37`（形の検証無しで `.map`）、`local-storage.ts:44, 74, 81`、`local-storage.ts:90`（`as T` キャストのみ）
- **推奨テスト名**: `it("falls back to [] when presets are not an array", ...)`（現状は失敗するテスト。修正方針とあわせて追加する）
- **優先度**: 中

##### [観点 E] 異常系の境界

- **対象**: `loadFromStorage`
- **不足内容**: `localStorage.getItem` 自体が throw する場合（ストレージへのアクセスが拒否された環境の SecurityError）が未テスト。`getItem` は try の外にあるので例外がそのまま伝わり、try で包んでいる save / remove と扱いが違う
- **根拠コード**: `local-storage.ts:87`（try の外）と `local-storage.ts:101-107, 111-115`
- **推奨テスト名**: `it("returns the fallback when getItem throws", ...)`
- **優先度**: 中

- 観点 C: 不足なし（空・上書き・削除後・`setItem` の QuotaExceededError を確認済み）
- 観点 I・J: 不足なし（実際の `localStorage.getItem` で保存内容を確認し、`afterEach` で `clear()` と `restoreAllMocks()`）
- 観点 D・F・G・H: 該当なし

---

### `frontend/src/infrastructure/udp/client.ts`

#### 対象関数・定数

| 名前 | ファイル:行 | 分岐/バリエーション数 | テスト済み? |
|---|---|---|---|
| send | client.ts:38 | 1（createFrom 変換） | 部分的 |
| getTargets / saveTarget / deleteTarget | client.ts:43-55 | 1 | 済 |
| startListen / stopListen / getListeners | client.ts:66-81 | 1（encoding は as キャスト） | 済 |
| onMessage | client.ts:83 | 1 | 済 |

#### 不足テストケース

##### [観点 G] Wails バインディング層の検証

- **対象**: `send`（`toWailsRequest`）
- **不足内容**: `udpdomain.UDPSendRequest.createFrom` は本物が動くのに、`fixedLengthPayload.fields` と `messageLength` の受け渡しが未検証。既存テストは `toMatchObject` で host・port・encoding・payload・endianness だけを見ており、`encoding: "fixed"` のフィールド配列（fieldType・length・value）を渡すケースが無い
- **根拠コード**: `client.ts:21-23`。入れ子クラスの配列変換（`convertValues`）は生成コード側の処理で、http 側のテストのコメント（`http/client.test.ts:271-273`）にあるように過去に配列の形が壊れた経路
- **推奨テスト名**: `it("passes fixed-length fields intact", ...)` で送信値を `toEqual`
- **優先度**: 中

##### [観点 E] 異常系の境界

- **対象**: `onMessage`
- **不足内容**: イベントの中身を検証せずにコールバックへ渡すため、未知の encoding などがそのまま application 層へ届く。テストは中身の素通しだけを確認している
- **根拠コード**: `client.ts:84`（`EventsOn(WailsEvents.udpMessage, cb)` をそのまま返す）
- **推奨テスト名**: 検証を入れる場合に `it("drops messages with an unknown encoding", ...)`。入れないなら「モックで未検証」として扱う
- **優先度**: 低

- 観点 A・I・J: 不足なし（全 export に正常系・reject・`toEqual` による戻り値確認がある）
- 観点 B・C・D・F・H: 該当なし
- モックで未検証: `startListen` / `getListeners` の encoding は `as` キャスト（`client.ts:62`）で、テスト（`client.test.ts:253-259, 307-313`）は未知の値が素通りすることを記録している

---

## 全体評価

### 観点別カバレッジ

> **対象範囲**: 集計はテストファイルがある 29 モジュールだけを数えた。末尾の「参考」に挙げたテストファイルが無い本体コードは含まないので、フロントエンド全体のカバー率として読まない。
> **「不足 N 件」の単位**: 本文の「対象」の件数（サマリー表の不足ケース数の合計 94 件を観点別に分けたもの）。1 件に複数の関数・分岐をまとめたものがあるので、未テストの関数の数とは一致しない。
> **カバー率の単位**: [A] だけは数えた値で、分母・分子は export 関数・状態メソッドの数（174 件のうち正常系のテストがあるもの 132 件、未テスト 42 件）。本文では、この 42 件を 22 件の指摘にまとめた（例: file-io の未テスト 4 関数は 1 件）。
> [B]〜[J] の割合は、チェック項目を数え上げずに付けた目安で、分母・分子を示せないため再計算できない。観点どうしの相対的な手薄さを見るためだけに使い、数値として引用しない。

| 観点 | カバー率 | 不足（指摘件数） |
|---|---|---|
| [A] 正常系の網羅 | 76%（132 / 174 関数・状態メソッド） | 22 件 |
| [B] 入力値の境界 | 目安 65% | 21 件 |
| [C] 状態の境界 | 目安 70% | 7 件 |
| [D] 組み合わせの境界 | 目安 65% | 9 件 |
| [E] 異常系の境界 | 目安 70% | 15 件 |
| [F] 非同期・時間の境界 | 目安 45% | 4 件 |
| [G] Wails バインディング層の検証 | 目安 85% | 4 件 |
| [H] 型ガード・バリデーション関数の境界 | 目安 70% | 4 件 |
| [I] 戻り値・副作用の検証 | 目安 75% | 6 件 |
| [J] テスト構造の整合性 | 目安 90% | 2 件 |

層ごとの傾向:

- **domain・infrastructure・純粋関数**（topic、field-validation、local-storage、http/mqtt/udp の client など）は、境界値・reject・マッピングまで厚くテストされている。
- **application 層の状態管理**（`createConnectionsState`、`createUdpReceiveState`、`createRequestState` の保存系、`createAutoSaveEffect`）は、復元や一部の失敗通知しかテストされていない。テストダブルがイベントのハンドラを捨てる作りのため、受信経路には一度も到達していない。

### 最優先で追加すべきテスト TOP5

1. **`createAutoSaveEffect` と `saveCurrentRequest`（application/http/request.ts）** — 根拠: HTTP の編集内容を保存する経路（`request.ts:319-344, 387-417`）にテストが無い。デバウンス・未保存タブのスキップ・失敗時の `saveError` と通知が壊れても検出できない。`afterSave` → `patchRequest`（`collections.ts:258`、こちらも未テスト）でサイドバーとの同期もこの経路に乗っている。
2. **MQTT の受信バッチと上限（application/mqtt/connections.ts `flushMessages`）** — 根拠: 受信メッセージを購読で振り分け、muted を除き、`maxMessages` / `maxTopics` / バッファ 5000 件で抑える処理（`connections.ts:160-248`）が未テスト。複数の購読が一致したときに最初の購読の `muted` だけで判定する点（`connections.ts:191-196`）も、仕様を決めてあわせて固定する。テストの `noopEvent` がハンドラを捨てるので到達できない。表示漏れやメモリの増え続けに直結する。
3. **ブローカーポートの検証と URL の往復（application/mqtt/profile-validation.ts）** — 根拠: `type="number"` のポート欄からも入力できる `"1e3"`・`"1883.0"` が `isValidBrokerPort`（`profile-validation.ts:12-13`）を通り、保存した broker URL を `parseBrokerUrl` が読めずに `mqtt://localhost:1883` へ戻る（検証関数の挙動は Node で確認済み、画面からの入力可否は HTML 仕様による推定）。既存データを黙って書き換える経路なので、テストと修正を同時に入れる。
4. **`sendRequest` の失敗経路（application/http/request.ts）** — 根拠: `api.sendRequest` の reject 時にエラーレスポンスを表示し `loading` を解除する catch / finally（`request.ts:227-240`）が未テスト。テストの偽 API は resolve しかしない。あわせて、送信中に別リクエストへ切り替えた後の遅れた応答（`request.ts:220`）の扱いも固定する。
5. **UDP 受信の開始と受信上限（application/udp/receive.ts）** — 根拠: `startListen` の成功・失敗（loading・error・通知、`receive.ts:43-56`）と、受信メッセージを新しい順に `UDP_MAX_MESSAGES` 件で切る処理（`receive.ts:29-31`）が未テスト。テストは 2 件で、どちらも起動時の復元だけを見ている。

### 総合評価

**信頼度**: 中

**主なリスク**:

- MQTT の接続タブの状態遷移（再接続での id の引き継ぎ・再購読、切断・クローズ、ライフサイクルイベント）と受信メッセージの振り分け・上限処理が壊れても、単体テストでは検出できない。
- HTTP の自動保存・切り替え時の保存・送信失敗時の表示が未テストのため、編集内容の消失やローディング表示の固着といった退行を見逃しやすい。
- 入力検証の抜け（ポート文字列の `Number()` 変換、float の空白、正しい JSON だが形の違う localStorage の値）が、保存データの書き換えや起動時の TypeError につながる経路として残っている。

**推奨アクション**:

1. application 層のテストダブルを、イベントハンドラと永続化の呼び出しを記録できる形（`vi.fn` と、ハンドラを保持して発火できる `onEvent` / `onMessage`）にそろえる。node 環境では `requestAnimationFrame` を `vi.stubGlobal` で同期実行に差し替える。これで TOP5 の 2・5 と connections の大半に手が届く。
2. TOP5 の 1・4 は、既存の `request.test.ts` の `makeApi` に reject を返せる口を足し、fake timers で `createAutoSaveEffect` を回せば追加できる。
3. TOP5 の 3 と local-storage の形の検証は、現状の挙動が不具合寄りなので、テストを先に書いて失敗させてから本体を直す。
4. 残りの低優先度の項目（境界値の追加、`it.each` を定数配列から作る、リスナーやタイマーの後始末）は、該当ファイルを次に触るときにまとめて足す。

---

## 参考: テストファイルが無い本体コード（今回の分析対象外）

スコープ（`*.test.ts` とその同名ファイル）の外だが、分岐を持つのにテストファイルが無いもの。ほかのテスト経由で一部が実行されているものは括弧内に記した。サマリー表と観点別カバレッジの集計には含めていない。

| ファイル | 主な export | 行数 |
|---|---|---|
| application/udp/send.ts | createUdpSendState | 157 |
| application/mqtt/subscriptions.ts | createSubscriptionsState | 151 |
| infrastructure/openapi/parser.ts | parseSpec | 53 |
| components/ui/focus-trap.ts | createFocusTrap | 53 |
| presentation/utils/json-highlight.ts | highlightJson | 33 |
| application/ui/theme.ts | createThemeStore | 17 |
| application/mqtt/subscription.ts | makeSubscription（connections.test.ts の restore 経由で一部） | 15 |
| shared/async-op.ts | withLoading | 15 |
| application/shared/order.ts | applyOrder（profiles / targets のテスト経由） | 11 |
| domain/mqtt/types.ts | isConnected | 79 |
