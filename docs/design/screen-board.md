# 画面設計: カンバンボード画面

> 全体方針・技術スタックは`docs/design/design.md`、DBスキーマは`docs/design/data-model.md`を
> 参照。本書は`GET /`（`internal/web/templates/board.html.tmpl`）で表示するメイン画面
> （カンバンボード、タスク編集/新規作成モーダル）の仕様を扱う。

## レーン構成
- 4レーン固定: `未着手`(todo) / `進行中`(in_progress) / `確認中`(reviewing) / `完了`(done)。
  レーン追加は`internal/web/kanban.go`の`Statuses`/`StatusLabels`とテンプレート内の
  `accentBarClass`/`accentPillClass`に値を足すだけで済む。
- **完了レーンは直近7日以内に完了したタスクのみ表示**する
  （`DoneLaneWindowDays = 7`、`isRecentlyClosed()`で判定）。7日を超えたものは
  データとしては残るが、「非表示分も表示」をONにしても一切表示されない（無制限に膨らむのを防ぐ
  ための表示上のフィルタであり、削除ではない）。

## フィルタ（ツールバー）
- 対象(`target`)プルダウンと「非表示分も表示」(`show_untracked`)チェックボックスは、
  `hx-trigger="change"`で変更時に自動送信する（絞り込みボタンは無い）。htmxの
  `hx-get="/" hx-select="#board" hx-target="#board" hx-swap="outerHTML" hx-push-url="true"`で、
  フルページを取得しつつ`#board`部分だけを差し替え、ブラウザURLも更新する。
- `show_untracked`がOFFの場合、SQLクエリ側（`ListTasks`、`sqlc.narg`によるオプショナル
  WHERE）で`tracked = 1`のみに絞り込む。ONの場合は`tracked`に関わらず全件取得したうえで、
  完了レーンの7日フィルタだけは常に適用される。

## カード
- クリックすると編集モーダル（詳細表示＋編集を兼ねる）を開く。カード上のdata属性
  （`data-title`等）からモーダルへ値を流し込む方式で、サーバー往復なしに即座に開く
  （Flask版と同じ設計）。
- `draggable="true"`。ドラッグ&ドロップで別レーンへ移動できる（後述）。
- レイアウト・タグ表示・非表示バッジ・期限バッジの見た目はFlask版から変更していない。

## 編集・新規作成の送信（htmx化）
- 編集フォーム・新規作成フォーム・非表示切替ボタンは、いずれも`hx-post`で送信し、
  レスポンスとして「ボード全体（4レーン+カード）のHTMLフラグメント」を受け取り
  `hx-target="#board" hx-swap="outerHTML"`で差し替える。個別カード単位の差分更新は
  レーン移動・7日フィルタの再計算等が絡み複雑になるため採用していない。
- 現在のフィルタ状態（`target`/`show_untracked`）は、各操作のhx-valsで
  `filter_target`/`filter_show_untracked`という専用キー名として送る（JS関数
  `filterStateVals()`）。編集フォーム自身が"target"という別の意味のフィールド（タスクの
  対象）を持つため、フィルタと同じキー名で送ると値が衝突する。この回避のため専用キー名に
  している。
- サーバー側の各POSTハンドラは、バリデーション成功・失敗いずれもHTTP 200で
  「ボードフラグメント＋トースト」を返す（htmxは4xx/5xxを自動スワップしないため）。
  成功/失敗の種別はレスポンスヘッダー`X-Toast-Category`（`success`/`error`）で伝える。
- **編集・新規作成モーダルは、`X-Toast-Category`が`error`でない場合のみ閉じる**
  （`document.body`に張った`htmx:afterRequest`のグローバルリスナーで判定）。
  **これがFlask版からの唯一の意図的な挙動変更**: Flask版はredirectベースだったため
  バリデーションエラー時もモーダル相当の入力内容は失われていたが、Go+htmx版では
  エラー時に入力内容を保持したままモーダルを開いておける（ユーザー承認済みのUX改善）。

## ドラッグ&ドロップ
- 個々の列（`.column`）ではなく`document`全体に`dragover`/`drop`リスナーを張り、
  **ポインタのx座標がどの列の左右範囲(`getBoundingClientRect`)に入っているかだけ**で
  ドロップ先レーンを決定する（y座標・列の高さは一切考慮しない。Flask版と同じロジック）。
- ドロップ確定時は`htmx.ajax('POST', '/tasks/<id>/move', {target:'#board', swap:'outerHTML', values:{...}})`
  を呼ぶ（Flask版の`fetch`+`location.reload()`から置き換え。フルリロードなしで`#board`のみ
  更新される）。

## モーダル（2種、ともに標準`<dialog>`要素、外部ライブラリ不使用）
- 構成・項目はFlask版から変更していない（詳細表示＋編集のハイブリッドUI、新規タスクの
  簡易フォーム、背景クリックで閉じる等）。

## 自動リフレッシュ
- 45秒間隔で`htmx.ajax('GET', '/', {target:'#board', swap:'outerHTML', select:'#board', ...})`
  を呼び、フルリロードなしで`#board`のみ更新する（Flask版は`location.reload()`によるフル
  リロードだった）。編集モーダル・新規タスクモーダルのいずれかが開いている間はスキップする
  （Flask版と同じ）。

## 通知（トースト）
- Flask版のsessionベースのflashメッセージは廃止し、POSTレスポンスに
  `<div id="toast-container" hx-swap-oob="true">...</div>`を含める方式に統一した
  （htmxのout-of-band swap）。セッション機構（署名付きcookie等）が不要になり、実装の軽量化に
  直結している。表示位置・見た目（`</body>`直前、`position: fixed; bottom-0`）はFlask版と
  同じ。

## 関連ルート（`internal/web/handlers.go`）

| メソッド/パス | 概要 |
|---|---|
| `GET /` | ボード表示。`target`・`show_untracked`をクエリパラメータで受け取る |
| `POST /tasks/new` | 新規タスク作成（due_date任意）。target が jira_a/jira_b の場合はJIRA起票スタブのログのみ出力（実通信なし） |
| `POST /tasks/{id}/edit` | 編集モーダルからの保存。title/target/statusを検証し更新（due_dateは未入力ならNULLとして保存）。`status`が`done`へ/から変化する際は`closed_at`をその場で設定/クリアする |
| `POST /tasks/{id}/move` | ドラッグ&ドロップからの状態変更。JIRA連携タスクなら`stubJiraTransition()`を呼ぶ |
| `POST /tasks/{id}/track` | 「非表示」/「再表示」ボタン。後述の業務ルール参照 |

## 「非表示」の業務ルール（`handleToggleTrack`）

- **非表示にする（`tracked: 1→0`）**: 対象タスクが`status != 'done'`の場合、同時に
  `status = 'done'`・`closed_at = 現在時刻`を設定する（＝一旦完了扱いにする）。既に`done`だった
  場合は`tracked`のみ変更し、元の`closed_at`は上書きしない。
- **再表示する（`tracked: 0→1`）**: `jira_key`がある（JIRA連携）タスクの場合のみ
  `stubJiraTransition(jiraKey, "resync")`を呼び、`last_synced_at`を現在時刻に更新する
  （実際のJIRA API通信はしない）。個人タスクの場合は`tracked`を戻すのみ。
- 「削除」に相当する機能は存在しない（Flask版から変更なし）。

## 関連ドキュメント

- `docs/design/design.md` — 全体方針・技術スタック・ルート一覧の索引・既知の制限。
- `docs/design/data-model.md` — `tasks`テーブルのスキーマ詳細。
- `docs/design/screen-close-requests.md` — 別画面（クローズ要求一覧）の仕様。
