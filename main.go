// タスク管理自動化パイロットの管理画面(Go + sqlc + htmx)。
//
// 外部通信は一切行わない。JIRA連携タスクのクローズ操作は「本来ここでJIRA APIの
// ステータス遷移を呼ぶ」ことが分かるようstubJiraTransition()でログ出力するのみで、
// 実際のHTTPリクエストは送らない(本番実装時にJIRA REST API呼び出しへ差し替える想定)。
//
// ローカル実行(Docker経由): compose/docker-compose.yml参照。
// 環境変数 TASK_DASHBOARD_HOST 等でホスト/ポートを上書きする。
package main

import (
	"context"
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"
	"time"

	"taskmanager/internal/taskstore"
	"taskmanager/internal/web"
)

//go:embed static
var embeddedStatic embed.FS

func main() {
	dbPath := getEnv("TASK_DASHBOARD_DB_PATH", "task_dashboard.db")
	db, err := taskstore.OpenDB(dbPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := taskstore.InitSchema(ctx, db); err != nil {
		log.Fatalf("init schema: %v", err)
	}

	if getEnv("TASK_DASHBOARD_AUTO_SEED", "1") == "1" {
		count, err := taskstore.New(db).CountTasks(ctx)
		if err != nil {
			log.Fatalf("count tasks: %v", err)
		}
		if count == 0 {
			if err := taskstore.Seed(ctx, db); err != nil {
				log.Fatalf("seed: %v", err)
			}
			log.Println("サンプルデータを投入しました（外部通信なし）")
		}
	}

	// 週次サイクルの繰り越し。サーバー起動時にも即座に1回実行することで、サーバー停止中に
	// 週境界(月曜0:00 JST)をまたいでいた場合を補完する(docs/adr/proposals/cycle-rollover-execution.md参照)。
	if affected, err := taskstore.RunRollover(ctx, taskstore.New(db), time.Now()); err != nil {
		log.Printf("initial rollover failed (continuing): %v", err)
	} else if affected > 0 {
		log.Printf("起動時ロールオーバー: %d件のタスクを繰り越しました", affected)
	}
	taskstore.StartRolloverLoop(ctx, db)

	staticFS, err := fs.Sub(embeddedStatic, "static")
	if err != nil {
		log.Fatalf("static fs: %v", err)
	}

	mux := web.NewServer(db).Routes()
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	host := getEnv("TASK_DASHBOARD_HOST", "127.0.0.1")
	port := getEnv("TASK_DASHBOARD_PORT", "5000")
	addr := host + ":" + port

	log.Printf("listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
