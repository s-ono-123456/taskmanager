package web

import (
	"embed"
	"html/template"
	"log"
	"net/http"
)

//go:embed templates/*.tmpl
var templateFS embed.FS

var accentBarClass = map[string]string{
	"todo":        "bg-slate-400",
	"in_progress": "bg-blue-500",
	"reviewing":   "bg-violet-500",
	"done":        "bg-emerald-500",
}

var accentPillClass = map[string]string{
	"todo":        "bg-slate-200 text-slate-700 dark:bg-slate-700 dark:text-slate-200",
	"in_progress": "bg-blue-100 text-blue-700 dark:bg-blue-900/50 dark:text-blue-200",
	"reviewing":   "bg-violet-100 text-violet-700 dark:bg-violet-900/50 dark:text-violet-200",
	"done":        "bg-emerald-100 text-emerald-700 dark:bg-emerald-900/50 dark:text-emerald-200",
}

var targetPillClass = map[string]string{
	"jira_a":   "bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-200",
	"jira_b":   "bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-200",
	"personal": "bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-200",
}

var priorityPillClass = map[string]string{
	"highest": "bg-rose-100 text-rose-700 dark:bg-rose-900/40 dark:text-rose-200",
	"high":    "bg-orange-100 text-orange-700 dark:bg-orange-900/40 dark:text-orange-200",
	"medium":  "bg-slate-100 text-slate-600 dark:bg-slate-700 dark:text-slate-300",
	"low":     "bg-sky-100 text-sky-700 dark:bg-sky-900/40 dark:text-sky-200",
}

var funcMap = template.FuncMap{
	"isOverdue":         isOverdue,
	"accentBarClass":    func(status string) string { return accentBarClass[status] },
	"accentPillClass":   func(status string) string { return accentPillClass[status] },
	"targetPillClass":   func(target string) string { return targetPillClass[target] },
	"statusLabel":       func(status string) string { return StatusLabels[status] },
	"priorityPillClass": func(priority string) string { return priorityPillClass[priority] },
	"priorityLabel":     func(priority string) string { return PriorityLabels[priority] },
}

var templates = template.Must(template.New("").Funcs(funcMap).ParseFS(templateFS, "templates/*.tmpl"))

// RenderPage はGET /用のフルページを描画する。
func RenderPage(w http.ResponseWriter, data BoardData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := templates.ExecuteTemplate(w, "page", data); err != nil {
		log.Printf("render page error: %v", err)
	}
}

// RenderFragment はPOST操作後に返すHTMLフラグメント(ボード本体+トーストのoob swap)を描画する。
// バリデーションエラー時も含め、常にHTTP 200で返す(htmxは4xx/5xxを自動スワップしないため)。
// 成功/失敗はX-Toast-Categoryヘッダーで伝える(HTTPステータスが常に200のため、
// htmxのevent.detail.successfulでは判定できない。モーダルを閉じるかどうかの
// 判定にテンプレート側のJSから使う)。
func RenderFragment(w http.ResponseWriter, data BoardData) {
	if data.Toast != nil {
		w.Header().Set("X-Toast-Category", data.Toast.Category)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := templates.ExecuteTemplate(w, "boardAndToast", data); err != nil {
		log.Printf("render fragment error: %v", err)
	}
}
