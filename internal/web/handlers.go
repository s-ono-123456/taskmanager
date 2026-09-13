package web

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"taskmanager/internal/taskstore"
)

// Server はDB接続とsqlc生成クエリを保持し、7つのルートを提供する。
type Server struct {
	db *sql.DB
	q  *taskstore.Queries
}

func NewServer(db *sql.DB) *Server {
	return &Server{db: db, q: taskstore.New(db)}
}

// Routes はルーティングを登録したServeMuxを返す。Go 1.22+のServeMux
// (メソッド+パスパラメータ対応)のみを使い、追加のルーターライブラリは使わない。
func (s *Server) Routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", s.handleBoard)
	mux.HandleFunc("POST /tasks/new", s.handleNewTask)
	mux.HandleFunc("POST /tasks/{id}/edit", s.handleEditTask)
	mux.HandleFunc("POST /tasks/{id}/move", s.handleMoveTask)
	mux.HandleFunc("POST /tasks/{id}/track", s.handleToggleTrack)
	mux.HandleFunc("POST /candidates/{id}/approve", s.handleApproveCandidate)
	mux.HandleFunc("POST /candidates/{id}/reject", s.handleRejectCandidate)
	mux.HandleFunc("POST /task-candidates/{id}/approve", s.handleApproveTaskCandidate)
	mux.HandleFunc("POST /task-candidates/{id}/reject", s.handleRejectTaskCandidate)
	return mux
}

func parseTaskID(r *http.Request) (int64, error) {
	return strconv.ParseInt(r.PathValue("id"), 10, 64)
}

func parseCandidateID(r *http.Request) (int64, error) {
	return strconv.ParseInt(r.PathValue("id"), 10, 64)
}

// filterFromRequest はGETならクエリパラメータ、POSTならフォーム値からフィルタ状態を読む。
//
// POST側は "filter_target"/"filter_show_untracked" という専用キー名を使う
// (テンプレート側でhx-valsにより付与)。edit/newフォーム自身が"target"という
// タスクの対象を表すフィールドを持つため、フィルタも同じ"target"キーで送ると
// 値が衝突してしまう(hx-includeでフィルタ用select/checkboxを素直に含めると
// 名前が重複する)ための回避策。
func filterFromRequest(r *http.Request) BoardFilter {
	var target, showUntracked string
	if r.Method == http.MethodGet {
		target = r.URL.Query().Get("target")
		showUntracked = r.URL.Query().Get("show_untracked")
	} else {
		target = r.FormValue("filter_target")
		showUntracked = r.FormValue("filter_show_untracked")
	}
	return BoardFilter{Target: target, ShowUntracked: showUntracked == "1"}
}

// respondBoard はPOST操作後、ボード全体フラグメント+トーストを200で返す
// (成功・失敗どちらもこの関数を通る。htmxは4xx/5xxを自動スワップしないため常に200)。
func (s *Server) respondBoard(w http.ResponseWriter, r *http.Request, filter BoardFilter, toast *Toast) {
	data, err := LoadBoardData(r.Context(), s.q, filter)
	if err != nil {
		log.Printf("load board data: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	data.Toast = toast
	RenderFragment(w, data)
}

func (s *Server) handleBoard(w http.ResponseWriter, r *http.Request) {
	filter := filterFromRequest(r)
	data, err := LoadBoardData(r.Context(), s.q, filter)
	if err != nil {
		log.Printf("load board data: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	RenderPage(w, data)
}

func (s *Server) handleNewTask(w http.ResponseWriter, r *http.Request) {
	filter := filterFromRequest(r)
	title := strings.TrimSpace(r.FormValue("title"))
	target := r.FormValue("target")
	priority := r.FormValue("priority")
	if priority == "" {
		priority = "medium"
	}

	if title == "" {
		s.respondBoard(w, r, filter, &Toast{Category: "error", Message: "タイトルは必須です"})
		return
	}
	if !isValidTarget(target) {
		s.respondBoard(w, r, filter, &Toast{Category: "error", Message: "対象プロジェクトの値が不正です"})
		return
	}
	if !isValidPriority(priority) {
		s.respondBoard(w, r, filter, &Toast{Category: "error", Message: "優先度の値が不正です"})
		return
	}

	_, err := s.q.CreateTask(r.Context(), taskstore.CreateTaskParams{
		Title:       title,
		Description: nullStrIfNotEmpty(r.FormValue("description")),
		Target:      target,
		Status:      "todo",
		CreatedAt:   nowISO(),
		Tracked:     1,
		DueDate:     nullStrIfNotEmpty(r.FormValue("due_date")),
		Priority:    priority,
		// 新規作成タスクは常にバックログ固定(仕様)。CycleStartDateは未指定のまま
		// ゼロ値sql.NullString{}(NULL)とする。
	})
	if err != nil {
		log.Printf("create task: %v", err)
		s.respondBoard(w, r, filter, &Toast{Category: "error", Message: "タスクの作成に失敗しました"})
		return
	}

	if target == "jira_a" || target == "jira_b" {
		fmt.Printf(
			"[stub] JIRA API呼び出し想定: jira_key=(未発行) action=create target=%s"+
				"（実際の通信なし。手動追加のためJIRA起票は未実施）\n",
			target,
		)
	}

	s.respondBoard(w, r, filter, &Toast{Category: "success", Message: fmt.Sprintf("タスクを追加しました: %s", title)})
}

func (s *Server) handleEditTask(w http.ResponseWriter, r *http.Request) {
	filter := filterFromRequest(r)
	id, err := parseTaskID(r)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	task, err := s.q.GetTask(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		s.respondBoard(w, r, filter, &Toast{Category: "error", Message: "タスクが見つかりません"})
		return
	} else if err != nil {
		log.Printf("get task: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	title := strings.TrimSpace(r.FormValue("title"))
	target := r.FormValue("target")
	status := r.FormValue("status")
	priority := r.FormValue("priority")
	if priority == "" {
		priority = "medium"
	}

	if title == "" {
		s.respondBoard(w, r, filter, &Toast{Category: "error", Message: "タイトルは必須です"})
		return
	}
	if !isValidTarget(target) {
		s.respondBoard(w, r, filter, &Toast{Category: "error", Message: "対象プロジェクトの値が不正です"})
		return
	}
	if !isValidStatus(status) {
		s.respondBoard(w, r, filter, &Toast{Category: "error", Message: "状態の値が不正です"})
		return
	}
	if !isValidPriority(priority) {
		s.respondBoard(w, r, filter, &Toast{Category: "error", Message: "優先度の値が不正です"})
		return
	}

	closedAt := closedAtForTransition(task.Status, status, task.ClosedAt)

	if err := s.q.UpdateTask(r.Context(), taskstore.UpdateTaskParams{
		Title:       title,
		Description: nullStrIfNotEmpty(r.FormValue("description")),
		Target:      target,
		Status:      status,
		ClosedAt:    closedAt,
		DueDate:     nullStrIfNotEmpty(r.FormValue("due_date")),
		Priority:    priority,
		ID:          id,
	}); err != nil {
		log.Printf("update task: %v", err)
		s.respondBoard(w, r, filter, &Toast{Category: "error", Message: "更新に失敗しました"})
		return
	}

	if task.JiraKey.Valid {
		stubJiraTransition(task.JiraKey.String, "update_fields")
	}

	s.respondBoard(w, r, filter, &Toast{Category: "success", Message: "更新しました"})
}

func (s *Server) handleMoveTask(w http.ResponseWriter, r *http.Request) {
	filter := filterFromRequest(r)
	id, err := parseTaskID(r)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	status := r.FormValue("status")
	if !isValidStatus(status) {
		s.respondBoard(w, r, filter, &Toast{Category: "error", Message: "状態の値が不正です"})
		return
	}
	cycle := r.FormValue("cycle")
	if !isValidCycle(cycle) {
		s.respondBoard(w, r, filter, &Toast{Category: "error", Message: "週の値が不正です"})
		return
	}

	task, err := s.q.GetTask(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		s.respondBoard(w, r, filter, &Toast{Category: "error", Message: "タスクが見つかりません"})
		return
	} else if err != nil {
		log.Printf("get task: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if task.JiraKey.Valid {
		stubJiraTransition(task.JiraKey.String, "move_to_"+status)
	}

	closedAt := closedAtForTransition(task.Status, status, task.ClosedAt)
	if err := s.q.UpdateTaskStatusAndCycle(r.Context(), taskstore.UpdateTaskStatusAndCycleParams{
		Status:         status,
		ClosedAt:       closedAt,
		CycleStartDate: cycleStartDateForLane(cycle),
		ID:             id,
	}); err != nil {
		log.Printf("update task status and cycle: %v", err)
		s.respondBoard(w, r, filter, &Toast{Category: "error", Message: "更新に失敗しました"})
		return
	}

	s.respondBoard(w, r, filter, &Toast{
		Category: "success",
		Message:  fmt.Sprintf("「%s」を%s・%sに移動しました", task.Title, LaneLabels[cycle], StatusLabels[status]),
	})
}

func (s *Server) handleToggleTrack(w http.ResponseWriter, r *http.Request) {
	filter := filterFromRequest(r)
	id, err := parseTaskID(r)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	task, err := s.q.GetTask(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		s.respondBoard(w, r, filter, &Toast{Category: "error", Message: "タスクが見つかりません"})
		return
	} else if err != nil {
		log.Printf("get task: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	var toast *Toast
	if task.Tracked != 0 {
		// 非表示にする時点で一旦完了扱いにする(既に完了済みなら完了日時は変更しない)。
		if task.Status == "done" {
			err = s.q.SetTaskUntrackedKeepStatus(r.Context(), id)
		} else {
			err = s.q.SetTaskUntrackedAsDone(r.Context(), taskstore.SetTaskUntrackedAsDoneParams{
				ClosedAt: sql.NullString{String: nowISO(), Valid: true},
				ID:       id,
			})
		}
		toast = &Toast{Category: "success", Message: "非表示にしました(完了扱いにしました)"}
	} else if task.JiraKey.Valid {
		// 再表示のタイミングでJIRAとの再同期を行う想定のスタブ(実際の通信なし)。
		stubJiraTransition(task.JiraKey.String, "resync")
		err = s.q.SetTaskTrackedAndSynced(r.Context(), taskstore.SetTaskTrackedAndSyncedParams{
			LastSyncedAt: sql.NullString{String: nowISO(), Valid: true},
			ID:           id,
		})
		toast = &Toast{Category: "success", Message: "再表示し、JIRAと再同期しました"}
	} else {
		err = s.q.SetTaskTracked(r.Context(), id)
		toast = &Toast{Category: "success", Message: "再表示しました"}
	}

	if err != nil {
		log.Printf("toggle track: %v", err)
		toast = &Toast{Category: "error", Message: "更新に失敗しました"}
	}

	s.respondBoard(w, r, filter, toast)
}

// handleApproveCandidate はクローズ要求一覧(論点C3)の「承認」ボタン。
// related_jira_keyがあればそのjira_keyでタスクを自動解決し、無ければフォームの
// task_id(ユーザーが<select>で選んだ対象タスク)を使う。
func (s *Server) handleApproveCandidate(w http.ResponseWriter, r *http.Request) {
	filter := filterFromRequest(r)
	id, err := parseCandidateID(r)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	candidate, err := s.q.GetCandidate(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		s.respondBoard(w, r, filter, &Toast{Category: "error", Message: "候補が見つかりません"})
		return
	} else if err != nil {
		log.Printf("get candidate: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if candidate.HumanVerdict.Valid && candidate.HumanVerdict.String != "" {
		s.respondBoard(w, r, filter, &Toast{Category: "error", Message: "この候補は既に処理済みです"})
		return
	}

	var task taskstore.Task
	if candidate.RelatedJiraKey.Valid && candidate.RelatedJiraKey.String != "" {
		task, err = s.q.GetTaskByJiraKey(r.Context(), sql.NullString{String: candidate.RelatedJiraKey.String, Valid: true})
		if errors.Is(err, sql.ErrNoRows) {
			s.respondBoard(w, r, filter, &Toast{
				Category: "error",
				Message:  fmt.Sprintf("対象タスク(%s)が見つかりません", candidate.RelatedJiraKey.String),
			})
			return
		} else if err != nil {
			log.Printf("get task by jira key: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
	} else {
		taskID, err := strconv.ParseInt(r.FormValue("task_id"), 10, 64)
		if err != nil {
			s.respondBoard(w, r, filter, &Toast{Category: "error", Message: "対象タスクを選択してください"})
			return
		}
		task, err = s.q.GetTask(r.Context(), taskID)
		if errors.Is(err, sql.ErrNoRows) {
			s.respondBoard(w, r, filter, &Toast{Category: "error", Message: "対象タスクが見つかりません"})
			return
		} else if err != nil {
			log.Printf("get task: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
	}

	if err := closeTask(r.Context(), s.q, task); err != nil {
		log.Printf("close task: %v", err)
		s.respondBoard(w, r, filter, &Toast{Category: "error", Message: "クローズに失敗しました"})
		return
	}
	if err := s.q.UpdateCandidateVerdict(r.Context(), taskstore.UpdateCandidateVerdictParams{
		HumanVerdict: sql.NullString{String: "correct", Valid: true},
		ID:           id,
	}); err != nil {
		log.Printf("update candidate verdict: %v", err)
		s.respondBoard(w, r, filter, &Toast{Category: "error", Message: "候補の更新に失敗しました"})
		return
	}

	s.respondBoard(w, r, filter, &Toast{
		Category: "success",
		Message:  fmt.Sprintf("「%s」をクローズしました", task.Title),
	})
}

// handleRejectCandidate はクローズ要求一覧(論点C3)の「却下」ボタン。
// candidates.human_verdictをfalse_positiveにするのみで、タスク側は変更しない。
func (s *Server) handleRejectCandidate(w http.ResponseWriter, r *http.Request) {
	filter := filterFromRequest(r)
	id, err := parseCandidateID(r)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	if err := s.q.UpdateCandidateVerdict(r.Context(), taskstore.UpdateCandidateVerdictParams{
		HumanVerdict: sql.NullString{String: "false_positive", Valid: true},
		ID:           id,
	}); err != nil {
		log.Printf("update candidate verdict: %v", err)
		s.respondBoard(w, r, filter, &Toast{Category: "error", Message: "更新に失敗しました"})
		return
	}

	s.respondBoard(w, r, filter, &Toast{Category: "success", Message: "候補を却下しました"})
}

// handleApproveTaskCandidate はタスク候補一覧の「承認」ボタン。
// クローズ要求と異なり既存タスクの特定は行わず、フォームで選ばれたtargetを使って
// 新規タスクを作成する(related_jira_keyによる自動解決に相当する分岐は無い)。
func (s *Server) handleApproveTaskCandidate(w http.ResponseWriter, r *http.Request) {
	filter := filterFromRequest(r)
	id, err := parseCandidateID(r)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	candidate, err := s.q.GetCandidate(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		s.respondBoard(w, r, filter, &Toast{Category: "error", Message: "候補が見つかりません"})
		return
	} else if err != nil {
		log.Printf("get candidate: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if candidate.HumanVerdict.Valid && candidate.HumanVerdict.String != "" {
		s.respondBoard(w, r, filter, &Toast{Category: "error", Message: "この候補は既に処理済みです"})
		return
	}

	target := r.FormValue("target")
	if !isValidTarget(target) {
		s.respondBoard(w, r, filter, &Toast{Category: "error", Message: "対象を選択してください"})
		return
	}

	task, err := createTaskFromCandidate(r.Context(), s.q, candidate, target)
	if err != nil {
		log.Printf("create task from candidate: %v", err)
		s.respondBoard(w, r, filter, &Toast{Category: "error", Message: "タスクの作成に失敗しました"})
		return
	}
	if err := s.q.UpdateCandidateVerdict(r.Context(), taskstore.UpdateCandidateVerdictParams{
		HumanVerdict: sql.NullString{String: "correct", Valid: true},
		ID:           id,
	}); err != nil {
		log.Printf("update candidate verdict: %v", err)
		s.respondBoard(w, r, filter, &Toast{Category: "error", Message: "候補の更新に失敗しました"})
		return
	}

	s.respondBoard(w, r, filter, &Toast{
		Category: "success",
		Message:  fmt.Sprintf("タスクを追加しました: %s", task.Title),
	})
}

// handleRejectTaskCandidate はタスク候補一覧の「却下」ボタン。
// candidates.human_verdictをfalse_positiveにするのみで、タスクは作成しない。
func (s *Server) handleRejectTaskCandidate(w http.ResponseWriter, r *http.Request) {
	filter := filterFromRequest(r)
	id, err := parseCandidateID(r)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	if err := s.q.UpdateCandidateVerdict(r.Context(), taskstore.UpdateCandidateVerdictParams{
		HumanVerdict: sql.NullString{String: "false_positive", Valid: true},
		ID:           id,
	}); err != nil {
		log.Printf("update candidate verdict: %v", err)
		s.respondBoard(w, r, filter, &Toast{Category: "error", Message: "更新に失敗しました"})
		return
	}

	s.respondBoard(w, r, filter, &Toast{Category: "success", Message: "候補を却下しました"})
}
