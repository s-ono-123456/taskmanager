# セッションコンテキスト

> 「現在進行中の状態」のみを記録する。完了した作業の経緯・学んだことは docs/work-log.md へ、
> 次にやるべきタスクは docs/task-queue.md へ書く。
>
> **並列セッション対応のため、このファイルは全文上書きしない。** 自分が追加した行、
> または自分が担当しているセッション行のみを更新すること。他セッションの行は編集・削除しない。

## プロジェクト概要

`taskmanager`はタスク管理自動化構想（JIRA2プロジェクト＋個人タスクをMattermost/メール/Zoomから
自動収集し、JIRA自動起票・完了候補提示まで行う）のうち、「スキーマとダッシュボードUIの
パイロット実装」に相当するリポジトリ。2026-09-12に`/work`（メインリポジトリ）から独立した。
外部通信は一切行わない（詳細はCLAUDE.md・docs/design/design.md参照）。

## アクティブセッション

作業を開始したら、この表に自分の行を追加する。作業が完了/中断したら自分の行を削除する
（完了した作業の詳細は docs/work-log.md へ）。

| セッションID（ツール種別@着手時刻） | 作業内容（1行） | 対象ファイル/ディレクトリ | 更新時刻 |
|---|---|---|---|

## 関連ファイル

- `docs/design/design.md` — 全体方針（位置づけ・技術スタック・ルート一覧の索引等）
- `docs/design/data-model.md` — DB設計
- `docs/design/screen-board.md` / `docs/design/screen-close-requests.md` — 画面設計（画面ごと）
- `docs/adr/proposals/task-management-automation.md`（索引） — 全体構想のADR（論点ごとに分割）
- `docs/design/task-management-automation.md` — 全体構想のうち未実装部分の確定設計
- `CLAUDE.md` — 技術スタック・実行方法・誤解しやすい業務ルール・運用ルール
