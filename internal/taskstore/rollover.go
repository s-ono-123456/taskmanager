package taskstore

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// jst はJST(UTC+9)。日本はDSTを採用していないため、tzdataに依存する
// time.LoadLocation("Asia/Tokyo")を避け、time.FixedZoneで正確かつ軽量に表現する
// (distroless/static-debian12の実行イメージにはtzdataが含まれないため)。
var jst = time.FixedZone("JST", 9*60*60)

// CurrentWeekMonday は基準時刻tを含む週の月曜日(JST 0:00始まり)をYYYY-MM-DD文字列で返す。
func CurrentWeekMonday(t time.Time) string {
	jt := t.In(jst)
	offset := (int(jt.Weekday()) + 6) % 7 // Mon=1->0, ..., Sun=0->6
	monday := time.Date(jt.Year(), jt.Month(), jt.Day(), 0, 0, 0, 0, jst).AddDate(0, 0, -offset)
	return monday.Format("2006-01-02")
}

// RunRollover は「今週」(cycle_start_date IS NOT NULL)に属し未完了(status != done)の
// タスクを、現在の週の月曜日へ繰り越す。doneのタスクはWHERE句で除外され、
// cycle_start_dateは元のまま据え置かれる(直近7日ウィンドウで自然に非表示化される)。
// 戻り値は実際に更新された件数(ログ用)。
func RunRollover(ctx context.Context, q *Queries, now time.Time) (int64, error) {
	affected, err := q.RolloverCycles(ctx, sql.NullString{String: CurrentWeekMonday(now), Valid: true})
	if err != nil {
		return 0, fmt.Errorf("rollover cycles: %w", err)
	}
	return affected, nil
}

// RolloverCheckInterval: 常駐goroutineが繰り越し判定を行う間隔。週境界(月曜0:00 JST)ちょうど
// に厳密である必要はなく(UPDATE文自体が冪等)、数分〜十数分程度の遅延は許容できるため、
// DB負荷とのバランスでこの値にしている。
const RolloverCheckInterval = 15 * time.Minute

// StartRolloverLoop は常駐goroutineを起動し、RolloverCheckInterval間隔でRunRolloverを
// 呼び続ける。ctxがキャンセルされると停止する。呼び出し元(main.go)は、起動直後に
// RunRolloverを1回同期実行してから呼ぶこと(サーバー停止中に週境界をまたいだ場合の補完)。
func StartRolloverLoop(ctx context.Context, db *sql.DB) {
	q := New(db)
	ticker := time.NewTicker(RolloverCheckInterval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if affected, err := RunRollover(ctx, q, time.Now()); err != nil {
					fmt.Printf("[rollover] エラー: %v\n", err)
				} else if affected > 0 {
					fmt.Printf("[rollover] %d件のタスクを繰り越しました\n", affected)
				}
			}
		}
	}()
}
