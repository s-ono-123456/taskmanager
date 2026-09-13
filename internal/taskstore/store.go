package taskstore

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaSQL string

// OpenDB はSQLiteファイルへの接続を開く。SQLiteは複数コネクションからの
// 同時書き込みに弱いため、Python版(sqlite3の単一コネクション運用)と同等の
// 安全性を保つために最大コネクション数を1に制限する。
func OpenDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)

	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("enable foreign_keys: %w", err)
	}
	if _, err := db.Exec("PRAGMA busy_timeout = 5000"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set busy_timeout: %w", err)
	}
	return db, nil
}

// InitSchema はスキーマを作成し、旧バージョンからのマイグレーションを適用する。
// db.py の init_db() と同じ内容(CREATE TABLE IF NOT EXISTS + 2種のマイグレーション)。
func InitSchema(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, schemaSQL); err != nil {
		return fmt.Errorf("create schema: %w", err)
	}

	// 旧スキーマ(status='open'/'done'の2値)からのマイグレーション。
	// カンバン化(todo/in_progress/reviewing/done の4値)に合わせて既存データを寄せる。
	if _, err := db.ExecContext(ctx, "UPDATE tasks SET status = 'todo' WHERE status = 'open'"); err != nil {
		return fmt.Errorf("migrate legacy status: %w", err)
	}

	// 既存DB(due_date列がまだ無いもの)へのマイグレーション。
	hasDueDate, err := columnExists(ctx, db, "tasks", "due_date")
	if err != nil {
		return fmt.Errorf("check due_date column: %w", err)
	}
	if !hasDueDate {
		if _, err := db.ExecContext(ctx, "ALTER TABLE tasks ADD COLUMN due_date TEXT"); err != nil {
			return fmt.Errorf("add due_date column: %w", err)
		}
	}

	// 既存DB(cycle_start_date列がまだ無いもの)へのマイグレーション。
	hasCycleStartDate, err := columnExists(ctx, db, "tasks", "cycle_start_date")
	if err != nil {
		return fmt.Errorf("check cycle_start_date column: %w", err)
	}
	if !hasCycleStartDate {
		if _, err := db.ExecContext(ctx, "ALTER TABLE tasks ADD COLUMN cycle_start_date TEXT"); err != nil {
			return fmt.Errorf("add cycle_start_date column: %w", err)
		}
	}
	return nil
}

func columnExists(ctx context.Context, db *sql.DB, table, column string) (bool, error) {
	rows, err := db.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return false, err
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, ctype string
		var notNull, pk int
		var dfltValue sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notNull, &dfltValue, &pk); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}
	return false, rows.Err()
}
