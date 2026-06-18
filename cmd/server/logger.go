package main

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// 日志相关常量
const (
	maxLogFiles  = 7  // 最多保留 7 天日志
	maxLogSizeMB = 10 // 单个日志文件最大 10MB
)

var (
	// rootJobLogger 全局 job 事件 logger
	rootJobLogger *slog.Logger
	// logFileMu 保护 logFile + writers（log rotation 用）
	logFileMu sync.Mutex
	// currentFile 当前日志文件句柄
	currentFile *os.File
	// currentSize 当前文件已写入字节数（用于触发 rotation）
	currentSize atomic.Int64
	// safeMW 包装 stdout + file 的多写器，自带锁，避免 rotation 时与并发写竞争
	safeMW *safeMultiWriter
)

// safeMultiWriter 线程安全的多写器
// 解决原版 checkLogRotation 与 log.Print 之间的 data race
type safeMultiWriter struct {
	mu      sync.RWMutex
	writers []io.Writer
}

func (m *safeMultiWriter) Write(p []byte) (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, w := range m.writers {
		n, err := w.Write(p)
		if err != nil {
			return n, err
		}
		if n < len(p) {
			return n, io.ErrShortWrite
		}
	}
	return len(p), nil
}

func (m *safeMultiWriter) swap(writers []io.Writer) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.writers = writers
}

// initLogger 初始化结构化日志（slog JSON）
// - 输出到 stdout + 当天日志文件
// - 多写器加锁，无 data race
// - 启动时清理 7 天前的旧日志
func initLogger(dir string) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "日志目录创建失败: %v\n", err)
		return
	}
	cleanOldLogs(dir)

	safeMW = &safeMultiWriter{writers: []io.Writer{os.Stdout}}
	handler := slog.NewJSONHandler(safeMW, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	logger := slog.New(handler).With(
		slog.String("app", "storybook"),
	)
	slog.SetDefault(logger)
	rootJobLogger = logger

	openTodayLogFile(dir)
}

// openTodayLogFile 打开（或重新打开）今天的日志文件并加入 writers
func openTodayLogFile(dir string) {
	logFileMu.Lock()
	defer logFileMu.Unlock()

	logPath := filepath.Join(dir, fmt.Sprintf("storybook-%s.log", time.Now().Format("2006-01-02")))
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "日志文件打开失败: %v\n", err)
		return
	}
	info, _ := f.Stat()
	if info != nil {
		currentSize.Store(info.Size())
	}

	// 关闭旧文件（如果存在）
	if currentFile != nil {
		currentFile.Close()
	}
	currentFile = f

	// 重置 writers：stdout + 新文件
	safeMW.swap([]io.Writer{os.Stdout, f})
}

// closeLogger 关闭日志
func closeLogger() {
	logFileMu.Lock()
	defer logFileMu.Unlock()
	if currentFile != nil {
		currentFile.Close()
		currentFile = nil
	}
}

// cleanOldLogs 清理超过 maxLogFiles 天的旧日志
func cleanOldLogs(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	var logFiles []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), "storybook-") && strings.HasSuffix(e.Name(), ".log") {
			logFiles = append(logFiles, e.Name())
		}
	}
	if len(logFiles) <= maxLogFiles {
		return
	}
	sort.Strings(logFiles)
	for _, name := range logFiles[:len(logFiles)-maxLogFiles] {
		os.Remove(filepath.Join(dir, name))
	}
}

// checkLogRotation 每次写日志前检查 size，超限就轮转
func checkLogRotation() {
	if currentSize.Load() < int64(maxLogSizeMB)*1024*1024 {
		return
	}
	dir := ""
	if currentFile != nil {
		dir = filepath.Dir(currentFile.Name())
	}
	if dir == "" {
		return
	}
	openTodayLogFile(dir)
}

// LogJob 记录任务关键节点（结构化字段便于日志聚合）
func LogJob(jobID, event, detail string) {
	checkLogRotation()
	if rootJobLogger == nil {
		return
	}
	rootJobLogger.Info("job_event",
		slog.String("job_id", jobID),
		slog.String("event", event),
		slog.String("detail", detail),
	)
}

// LogJobError 记录任务错误（warn 级别）
func LogJobError(jobID, event string, err error) {
	checkLogRotation()
	if rootJobLogger == nil {
		return
	}
	rootJobLogger.Warn("job_error",
		slog.String("job_id", jobID),
		slog.String("event", event),
		slog.String("err", errString(err)),
	)
}

// LogJobWarn 记录任务警告
func LogJobWarn(jobID, event, detail string) {
	checkLogRotation()
	if rootJobLogger == nil {
		return
	}
	rootJobLogger.Warn("job_warn",
		slog.String("job_id", jobID),
		slog.String("event", event),
		slog.String("detail", detail),
	)
}

// LogJobFatal 记录致命错误（error 级别）
func LogJobFatal(jobID, event string, err error) {
	checkLogRotation()
	if rootJobLogger == nil {
		return
	}
	rootJobLogger.Error("job_fatal",
		slog.String("job_id", jobID),
		slog.String("event", event),
		slog.String("err", errString(err)),
	)
}

// LogRequest 记录 HTTP 请求（用于访问监控）
func LogRequest(requestID, method, path string, status int, duration time.Duration) {
	if rootJobLogger == nil {
		return
	}
	rootJobLogger.Info("http_request",
		slog.String("request_id", requestID),
		slog.String("method", method),
		slog.String("path", path),
		slog.Int("status", status),
		slog.Int64("duration_ms", duration.Milliseconds()),
	)
}

// LogAPICall 记录 LLM / 图片 API 调用（用于监控 API 耗时和调用频次）
func LogAPICall(model, kind string, duration time.Duration, ok bool, err error) {
	if rootJobLogger == nil {
		return
	}
	attrs := []any{
		slog.String("model", model),
		slog.String("kind", kind),
		slog.Int64("duration_ms", duration.Milliseconds()),
		slog.Bool("ok", ok),
	}
	if err != nil {
		attrs = append(attrs, slog.String("err", errString(err)))
	}
	rootJobLogger.Info("api_call", attrs...)
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
