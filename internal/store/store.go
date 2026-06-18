package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// Job 持久化的任务记录（与 main.go 的 Job 结构对齐）
type Job struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Theme       string   `json:"theme"`
	StoryType   string   `json:"story_type"`
	Style       string   `json:"style"`
	PageCount   int      `json:"page_count"`
	Status      string   `json:"status"`
	Phase       string   `json:"phase"`
	Total       int      `json:"total"`
	Done        int      `json:"done"`
	UpdatedAt   string   `json:"updated_at"`
	StartedAt   string   `json:"started_at"`
	StoryPath   string   `json:"story_path"`
	StoryFolder string   `json:"story_folder"`
	CoverURL    string   `json:"cover_url"`
	Error       string   `json:"error"`
	Logs        []string `json:"logs"`
	// DonePages 已完成页码列表（用于断点续做）
	DonePages []int `json:"done_pages,omitempty"`
}

// Store SQLite 持久化存储
type Store struct {
	db *sql.DB
	mu sync.Mutex // 保护写操作（SQLite 单写）

	// 进度内存缓冲：避免每 emit 一次 progress 就写一次 DB
	progMu     sync.Mutex
	pendingProg map[string]jobProgress
	// lastPhase 记录每个 job 上次日志记录的 phase，避免重复追加
	progPhaseMu sync.Mutex
	lastPhase   map[string]string

	// 状态变化：状态字段（status/error/path/cover）变化时调用，要求立即落库
	// 进度字段（total/done/phase/title）走 pendingProg 缓冲
	stopCh chan struct{}
	wg     sync.WaitGroup
}

// jobProgress 进度快照（仅这些字段走内存缓冲）
type jobProgress struct {
	Total int
	Done  int
	Phase string
	Title string
}

// schema 完整版本（含 done_pages，用于断点续做）
const schema = `
CREATE TABLE IF NOT EXISTS jobs (
	id          TEXT PRIMARY KEY,
	title       TEXT NOT NULL DEFAULT '',
	theme       TEXT NOT NULL DEFAULT '',
	story_type  TEXT NOT NULL DEFAULT '',
	style       TEXT NOT NULL DEFAULT '',
	page_count  INTEGER NOT NULL DEFAULT 0,
	status      TEXT NOT NULL DEFAULT 'running',
	phase       TEXT NOT NULL DEFAULT '',
	total       INTEGER NOT NULL DEFAULT 0,
	done        INTEGER NOT NULL DEFAULT 0,
	updated_at  TEXT NOT NULL DEFAULT '',
	started_at  TEXT NOT NULL DEFAULT '',
	story_path  TEXT NOT NULL DEFAULT '',
	story_folder TEXT NOT NULL DEFAULT '',
	cover_url   TEXT NOT NULL DEFAULT '',
	error_msg   TEXT NOT NULL DEFAULT '',
	logs        TEXT NOT NULL DEFAULT '[]',
	done_pages  TEXT NOT NULL DEFAULT '[]',
	created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_jobs_status ON jobs(status);
CREATE INDEX IF NOT EXISTS idx_jobs_created_at ON jobs(created_at);
`

// migrations 老 DB 升级：加缺失的列
var migrations = []struct {
	column string
	def    string
}{
	{"done_pages", "TEXT NOT NULL DEFAULT '[]'"},
}

// progressFlushInterval 进度批量落库周期
const progressFlushInterval = 500 * time.Millisecond

// Open 打开（或创建）SQLite 数据库
func Open(dbPath string) (*Store, error) {
	if dbPath == "" {
		dbPath = "storybook.db"
	}
	// 确保目录存在
	dir := filepath.Dir(dbPath)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("create db dir: %w", err)
		}
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// 性能优化：WAL 模式 + 外键
	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA foreign_keys=ON",
		"PRAGMA busy_timeout=5000",
	} {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("pragma %q: %w", pragma, err)
		}
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("create schema: %w", err)
	}
	// 迁移：给已存在的 jobs 表加新列
	for _, m := range migrations {
		if err := addColumnIfMissing(db, "jobs", m.column, m.def); err != nil {
			db.Close()
			return nil, fmt.Errorf("migrate %s: %w", m.column, err)
		}
	}
	s := &Store{
		db:          db,
		pendingProg: make(map[string]jobProgress),
		lastPhase:   make(map[string]string),
		stopCh:      make(chan struct{}),
	}
	s.wg.Add(1)
	go s.flushLoop(progressFlushInterval)
	return s, nil
}

// flushLoop 后台定时把内存里的进度批量写进 DB
func (s *Store) flushLoop(interval time.Duration) {
	defer s.wg.Done()
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-s.stopCh:
			s.flushAllProgress()
			return
		case <-t.C:
			s.flushAllProgress()
		}
	}
}

// flushAllProgress 落库所有待写进度（只写发生过变化的）
func (s *Store) flushAllProgress() {
	s.progMu.Lock()
	pending := s.pendingProg
	s.pendingProg = make(map[string]jobProgress)
	s.progMu.Unlock()

	if len(pending) == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().Format(time.RFC3339)
	for id, p := range pending {
		_, _ = s.db.Exec(`UPDATE jobs SET total=?, done=?, phase=?,
			title=COALESCE(NULLIF(?, ''), title), updated_at=? WHERE id=?`,
			p.Total, p.Done, p.Phase, p.Title, now, id)
	}
}

// Close 关闭数据库（先做最后一次落库）
func (s *Store) Close() error {
	close(s.stopCh)
	s.wg.Wait()
	return s.db.Close()
}

// addColumnIfMissing 给表加新列（如果不存在）
func addColumnIfMissing(db *sql.DB, table, column, def string) error {
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return err
		}
		if name == column {
			return nil // 已存在
		}
	}
	_, err = db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, def))
	return err
}

// CreateJob 创建新任务
func (s *Store) CreateJob(j *Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	logsJSON, _ := json.Marshal(j.Logs)
	_, err := s.db.Exec(`INSERT INTO jobs
		(id, title, theme, story_type, style, page_count, status, phase, total, done,
		 updated_at, started_at, story_path, story_folder, cover_url, error_msg, logs)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		j.ID, j.Title, j.Theme, j.StoryType, j.Style, j.PageCount,
		j.Status, j.Phase, j.Total, j.Done,
		j.UpdatedAt, j.StartedAt, j.StoryPath, j.StoryFolder, j.CoverURL, j.Error,
		string(logsJSON),
	)
	return err
}

// UpdateJob 更新任务字段（只更新传入的字段）
// 重要：调用前会自动 flush 该 job 的内存进度缓冲，避免读到陈旧数据
func (s *Store) UpdateJob(id string, update func(j *Job)) error {
	s.FlushProgress(id)

	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	row := tx.QueryRow(`SELECT
		id, title, theme, story_type, style, page_count, status, phase, total, done,
		updated_at, started_at, story_path, story_folder, cover_url, error_msg, logs
		FROM jobs WHERE id = ?`, id)

	var j Job
	var logsJSON string
	err = row.Scan(
		&j.ID, &j.Title, &j.Theme, &j.StoryType, &j.Style, &j.PageCount,
		&j.Status, &j.Phase, &j.Total, &j.Done,
		&j.UpdatedAt, &j.StartedAt, &j.StoryPath, &j.StoryFolder, &j.CoverURL, &j.Error,
		&logsJSON,
	)
	if err == sql.ErrNoRows {
		return fmt.Errorf("job %s not found", id)
	}
	if err != nil {
		return err
	}
	json.Unmarshal([]byte(logsJSON), &j.Logs)

	// 应用更新
	update(&j)
	j.UpdatedAt = time.Now().Format(time.RFC3339)

	newLogsJSON, _ := json.Marshal(j.Logs)
	_, err = tx.Exec(`UPDATE jobs SET
		title=?, theme=?, story_type=?, style=?, page_count=?, status=?, phase=?,
		total=?, done=?, updated_at=?, started_at=?, story_path=?, story_folder=?,
		cover_url=?, error_msg=?, logs=?
		WHERE id=?`,
		j.Title, j.Theme, j.StoryType, j.Style, j.PageCount,
		j.Status, j.Phase, j.Total, j.Done,
		j.UpdatedAt, j.StartedAt, j.StoryPath, j.StoryFolder, j.CoverURL, j.Error,
		string(newLogsJSON), id,
	)
	if err != nil {
		return err
	}
	return tx.Commit()
}

// GetJob 获取单个任务
func (s *Store) GetJob(id string) (*Job, error) {
	row := s.db.QueryRow(`SELECT
		id, title, theme, story_type, style, page_count, status, phase, total, done,
		updated_at, started_at, story_path, story_folder, cover_url, error_msg, logs
		FROM jobs WHERE id = ?`, id)

	var j Job
	var logsJSON string
	err := row.Scan(
		&j.ID, &j.Title, &j.Theme, &j.StoryType, &j.Style, &j.PageCount,
		&j.Status, &j.Phase, &j.Total, &j.Done,
		&j.UpdatedAt, &j.StartedAt, &j.StoryPath, &j.StoryFolder, &j.CoverURL, &j.Error,
		&logsJSON,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	json.Unmarshal([]byte(logsJSON), &j.Logs)
	return &j, nil
}

// ListJobs 列出所有任务（按创建时间倒序）
func (s *Store) ListJobs() ([]*Job, error) {
	rows, err := s.db.Query(`SELECT
		id, title, theme, story_type, style, page_count, status, phase, total, done,
		updated_at, started_at, story_path, story_folder, cover_url, error_msg, logs
		FROM jobs ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []*Job
	for rows.Next() {
		var j Job
		var logsJSON string
		if err := rows.Scan(
			&j.ID, &j.Title, &j.Theme, &j.StoryType, &j.Style, &j.PageCount,
			&j.Status, &j.Phase, &j.Total, &j.Done,
			&j.UpdatedAt, &j.StartedAt, &j.StoryPath, &j.StoryFolder, &j.CoverURL, &j.Error,
			&logsJSON,
		); err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(logsJSON), &j.Logs)
		jobs = append(jobs, &j)
	}
	return jobs, rows.Err()
}

// DeleteJob 删除任务
func (s *Store) DeleteJob(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec("DELETE FROM jobs WHERE id = ?", id)
	return err
}

// CleanupStaleJobs 将所有 running 状态的任务标记为 failed（启动时调用）
func (s *Store) CleanupStaleJobs() (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result, err := s.db.Exec(`UPDATE jobs SET status='failed', phase='服务重启中断',
		updated_at=? WHERE status='running'`, time.Now().Format(time.RFC3339))
	if err != nil {
		return 0, err
	}
	n, _ := result.RowsAffected()
	return int(n), nil
}

// === 进度缓冲（节流落库）=============================================

// EnqueueProgress 把进度更新写入内存缓冲，500ms 内合并后批量落库
// 适用于高频更新（如图片生成 worker 每完成一页调用一次）
// 状态字段（status/error/path/cover）请用 UpdateJob
func (s *Store) EnqueueProgress(id string, total, done int, phase, title string) {
	s.progMu.Lock()
	// 覆盖：同一个 job 的多次更新只保留最后一次
	s.pendingProg[id] = jobProgress{
		Total: total,
		Done:  done,
		Phase: phase,
		Title: title,
	}
	s.progMu.Unlock()
}

// FlushProgress 同步落库指定 job 的内存进度（用于状态变化前）
func (s *Store) FlushProgress(id string) {
	s.progMu.Lock()
	p, ok := s.pendingProg[id]
	if ok {
		delete(s.pendingProg, id)
	}
	s.progMu.Unlock()
	if !ok {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, _ = s.db.Exec(`UPDATE jobs SET total=?, done=?, phase=?,
		title=COALESCE(NULLIF(?, ''), title), updated_at=? WHERE id=?`,
		p.Total, p.Done, p.Phase, p.Title, time.Now().Format(time.RFC3339), id)
}

// AppendLog 追加一条日志（单条 UPDATE，不读全表）
// 用于 phase 切换、关键事件等需要持久化但频率低的场景
func (s *Store) AppendLog(id, entry string) error {
	s.FlushProgress(id)
	s.mu.Lock()
	defer s.mu.Unlock()

	// 用 SQL 直接 append，避免 read-modify-write
	now := time.Now().Format(time.RFC3339)
	_, err := s.db.Exec(`
		UPDATE jobs
		SET logs = CASE
				WHEN logs = '[]' OR logs = '' THEN ?
				ELSE substr(logs, 1, length(logs) - 1) || ',' || ? || ']'
			END,
			updated_at = ?
		WHERE id = ?`,
		entry, entry, now, id)
	return err
}

// AppendLogIfPhaseChanged 仅当 phase 切换时才追加日志
// 解决图片生成阶段每页都 emit progress 导致的日志爆炸
func (s *Store) AppendLogIfPhaseChanged(id, phase string, done, total int) {
	s.progPhaseMu.Lock()
	prev, seen := s.lastPhase[id]
	s.lastPhase[id] = phase
	s.progPhaseMu.Unlock()

	if !seen || prev != phase {
		_ = s.AppendLog(id, fmt.Sprintf("→ %s (%d/%d)", phase, done, total))
	}
}

// MarkDonePages 把指定页标记为已完成（用于断点续做）
func (s *Store) MarkDonePages(id string, pages []int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, _ := json.Marshal(pages)
	now := time.Now().Format(time.RFC3339)
	_, err := s.db.Exec(`UPDATE jobs SET done_pages=?, updated_at=? WHERE id=?`,
		string(data), now, id)
	return err
}

// ResetJobTrackers 清理内存里某个 job 的所有临时状态（pending 进度、最后 phase）
// 用于删除任务、任务结束后释放内存
func (s *Store) ResetJobTrackers(id string) {
	s.progMu.Lock()
	delete(s.pendingProg, id)
	s.progMu.Unlock()
	s.progPhaseMu.Lock()
	delete(s.lastPhase, id)
	s.progPhaseMu.Unlock()
}
