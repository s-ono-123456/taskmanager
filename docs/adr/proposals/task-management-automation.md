# 検討案: タスク管理自動化(JIRA/個人タスク)の設計（task-management-automation）

- 起票: 2026-09-12 / タスクID: `task-management-automation`
- 目的: JIRA2プロジェクト＋個人タスク（Mattermost/メールで飛んでくる、現状管理不在）を対象に、
  (1) Mattermost/メール/Zoomからのタスク自動収集、(2) 打ち合わせ内容からのJIRA自動起票、
  (3) 完了確認による完了候補提示、(4) 画面でのタスク状況確認・更新・非表示化・クローズ、を
  実現する仕組みを設計する（「削除」は[論点F](task-management-automation--f-delete-vs-hide.md)
  の採用により設けない方針）。
- 種別: 索引（複数論点方式。論点ごとの詳細は下記の個別ファイル参照）
- 状態: 論点A〜G全て決着（A2・B1・C3・D2・E2・F2・G1採用。論点Cは当初C2を採用したが
  2026-09-13にC3へ変更）。Zoom収集・JIRA連携（2プロジェクト＋個人タスク）まで含めた
  実装設計は完了しているが、
  「スキーマとダッシュボードUI」部分のみGo + sqlc + htmxでパイロット実装済み
  （`/work/public/taskmanager`リポジトリ）。`collector`/`extractor`/`syncer`/`registrar`/
  `digest`（Mattermost/JIRA/Zoom/Claude APIとの実連携）は未実装のまま。

## 背景・現状

- JIRAはプロジェクトごとに別管理（2プロジェクト）。
- 個人タスクはMattermost/メールで依頼されるが、専用の管理先がなく漏れやすい。
- ユーザー確認済みの前提:
  - 完了判定(クローズ)は**候補提示＋人間承認**とする（自動クローズはしない）。JIRAの誤クローズは
    気づかれにくく実害が大きいため。
  - 利用可能な基盤: Zoom文字起こし/要約API、Claude API等のLLM呼び出し、JIRA/Mattermostの
    bot・Webhook権限、Dockerコンテナ＋cronでの定期実行環境。

## 論点一覧

| 論点 | タイトル | 状態 | ファイル |
|---|---|---|---|
| A | 個人タスクの格納先 | 採用済み（A2） | [task-management-automation--a-personal-task-store.md](task-management-automation--a-personal-task-store.md) |
| B | 収集トリガー方式 | 採用済み（B1） | [task-management-automation--b-collection-trigger.md](task-management-automation--b-collection-trigger.md) |
| C | 完了候補の承認UI | 採用済み（C3、2026-09-13。C2から変更） | [task-management-automation--c-completion-approval-ui.md](task-management-automation--c-completion-approval-ui.md) |
| D | ダッシュボードの実装技術 | 採用済み（D2） | [task-management-automation--d-dashboard-tech.md](task-management-automation--d-dashboard-tech.md) |
| E | タスクの進捗管理粒度（`tasks.status`） | 採用済み（E2） | [task-management-automation--e-status-granularity.md](task-management-automation--e-status-granularity.md) |
| F | 「追跡除外」操作の範囲（削除 vs 非表示） | 採用済み（F2） | [task-management-automation--f-delete-vs-hide.md](task-management-automation--f-delete-vs-hide.md) |
| G | 会議・メール本文の外部LLM API送信可否 | 決定済み（G1） | [task-management-automation--g-data-handling-policy.md](task-management-automation--g-data-handling-policy.md) |

## 関連する設計ドキュメント

- `docs/design/task-management-automation.md` — 全体パイプライン図・データモデルER図・
  収集/抽出/登録/完了候補提示・クローズ/管理画面/digestの各仕様・リスク・技術スタックなど、
  上記論点の採用結果として確定した設計。
- `docs/design/design.md` — 「スキーマとダッシュボードUI」部分のパイロット実装
  （Go + sqlc + htmx）の詳細設計書。

## 想定される次の一手

1. 認証情報・権限の準備（ユーザー側）: Mattermost botトークン、JIRA APIトークン、Zoom
   Server-to-Server OAuthアプリ（会議情報・会議要約の読み取りスコープ）。
2. `project_routing`（監視対象チャンネル/メールフォルダ/Zoom会議シリーズと`project_hint`の対応）
   と`user_map`（主要メンバーの初期データ）を整備する。
3. collector（mattermost/email/zoom）・extractor・registrar（JIRA登録/個人タスク登録）・
   digest（新規登録・対象不明タスクの日次まとめ投稿）を実装する（管理画面（ダッシュボード）
   はスキーマ・クローズ要求一覧画面（論点C3）含めパイロット実装済み。`docs/design/design.md`
   参照）。
4. 運用開始後、precision/recall（登録・クローズ候補それぞれ）を継続的にモニタリングし、
   プロンプト・`project_routing`・confidence閾値を調整する。
