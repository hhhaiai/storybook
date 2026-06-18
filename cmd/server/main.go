package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode"

	"github.com/go-pdf/fpdf"
	"mystorybook/internal/api"
	"mystorybook/internal/config"
	gen "mystorybook/internal/generator"
	"mystorybook/internal/store"
)

// jobCancelRegistry 维护 jobID → cancelFunc 的映射，供 /api/cancel 使用
var (
	jobCancelMu       sync.Mutex
	jobCancelRegistry = map[string]context.CancelFunc{}
)

func registerJobCancel(jobID string, cancel context.CancelFunc) {
	jobCancelMu.Lock()
	defer jobCancelMu.Unlock()
	jobCancelRegistry[jobID] = cancel
}

func unregisterJobCancel(jobID string) {
	jobCancelMu.Lock()
	defer jobCancelMu.Unlock()
	delete(jobCancelRegistry, jobID)
}

func cancelJob(jobID string) bool {
	jobCancelMu.Lock()
	cancel, ok := jobCancelRegistry[jobID]
	jobCancelMu.Unlock()
	if ok {
		cancel()
	}
	return ok
}

var jobStore *store.Store

const maxRequestBodySize = 1 << 20 // 1MB

func decodeBody(w http.ResponseWriter, r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)
	return json.NewDecoder(r.Body).Decode(v)
}

func main() {
	// 启动校验：确保 API 密钥已配置
	cfg := config.Get()
	if cfg.APIKey == "" {
		fmt.Fprintln(os.Stderr, "❌ 错误: 未配置 API 密钥")
		fmt.Fprintln(os.Stderr, "   请在 .env 文件中设置 USER_API_KEY")
		fmt.Fprintln(os.Stderr, "   示例: USER_API_KEY=your-api-key-here")
		os.Exit(1)
	}
	if cfg.APIBaseURL == "" {
		fmt.Fprintln(os.Stderr, "❌ 错误: 未配置 API 地址")
		fmt.Fprintln(os.Stderr, "   请在 .env 文件中设置 USER_BASE_URL")
		fmt.Fprintln(os.Stderr, "   示例: USER_BASE_URL=https://your-api-endpoint.com/v1")
		os.Exit(1)
	}

	// 初始化日志
	initLogger("logs")
	defer closeLogger()

	// 加载持久化配置（自定义模型和渠道）
	if err := config.LoadPersistentConfig(); err != nil {
		log.Printf("⚠️  加载配置文件失败: %v", err)
	} else {
		log.Println("✅ 已加载持久化配置")
	}

	// 初始化 SQLite 存储
	var err error
	jobStore, err = store.Open("storybook.db")
	if err != nil {
		fmt.Fprintf(os.Stderr, "数据库初始化失败: %v\n", err)
		os.Exit(1)
	}
	defer jobStore.Close()

	// 启动时清理中断的任务
	if cleaned, _ := jobStore.CleanupStaleJobs(); cleaned > 0 {
		log.Printf("启动清理: %d 个中断任务已标记失败", cleaned)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", servePage("web/admin.html"))
	mux.HandleFunc("/api/config", handleConfig)
	mux.HandleFunc("/api/presets", handlePresets)
	mux.HandleFunc("/api/generate", handleGenerate)
	mux.HandleFunc("/api/status", handleStatus)
	mux.HandleFunc("/api/stories", handleStories)
	mux.HandleFunc("/api/pdf", handlePDF)
	mux.HandleFunc("/api/delete", handleDeleteStories)
	mux.HandleFunc("/api/models", handleListModels) // 从远程 API 获取模型列表
	mux.HandleFunc("/api/models/custom", handleCustomModels)
	mux.HandleFunc("/api/styles", handleStyles)
	mux.HandleFunc("/api/story-styles", handleStoryStyles)
	mux.HandleFunc("/api/model-presets", handleModelPresets)
	mux.HandleFunc("/api/work-types", handleWorkTypes) // 管理自定义模型
	mux.HandleFunc("/api/cancel", handleCancel)
	mux.HandleFunc("/api/resume", handleResume)
	mux.HandleFunc("/api/jobs", handleJobs)                        // 获取所有持久化任务
	mux.HandleFunc("/api/providers", handleProviders)              // 渠道管理
	mux.HandleFunc("/api/providers/detect", handleDetectProtocol)  // 探测协议
	mux.HandleFunc("/api/providers/switch", handleSwitchProvider)  // 切换渠道
	mux.HandleFunc("/api/providers/pull-models", handlePullModels) // 拉取模型列表
	mux.Handle("/outputs/", http.StripPrefix("/outputs/", http.FileServer(http.Dir("outputs"))))
	addr := fmt.Sprintf(":%d", config.Get().Port)
	srv := &http.Server{Addr: addr, Handler: withRequestLog(mux)}

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("收到停机信号，正在关闭...")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		srv.Shutdown(ctx)
	}()

	log.Printf("服务启动 http://localhost%s", addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func servePage(path string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, path)
	}
}

func handleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		writeJSON(w, 200, config.Get())
		return
	}
	if r.Method == http.MethodPost {
		var patch config.ConfigPatch
		if err := decodeBody(w, r, &patch); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		writeJSON(w, 200, config.ApplyPatch(patch))
		return
	}
	http.Error(w, "method", 405)
}

// handleListModels 从远程 API 获取模型列表
func handleListModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method", 405)
		return
	}
	cfg := config.Get()
	baseURL, apiKey, protocol, requestModel := config.GetModelConfig(cfg.TextModel)
	client := api.NewRuntimeClient(baseURL, apiKey, requestModel, cfg.ImageModel, 30*time.Second, protocol)
	models, err := client.ListModels(r.Context())
	if err != nil {
		writeJSON(w, 200, map[string]any{
			"models": []string{},
			"error":  err.Error(),
		})
		return
	}
	writeJSON(w, 200, map[string]any{
		"models": models,
	})
}

// handleCancel 取消正在运行的任务
func handleCancel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method", 405)
		return
	}
	jobID := r.URL.Query().Get("job")
	if jobID == "" {
		http.Error(w, "job required", 400)
		return
	}
	if cancelJob(jobID) {
		writeJSON(w, 200, map[string]any{"ok": true, "job": jobID, "message": "已请求取消"})
		return
	}
	// 不在注册表里：可能已结束
	job, _ := jobStore.GetJob(jobID)
	if job == nil {
		http.Error(w, "job not found", 404)
		return
	}
	writeJSON(w, 200, map[string]any{"ok": false, "job": jobID, "status": job.Status, "message": "任务不在运行中"})
}

// handleResume 续做被取消或失败的任务
// 规则：job 必须是 paused/failed 状态且 StoryFolder 目录里有部分图片
func handleResume(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method", 405)
		return
	}
	jobID := r.URL.Query().Get("job")
	if jobID == "" {
		http.Error(w, "job required", 400)
		return
	}
	job, err := jobStore.GetJob(jobID)
	if err != nil || job == nil {
		http.Error(w, "job not found", 404)
		return
	}
	if job.Status == "running" {
		http.Error(w, "job is already running", 409)
		return
	}
	if job.Status == "done" {
		http.Error(w, "job already completed", 409)
		return
	}
	if job.StoryFolder == "" {
		http.Error(w, "no output dir to resume", 400)
		return
	}
	// 检查目录里还有没有图片
	if _, err := os.Stat(filepath.Join(job.StoryFolder, "images")); err != nil {
		http.Error(w, "output dir missing: "+job.StoryFolder, 400)
		return
	}

	// 准备续做：重置状态为 running
	jobStore.UpdateJob(jobID, func(j *store.Job) {
		j.Status = "running"
		j.Phase = "续做中"
		j.Error = ""
		j.Logs = append(j.Logs, "🔄 断点续做")
	})
	LogJob(jobID, "续做", fmt.Sprintf("目录=%s", job.StoryFolder))

	// 启动新的 runJob，传入 StoryFolder 让 generator 续做
	cfg := config.Get()
	runCtx, cancel := context.WithCancel(r.Context())
	registerJobCancel(jobID, cancel)
	go func(jID string, runCtx context.Context, runCancel context.CancelFunc, resumeDir string) {
		defer unregisterJobCancel(jID)
		defer runCancel()
		runJobResume(runCtx, cfg, jID, resumeDir)
	}(jobID, runCtx, cancel, job.StoryFolder)

	writeJSON(w, 200, map[string]any{"ok": true, "job": jobID, "message": "已启动续做", "dir": job.StoryFolder})
}

// handleJobs 返回所有持久化的任务（running/paused/failed）
func handleJobs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method", 405)
		return
	}
	jobs, err := jobStore.ListJobs()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	// 只返回未完成的任务（done 的不需要在前端显示，因为已经在 /api/stories 里了）
	var active []*store.Job
	for _, j := range jobs {
		if j.Status != "done" {
			active = append(active, j)
		}
	}
	writeJSON(w, 200, active)
}

// handleProviders 管理渠道（GET: 列表，POST: 添加，DELETE: 删除）
func handleProviders(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		providers := config.GetProviders()
		writeJSON(w, 200, providers)
	case http.MethodPost:
		var p config.Provider
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			http.Error(w, "invalid json", 400)
			return
		}
		if p.ID == "" || p.BaseURL == "" || p.APIKey == "" {
			http.Error(w, "id, base_url and api_key required", 400)
			return
		}
		if err := config.AddProvider(p); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		writeJSON(w, 200, map[string]interface{}{"ok": true, "provider": p})
	case http.MethodDelete:
		id := r.URL.Query().Get("id")
		if id == "" {
			http.Error(w, "id required", 400)
			return
		}
		if err := config.RemoveProvider(id); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		writeJSON(w, 200, map[string]bool{"ok": true})
	default:
		http.Error(w, "method not allowed", 405)
	}
}

// handleDetectProtocol 探测 API 协议并获取模型列表
func handleDetectProtocol(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	var req struct {
		BaseURL  string `json:"base_url"`
		APIKey   string `json:"api_key"`
		Protocol string `json:"protocol"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", 400)
		return
	}
	if req.BaseURL == "" || req.APIKey == "" {
		http.Error(w, "base_url and api_key required", 400)
		return
	}

	var protocol api.Protocol
	var err error
	if req.Protocol == "" || req.Protocol == "auto" {
		protocol, err = api.DetectProtocol(req.BaseURL, req.APIKey)
		if err != nil {
			writeJSON(w, 200, map[string]interface{}{
				"protocol": "openai", // 默认
				"models":   []string{},
				"error":    err.Error(),
			})
			return
		}
	} else {
		protocol = api.NormalizeProtocol(req.Protocol)
	}

	adapter := api.NewAdapter(protocol, req.BaseURL, req.APIKey, 30*time.Second)
	models, err := adapter.ListModels(context.Background())
	if err != nil {
		writeJSON(w, 200, map[string]interface{}{
			"protocol": string(protocol),
			"models":   []string{},
			"error":    err.Error(),
		})
		return
	}

	writeJSON(w, 200, map[string]interface{}{
		"protocol": string(protocol),
		"models":   models,
	})
}

// handleSwitchProvider 切换当前渠道
func handleSwitchProvider(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	var req struct {
		ProviderID string `json:"provider_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", 400)
		return
	}

	if err := config.SetCurrentProvider(req.ProviderID); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	writeJSON(w, 200, map[string]interface{}{
		"ok":       true,
		"provider": req.ProviderID,
	})
}

// handlePullModels 从指定渠道拉取模型列表并添加到自定义模型
func handlePullModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	var req struct {
		ProviderID string `json:"provider_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", 400)
		return
	}

	providers := config.GetProviders()
	var provider *config.Provider
	for _, p := range providers {
		if p.ID == req.ProviderID {
			provider = &p
			break
		}
	}
	if provider == nil {
		http.Error(w, "provider not found", 404)
		return
	}

	// 使用该渠道的配置拉取模型列表
	protocol := provider.Protocol
	if protocol == "" {
		protocol = "openai"
	}
	adapter := api.NewAdapter(api.Protocol(protocol), provider.BaseURL, provider.APIKey, 30*time.Second)
	models, err := adapter.ListModels(context.Background())
	if err != nil {
		writeJSON(w, 200, map[string]interface{}{
			"ok":     false,
			"models": []string{},
			"error":  err.Error(),
		})
		return
	}

	// 将模型添加到该渠道的模型列表
	for _, m := range models {
		config.AddModelToProvider(req.ProviderID, m.ID)
	}

	// 保存配置
	if err := config.SavePersistentConfig(); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	var modelIDs []string
	for _, m := range models {
		modelIDs = append(modelIDs, m.ID)
	}

	writeJSON(w, 200, map[string]interface{}{
		"ok":       true,
		"provider": req.ProviderID,
		"models":   modelIDs,
		"count":    len(modelIDs),
	})
}

// runJobResume 续做版本的 runJob，使用同一个 StoryFolder 目录，跳过已生成图片
func runJobResume(ctx context.Context, cfg config.RuntimeConfig, jobID, resumeDir string) {
	now := time.Now().Format(time.RFC3339)
	LogJob(jobID, "续做开始", fmt.Sprintf("目录=%s", resumeDir))
	jobStore.UpdateJob(jobID, func(j *store.Job) {
		j.StartedAt = now
		j.Logs = append(j.Logs, "续做开始")
	})

	job, _ := jobStore.GetJob(jobID)
	if job == nil {
		return
	}
	theme := job.Theme

	g := gen.New(cfg, func(p gen.Progress) {
		LogJob(jobID, p.Phase, fmt.Sprintf("%d/%d", p.Done, p.Total))
		jobStore.EnqueueProgress(jobID, p.Total, p.Done, p.Phase, p.Title)
		jobStore.AppendLogIfPhaseChanged(jobID, p.Phase, p.Done, p.Total)
	})
	defer func() {
		jobStore.UpdateJob(jobID, func(j *store.Job) {
			if j.Status == "running" {
				j.Status = "failed"
				j.Phase = "异常终止"
			}
		})
	}()

	htmlPath, err := g.GenerateResume(ctx, theme, resumeDir)
	defer jobStore.ResetJobTrackers(jobID)
	jobStore.UpdateJob(jobID, func(j *store.Job) {
		if err != nil {
			if errors.Is(err, context.Canceled) {
				LogJobError(jobID, "续做被取消", err)
				j.Status = "paused"
				j.Phase = "已取消（可再次续做）"
				j.Error = err.Error()
				j.Logs = append(j.Logs, "⏸ 续做再次被取消")
			} else {
				LogJobError(jobID, "续做失败", err)
				j.Status = "failed"
				j.Error = err.Error()
				j.Phase = "续做失败"
				j.Logs = append(j.Logs, "❌ 续做失败: "+err.Error())
			}
		} else {
			LogJob(jobID, "续做完成", fmt.Sprintf("路径=%s", htmlPath))
			j.Status = "done"
			j.Phase = "完成"
			j.Done = j.Total
			j.StoryPath = htmlPath
			if idx := strings.LastIndex(htmlPath, "/"); idx >= 0 {
				j.StoryFolder = htmlPath[:idx]
			}
			j.Logs = append(j.Logs, "✅ 续做完成")
			coverPath := filepath.Join(filepath.Dir(htmlPath), "images", "1.jpg")
			if _, statErr := os.Stat(coverPath); statErr == nil {
				rel, _ := filepath.Rel(".", coverPath)
				j.CoverURL = "/" + rel
			}
		}
	})
}

// handleCustomModels 管理自定义模型（GET 列表 / POST 添加 / DELETE 删除）
func handleCustomModels(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		cfg := config.Get()
		writeJSON(w, 200, map[string]any{"models": cfg.CustomModels})
	case http.MethodPost:
		var req struct {
			ID       string `json:"id"`
			Model    string `json:"model"`
			Name     string `json:"name"`
			Type     string `json:"type"`
			BaseURL  string `json:"base_url"`
			APIKey   string `json:"api_key"`
			Protocol string `json:"protocol"`
			Provider string `json:"provider"`
		}
		if err := decodeBody(w, r, &req); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		if req.ID == "" || req.Type == "" {
			http.Error(w, "id and type required", 400)
			return
		}
		models, err := config.AddCustomModel(config.ModelEntry{
			ID:       req.ID,
			Model:    req.Model,
			Name:     req.Name,
			Type:     req.Type,
			BaseURL:  req.BaseURL,
			APIKey:   req.APIKey,
			Protocol: req.Protocol,
			Provider: req.Provider,
		})
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		writeJSON(w, 200, map[string]any{"models": models})
	case http.MethodDelete:
		var req struct {
			ID string `json:"id"`
		}
		if err := decodeBody(w, r, &req); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		models, err := config.RemoveCustomModel(req.ID)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		writeJSON(w, 200, map[string]any{"models": models})
	default:
		http.Error(w, "method", 405)
	}
}

func handleGenerate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method", 405)
		return
	}
	var req struct {
		TextModel      string `json:"text_model"`
		ImageModel     string `json:"image_model"`
		StoryType      string `json:"story_type"`
		Theme          string `json:"theme"`
		Style          string `json:"style"`
		PromptTemplate string `json:"prompt_template"`
		Count          int    `json:"count"`
		PageCount      int    `json:"page_count"`
	}
	if err := decodeBody(w, r, &req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	cfg := config.Get()
	if req.TextModel != "" {
		cfg.TextModel = req.TextModel
	}
	if req.ImageModel != "" {
		cfg.ImageModel = req.ImageModel
	}
	if req.StoryType != "" {
		cfg.StoryType = req.StoryType
	}
	if req.Theme != "" {
		cfg.Theme = req.Theme
	}
	if req.Style != "" {
		cfg.Style = req.Style
	}
	if req.PromptTemplate != "" {
		if tmpl, ok := config.PromptPresets[req.PromptTemplate]; ok {
			cfg.PromptTemplate = tmpl
		} else {
			cfg.PromptTemplate = req.PromptTemplate
		}
	}
	if req.Count > 0 {
		cfg.BatchCount = req.Count
	}
	if req.PageCount >= 12 && req.PageCount <= 30 {
		cfg.PageCount = req.PageCount
	}

	ids := make([]string, 0, cfg.BatchCount)
	for i := 0; i < cfg.BatchCount; i++ {
		id := fmt.Sprintf("%s-%d", time.Now().Format("20060102150405"), i)
		theme := cfg.Theme
		if cfg.BatchCount > 1 {
			theme = fmt.Sprintf("%s-%d", cfg.Theme, i+1)
		}
		job := &store.Job{
			ID:        id,
			Theme:     theme,
			StoryType: cfg.StoryType,
			Style:     cfg.Style,
			PageCount: cfg.PageCount,
			Status:    "running",
			Phase:     "准备中",
			UpdatedAt: time.Now().Format(time.RFC3339),
			Logs:      []string{"任务已创建"},
		}
		if err := jobStore.CreateJob(job); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		LogJob(id, "任务创建", fmt.Sprintf("主题=%s 类型=%s 页数=%d", theme, cfg.StoryType, cfg.PageCount))
		// 父 ctx 必须是 Background：r.Context() 在 handler 返回时立刻被取消，
		// 用它当父 ctx 会导致 job 启动 <1ms 就被误杀。
		// 手动取消走 jobCancelRegistry / /api/cancel。
		runCtx, cancel := context.WithCancel(context.Background())
		registerJobCancel(id, cancel)
		go func(jobID string, runCtx context.Context, runCancel context.CancelFunc) {
			defer unregisterJobCancel(jobID)
			defer runCancel()
			runJob(runCtx, cfg, jobID)
		}(id, runCtx, cancel)
		ids = append(ids, id)
	}
	writeJSON(w, 200, map[string]any{"jobs": ids})
}

func runJob(ctx context.Context, cfg config.RuntimeConfig, jobID string) {
	now := time.Now().Format(time.RFC3339)
	LogJob(jobID, "开始生成", "")
	jobStore.UpdateJob(jobID, func(j *store.Job) {
		j.StartedAt = now
		j.Logs = append(j.Logs, "开始生成")
	})

	// 读取主题用于生成
	job, _ := jobStore.GetJob(jobID)
	if job == nil {
		return
	}
	theme := job.Theme

	// 预测输出目录（确保续做时能找到同一个 dir）
	outputDir := cfg.OutputPath
	if outputDir == "" {
		outputDir = "outputs"
	}
	predictedDir := filepath.Join(outputDir, fmt.Sprintf("story-%s", time.Now().Format("20060102-150405")))
	jobStore.UpdateJob(jobID, func(j *store.Job) {
		j.StoryFolder = predictedDir
	})

	g := gen.New(cfg, func(p gen.Progress) {
		LogJob(jobID, p.Phase, fmt.Sprintf("%d/%d", p.Done, p.Total))
		// 进度走内存缓冲（500ms 批量落库），避免每 emit 一次就全表 read-modify-write
		jobStore.EnqueueProgress(jobID, p.Total, p.Done, p.Phase, p.Title)
		// phase 切换才追加日志（高频的 done 递增不写日志，减少 DB 压力）
		jobStore.AppendLogIfPhaseChanged(jobID, p.Phase, p.Done, p.Total)
	})
	defer func() {
		jobStore.UpdateJob(jobID, func(j *store.Job) {
			if j.Status == "running" {
				j.Status = "failed"
				j.Phase = "异常终止"
			}
		})
	}()

	htmlPath, err := g.GenerateResume(ctx, theme, predictedDir)
	defer jobStore.ResetJobTrackers(jobID) // 结束任何状态都释放内存 tracker
	jobStore.UpdateJob(jobID, func(j *store.Job) {
		if err != nil {
			// ctx 取消导致的"失败"区分对待：标 paused 而不是 failed，支持断点续做
			if errors.Is(err, context.Canceled) {
				LogJobError(jobID, "用户取消", err)
				j.Status = "paused"
				j.Phase = "已取消（可续做）"
				j.Error = err.Error()
				j.Logs = append(j.Logs, "⏸ 任务已取消，支持断点续做")
			} else {
				LogJobError(jobID, "生成失败", err)
				j.Status = "failed"
				j.Error = err.Error()
				j.Phase = "生成失败"
				j.Logs = append(j.Logs, "❌ "+err.Error())
			}
		} else {
			LogJob(jobID, "生成完成", fmt.Sprintf("路径=%s", htmlPath))
			j.Status = "done"
			j.Phase = "完成"
			j.Done = j.Total
			j.StoryPath = htmlPath
			if idx := strings.LastIndex(htmlPath, "/"); idx >= 0 {
				j.StoryFolder = htmlPath[:idx]
			}
			j.Logs = append(j.Logs, "✅ 绘本生成完成")
			coverPath := filepath.Join(filepath.Dir(htmlPath), "images", "1.jpg")
			if _, statErr := os.Stat(coverPath); statErr == nil {
				rel, _ := filepath.Rel(".", coverPath)
				j.CoverURL = "/" + rel
			}
		}
	})
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
	jobID := r.URL.Query().Get("job")
	if jobID == "" {
		http.Error(w, "job required", 400)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "no flush", 500)
		return
	}
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			job, err := jobStore.GetJob(jobID)
			if err != nil || job == nil {
				return
			}
			data, _ := json.Marshal(job)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
			if job.Status != "running" {
				return
			}
		}
	}
}

func handlePresets(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method", 405)
		return
	}
	type PresetInfo struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Desc   string `json:"desc"`
		Theme  string `json:"theme"`
		Prompt string `json:"prompt"`
	}
	names := map[string][]string{
		"police":      {"🚔 警察救援", "男警官帮助迷路的小朋友找到家人", "迷路的小朋友在商场里害怕地哭泣"},
		"rescue":      {"🚁 武警救援", "男武警在危险环境中救出被困的人", "山路塌方后游客被困在悬崖边"},
		"catchThief":  {"🔍 抓坏人", "男警官抓住偷东西的坏人", "超市里有人偷了小朋友的玩具"},
		"armedRescue": {"💪 武警救援", "男武警在危险环境中救出被困的人", "暴雨中山洪爆发有人被困"},
		"armedCatch":  {"🎯 武警抓坏人", "男武警抓捕危险坏人", "银行里出现了蒙面坏人"},
		"swat":        {"⚡ 特警行动", "男特警迅速制服危险坏人", "坏人躲在废弃工厂里"},
		"detective":   {"🔎 刑警破案", "男刑警通过线索找出真相", "博物馆的珍贵宝石不见了"},
		"adventure":   {"🗺️ 冒险探索", "小探险家的奇妙旅程", "神秘森林里的宝藏地图"},
		"friendship":  {"💕 友谊温情", "友谊的力量", "新来的小朋友没有人玩"},
	}
	var presets []PresetInfo
	for id, tmpl := range config.PromptPresets {
		info := PresetInfo{ID: id, Prompt: tmpl}
		if n, ok := names[id]; ok {
			info.Name = n[0]
			info.Desc = n[1]
			info.Theme = n[2]
		}
		presets = append(presets, info)
	}
	writeJSON(w, 200, presets)
}

func handleDeleteStories(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method", 405)
		return
	}
	var req struct {
		Names []string `json:"names"`
	}
	if err := decodeBody(w, r, &req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	cfg := config.Get()
	outputDir := cfg.OutputPath
	if outputDir == "" {
		outputDir = "outputs"
	}
	deleted := 0
	for _, name := range req.Names {
		safe := filepath.Base(name)
		dir := filepath.Join(outputDir, safe)
		if err := os.RemoveAll(dir); err == nil {
			deleted++
		}
		// 同步清理内存 tracker（如果该 job 还在运行）
		jobStore.ResetJobTrackers(safe)
	}
	writeJSON(w, 200, map[string]any{"deleted": deleted})
}

func handleStories(w http.ResponseWriter, r *http.Request) {
	type Item struct {
		Name      string `json:"name"`
		Title     string `json:"title"`
		Theme     string `json:"theme"`
		StoryType string `json:"story_type"`
		PageCount int    `json:"page_count"`
		Path      string `json:"path"`
		HasCover  bool   `json:"has_cover"`
		Cover     string `json:"cover"`
	}
	cfg := config.Get()
	outputDir := cfg.OutputPath
	if outputDir == "" {
		outputDir = "outputs"
	}
	items := make([]Item, 0)
	entries, _ := os.ReadDir(outputDir)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(outputDir, e.Name(), "data.json"))
		if err != nil {
			continue
		}
		var book struct {
			Title     string     `json:"title"`
			Theme     string     `json:"theme"`
			StoryType string     `json:"story_type"`
			Pages     []struct{} `json:"pages"`
		}
		if err := json.Unmarshal(raw, &book); err != nil {
			continue
		}
		coverFile := filepath.Join(outputDir, e.Name(), "images", "1.jpg")
		coverPath := fmt.Sprintf("/outputs/%s/images/1.jpg", e.Name())
		hasCover := false
		if _, statErr := os.Stat(coverFile); statErr == nil {
			hasCover = true
		}
		items = append(items, Item{
			Name:      e.Name(),
			Title:     book.Title,
			Theme:     book.Theme,
			StoryType: book.StoryType,
			PageCount: len(book.Pages),
			Path:      fmt.Sprintf("/outputs/%s/index.html", e.Name()),
			HasCover:  hasCover,
			Cover:     coverPath,
		})
	}
	writeJSON(w, 200, items)
}

func handlePDF(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if name == "" {
		http.Error(w, "name required", 400)
		return
	}
	cfg := config.Get()
	outputDir := cfg.OutputPath
	if outputDir == "" {
		outputDir = "outputs"
	}
	dir := filepath.Join(outputDir, filepath.Base(name))
	raw, err := os.ReadFile(filepath.Join(dir, "data.json"))
	if err != nil {
		http.Error(w, "story not found", 404)
		return
	}
	var book struct {
		Title string `json:"title"`
		Pages []struct {
			PageNum int    `json:"page_num"`
			Text    string `json:"text"`
			Image   string `json:"image"`
		} `json:"pages"`
	}
	if err := json.Unmarshal(raw, &book); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	pdf := fpdf.New("L", "mm", "A4", "")
	pdf.SetAutoPageBreak(false, 0)
	// A4 横向: 297mm x 210mm
	imgW := 160.0  // 左侧图片宽度
	imgH := 210.0  // 全高
	textX := 165.0 // 右侧文字起始X
	textW := 125.0 // 右侧文字宽度

	pageLimit := len(book.Pages)
	if pageLimit == 0 {
		pageLimit = 20
	}
	for idx, p := range book.Pages {
		if idx >= pageLimit {
			break
		}
		pdf.AddPage()

		// 左侧图片
		img := p.Image
		if img == "" {
			img = fmt.Sprintf("images/%d.jpg", p.PageNum)
		}
		imgPath := filepath.Join(dir, img)
		if _, err := os.Stat(imgPath); err == nil {
			ext := strings.ToLower(filepath.Ext(imgPath))
			imgType := "JPEG"
			switch ext {
			case ".png":
				imgType = "PNG"
			case ".jpg", ".jpeg":
				imgType = "JPEG"
			}
			opt := fpdf.ImageOptions{ImageType: imgType, ReadDpi: false}
			pdf.ImageOptions(imgPath, 0, 0, imgW, imgH, false, opt, 0, "")
		}

		// 右侧文字
		pdf.SetFont("chinese", "", 14)
		pdf.SetTextColor(30, 30, 30)
		// 页码
		pdf.SetXY(textX, 8)
		pdf.SetFont("chinese", "", 10)
		pdf.SetTextColor(150, 150, 150)
		pdf.CellFormat(textW, 6, fmt.Sprintf("%d / %d", p.PageNum, pageLimit), "", 0, "R", false, 0, "")
		// 故事文字
		pdf.SetFont("chinese", "", 14)
		pdf.SetTextColor(30, 30, 30)
		pdf.SetXY(textX, 40)
		pdf.MultiCell(textW, 10, p.Text, "", "L", false)
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s.pdf", safeName(book.Title)))
	if err := pdf.Output(w); err != nil {
		http.Error(w, err.Error(), 500)
	}
}

func handleStyles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method", 405)
		return
	}
	cfg := config.Get()
	writeJSON(w, 200, map[string]any{
		"presets":  config.ImageStylePresets,
		"selected": cfg.ImageStyles,
	})
}

func handleWorkTypes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method", 405)
		return
	}
	cfg := config.Get()
	writeJSON(w, 200, map[string]any{
		"presets":  config.WorkTypePresets,
		"selected": cfg.WorkType,
	})
}

func handleStoryStyles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method", 405)
		return
	}
	cfg := config.Get()
	writeJSON(w, 200, map[string]any{
		"presets":  config.StoryStylePresets,
		"selected": cfg.StoryStyles,
	})
}

func handleModelPresets(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method", 405)
		return
	}
	writeJSON(w, 200, config.ModelPresets)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

// withRequestLog 给每个 HTTP 请求生成 request_id 并记录访问日志
func withRequestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = fmt.Sprintf("req-%d", time.Now().UnixNano())
		}
		w.Header().Set("X-Request-ID", requestID)
		rw := &statusRecorder{ResponseWriter: w, status: 200}
		next.ServeHTTP(rw, r)
		LogRequest(requestID, r.Method, r.URL.Path, rw.status, time.Since(start))
	})
}

// statusRecorder 捕获 HTTP 状态码用于日志
type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (r *statusRecorder) WriteHeader(code int) {
	if !r.wroteHeader {
		r.status = code
		r.wroteHeader = true
	}
	r.ResponseWriter.WriteHeader(code)
}

// Flush 实现 http.Flusher 接口，支持 SSE
func (r *statusRecorder) Flush() {
	if flusher, ok := r.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// safeName 把任意字符串转成跨平台安全的文件名片段
// - 保留所有 Unicode 字母和数字（含中文）
// - 替换文件系统非法字符为 -
// - 折叠连续 -，去除首尾 -
// - 长度限制 80 rune
func safeName(s string) string {
	invalid := func(r rune) bool {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return false
		}
		switch r {
		case '-', '_', '.':
			return false
		}
		return true
	}
	var b strings.Builder
	for _, r := range s {
		if invalid(r) {
			b.WriteRune('-')
		} else {
			b.WriteRune(r)
		}
	}
	v := b.String()
	for strings.Contains(v, "--") {
		v = strings.ReplaceAll(v, "--", "-")
	}
	v = strings.Trim(v, "-.")
	if v == "" {
		return "storybook"
	}
	runes := []rune(v)
	if len(runes) > 80 {
		runes = runes[:80]
	}
	return string(runes)
}
