package generator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"mystorybook/internal/api"
	"mystorybook/internal/config"
	"mystorybook/internal/style"
	"sync"
)

// ImageError 单页图片生成失败（支持 errors.As 提取）
type ImageError struct {
	Page  int    // 1-based 页码
	Stage string // "primary" | "fallback" | "write"
	Err   error
}

func (e *ImageError) Error() string {
	return fmt.Sprintf("page %d image %s: %v", e.Page, e.Stage, e.Err)
}

func (e *ImageError) Unwrap() error { return e.Err }

type Page struct {
	PageNum      int    `json:"page_num"`
	Text         string `json:"text"`
	Image        string `json:"image"`
	Subtitle     string `json:"subtitle,omitempty"`
	Illustration string `json:"illustration"`
}

// === 角色兜底（失败时使用）===========================================
//
// 这里是 LLM 规划/翻译失败时的默认角色描述。修改这里就能改变所有
// 故事的主角设定（默认是男警官，符合小朋友喜好）。
//
// 留空字符串 "" 表示失败时让 LLM 重试，不做兜底。

var (
	// fallbackCharactersCN 角色规划失败时的中文兜底
	fallbackCharactersCN = "男警官：短发英俊，穿深蓝色警服，戴警帽，身材挺拔"

	// fallbackCharactersEN 角色翻译失败时的英文兜底（封面 / 插画用）
	fallbackCharactersEN = "A brave male police officer with short hair, wearing a dark blue police uniform and cap, tall and handsome. Supporting characters include children and townspeople."

	// fallbackCharactersENShort 短英文兜底（角色描述为空时）
	fallbackCharactersENShort = "A brave male police officer with short hair, wearing a dark blue police uniform and cap, tall and handsome."

	// fallbackIllustrationPrefix 插画描述全部失败时给视觉风格的提示
	fallbackIllustrationPrefix = "A brave male police officer in dark blue uniform"
)

type PlanData struct {
	Title      string `json:"title"`
	Theme      string `json:"theme"`
	Summary    string `json:"summary"`
	Characters string `json:"characters"`
	// 用于图片生成的英文角色描述
	EnglishCharacters string `json:"english_characters,omitempty"`
	// 用于图片生成的英文故事摘要
	EnglishSummary string `json:"english_summary,omitempty"`
}

type BookData struct {
	Title      string `json:"title"`
	Author     string `json:"author"`
	StoryType  string `json:"story_type"`
	Theme      string `json:"theme"`
	Style      string `json:"style"`
	ImageModel string `json:"image_model"`
	ImageStyle string `json:"image_style"`
	PageCount  int    `json:"page_count"`
	CreatedAt  string `json:"created_at"`
	Pages      []Page `json:"pages"`
}

type Progress struct {
	Phase string
	Total int
	Done  int
	Title string
}

type Generator struct {
	cfg        config.RuntimeConfig
	onProgress func(Progress)
}

func New(cfg config.RuntimeConfig, onProgress func(Progress)) *Generator {
	return &Generator{
		cfg:        cfg,
		onProgress: onProgress,
	}
}

// getTextClient 动态创建文本模型客户端
func (g *Generator) getTextClient() *api.RuntimeClient {
	baseURL, apiKey, protocol, requestModel := config.GetModelConfig(g.cfg.TextModel)
	return api.NewRuntimeClient(baseURL, apiKey, requestModel, "", 120*time.Second, protocol)
}

// getImageClient 动态创建图片模型客户端
func (g *Generator) getImageClient() *api.RuntimeClient {
	baseURL, apiKey, protocol, requestModel := config.GetModelConfig(g.cfg.ImageModel)
	return api.NewRuntimeClient(baseURL, apiKey, "", requestModel, 120*time.Second, protocol)
}

var randomThemes = []string{
	"迷路的小猫咪找到了回家的路",
	"小恐龙的第一天上学",
	"月亮上的生日派对",
	"会说话的玩具熊的秘密",
	"海底世界的寻宝冒险",
	"云朵上的棉花糖工厂",
	"小兔子的魔法画笔",
	"森林里的音乐节",
	"丢失的彩虹颜色",
	"小火车的奇妙旅程",
	"星星掉进了池塘里",
	"会飞的小象",
	"厨房里的小精灵",
	"大树洞里的秘密王国",
	"小刺猬交朋友",
}

func (g *Generator) Generate(ctx context.Context, theme string) (string, error) {
	return g.GenerateResume(ctx, theme, "")
}

// GenerateResume 同 Generate，但如果 resumeDir 非空且存在，会续做：
// - 跳过已存在的 images/N.jpg
// - 已有 pages 数量不重新生成故事文本
func (g *Generator) GenerateResume(ctx context.Context, theme, resumeDir string) (string, error) {
	if theme == "" {
		theme = randomThemes[time.Now().UnixNano()%int64(len(randomThemes))]
		fmt.Printf("🎲 随机主题: %s\n", theme)
	}
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("generate cancelled: %w", err)
	}

	// 决定输出目录
	outputDir := g.cfg.OutputPath
	if outputDir == "" {
		outputDir = "outputs"
	}
	dir := resumeDir
	if dir == "" {
		dir = filepath.Join(outputDir, fmt.Sprintf("story-%s", time.Now().Format("20060102-150405")))
	}
	// 如果是续做但目录不存在（用户删了），fallback 到新目录
	resumeMode := false
	if resumeDir != "" {
		if _, err := os.Stat(dir); err == nil {
			resumeMode = true
		} else {
			fmt.Printf("⚠️ 续做目录 %s 不存在，重新创建\n", dir)
			dir = filepath.Join(outputDir, fmt.Sprintf("story-%s", time.Now().Format("20060102-150405")))
		}
	}
	if err := os.MkdirAll(filepath.Join(dir, "images"), 0755); err != nil {
		return "", err
	}
	if resumeMode {
		fmt.Printf("🔄 断点续做: %s\n", dir)
	}
	g.emit(Progress{Phase: "规划故事名称与主题", Total: g.cfg.PageCount, Done: 0})
	t0 := time.Now()
	plan, err := g.planStory(ctx, theme)
	fmt.Printf("⏱ 规划故事耗时: %v\n", time.Since(t0).Round(time.Second))
	if err != nil {
		return "", fmt.Errorf("story planning failed: %w", err)
	}
	if plan.Theme == "" {
		plan.Theme = theme
	}
	if plan.Title == "" {
		plan.Title = theme
	}
	plan.Summary = normalizeSummary(plan.Title, plan.Theme, plan.Summary)

	// 翻译角色和摘要为英文（用于图片生成）
	g.emit(Progress{Phase: "准备图片描述", Total: g.cfg.PageCount, Done: 0, Title: plan.Title})
	t0 = time.Now()
	plan = g.translatePlanForImages(ctx, plan)
	fmt.Printf("⏱ 翻译角色耗时: %v\n", time.Since(t0).Round(time.Second))

	g.emit(Progress{Phase: "生成故事文本", Total: g.cfg.PageCount, Done: 0, Title: plan.Title})
	t0 = time.Now()
	raw, err := g.generateStoryText(ctx, plan)
	fmt.Printf("⏱ 故事文本耗时: %v (长度=%d字符)\n", time.Since(t0).Round(time.Second), len(raw))
	if err != nil {
		return "", fmt.Errorf("story text failed: %w", err)
	}
	if strings.TrimSpace(raw) == "" {
		return "", fmt.Errorf("empty story response")
	}
	g.emit(Progress{Phase: "解析页面", Total: g.cfg.PageCount, Done: 0})
	pages := g.parsePages(raw)
	if len(pages) == 0 {
		return "", fmt.Errorf("story parse failed")
	}

	// 为每页生成纯英文场景描述（用于绘图）
	g.emit(Progress{Phase: "生成插画描述", Total: len(pages), Done: 0})
	t0 = time.Now()
	illustrations := g.generateIllustrations(ctx, plan, pages)
	fmt.Printf("⏱ 插画描述耗时: %v (生成%d条)\n", time.Since(t0).Round(time.Second), len(illustrations))
	for i := range pages {
		if i < len(illustrations) {
			pages[i].Illustration = illustrations[i]
		}
	}
	for len(pages) < g.cfg.PageCount {
		no := len(pages) + 1
		pages = append(pages, Page{PageNum: no, Text: fmt.Sprintf("第%d页故事待补充", no)})
	}
	if len(pages) > g.cfg.PageCount {
		pages = pages[:g.cfg.PageCount]
	}

	// 第1页作为封面海报页
	pages[0] = Page{
		PageNum: 1,
		Text:    plan.Title,
	}

	g.emit(Progress{Phase: "生成绘本插画", Total: len(pages), Done: 0, Title: plan.Title})
	book, err := g.buildBook(ctx, dir, plan, pages, resumeMode)
	if err != nil {
		return "", err
	}
	if err := g.saveJSON(dir, book); err != nil {
		return "", err
	}
	htmlPath, err := g.renderHTML(dir, book)
	if err != nil {
		return "", err
	}
	g.emit(Progress{Phase: "完成", Total: len(book.Pages), Done: len(book.Pages)})
	return htmlPath, nil
}

// translatePlanForImages 将中文角色描述和摘要翻译为英文（仅用于图片生成）
func (g *Generator) translatePlanForImages(ctx context.Context, plan PlanData) PlanData {
	prompt := fmt.Sprintf(`Translate the following Chinese text to English. Output ONLY the English translation, nothing else.

Character descriptions:
%s

Story summary:
%s

Output format (JSON only):
{
  "characters": "English character descriptions here",
  "summary": "English summary here"
}`, plan.Characters, plan.Summary)

	client := g.getTextClient()
	raw, err := client.Chat(ctx, prompt, 512)
	if err != nil {
		fmt.Printf("⚠️  翻译失败，使用备用英文描述: %v\n", err)
		plan.EnglishCharacters = fallbackCharactersEN
		plan.EnglishSummary = plan.Title
		return plan
	}

	var result struct {
		Characters string `json:"characters"`
		Summary    string `json:"summary"`
	}
	if err := json.Unmarshal([]byte(extractJSON(raw)), &result); err != nil {
		// 尝试直接使用原文（已经是英文的情况）
		if isEnglish(raw) {
			plan.EnglishCharacters = raw
			plan.EnglishSummary = plan.Summary
		} else {
			plan.EnglishCharacters = fallbackCharactersEN
			plan.EnglishSummary = plan.Title
		}
		return plan
	}

	plan.EnglishCharacters = stripChineseChars(result.Characters)
	plan.EnglishSummary = stripChineseChars(result.Summary)
	if plan.EnglishCharacters == "" {
		plan.EnglishCharacters = fallbackCharactersENShort
	}
	if plan.EnglishSummary == "" {
		plan.EnglishSummary = plan.Title
	}
	fmt.Printf("🌐 角色英文描述: %s\n", plan.EnglishCharacters)
	return plan
}

func (g *Generator) planStory(ctx context.Context, theme string) (PlanData, error) {
	prompt := fmt.Sprintf(`请为以下儿童绘本主题做一次简短策划。
主题：%s

## 故事名要求（最重要！）

名字是孩子决定要不要看这本书的第一秒。必须做到：一看就想去翻！

### 取名公式（随机用一种）：
- 角色昵称 + 意外物件："嘟嘟警车收到了一封彩虹信"
- 角色 + 反转场景："小熊警察在云朵上巡逻"
- 悬念 + 可爱感："谁偷走了月亮饼干？"
- 角色 + 奇妙能力："会飞的消防帽"
- 拟人 + 冒险："迷路的小雨滴找妈妈"

### 好名字示范（每次必须不同，不可重复）：
"嘟嘟警车收到了一封彩虹信"
"小熊警察在云朵上巡逻"
"谁偷走了月亮饼干？"
"会飞的消防帽"
"迷路的小雨滴找妈妈"
"口袋里的迷你消防站"
"星星警察的夜光手电筒"
"小不点和她的巨人保镖"
"下雨天的第100个拥抱"
"藏在书包里的小秘密"

### 坏名字（绝对不要）：
"正义追击"、"勇敢的警察"、"救援行动"、"守护平安"（太成人化）
"小明的一天"、"快乐的一天"（太平淡）

### 技巧：
- 用ABB式昵称（嘟嘟、乖乖、圆圆）更有童趣
- 加一个意想不到的物件（彩虹信、月亮饼干、夜光手电筒）
- 带问号或悬念更好奇
- 7-15个字最佳

## 其他要求
2. 输出主主题
3. 输出 60-100 字的故事梗概
4. 输出角色描述（外貌、服装、体型细节，用于保持画面一致性）。例如："男警官：短发、英俊、穿深蓝色警服、戴警帽、身材挺拔；小朋友：5-8岁、穿彩色衣服"
5. 仅输出 JSON，不要输出任何其他内容

JSON格式：
{
  "title": "故事名",
  "theme": "主主题",
  "summary": "简短梗概",
  "characters": "角色描述"
}`, theme)
	client := g.getTextClient()
	raw, err := client.Chat(ctx, prompt, 1024)
	if err != nil {
		return PlanData{}, err
	}
	var plan PlanData
	if err := json.Unmarshal([]byte(extractJSON(raw)), &plan); err != nil {
		return PlanData{Title: theme, Theme: theme, Characters: fallbackCharactersCN}, nil
	}
	if plan.Characters == "" {
		plan.Characters = fallbackCharactersCN
	}
	return plan, nil
}

func normalizeSummary(title, theme, summary string) string {
	s := strings.TrimSpace(summary)
	if s != "" {
		return s
	}
	if strings.TrimSpace(title) != "" {
		return strings.TrimSpace(title)
	}
	return strings.TrimSpace(theme)
}

func extractJSON(s string) string {
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start == -1 || end == -1 || end <= start {
		return s
	}
	return s[start : end+1]
}

func (g *Generator) generateStoryText(ctx context.Context, plan PlanData) (string, error) {
	var prompt string
	if g.cfg.PromptTemplate != "" {
		// 使用自定义模板
		prompt = g.cfg.PromptTemplate
		prompt = strings.ReplaceAll(prompt, "{{PAGE_COUNT}}", strconv.Itoa(g.cfg.PageCount))
		prompt = strings.ReplaceAll(prompt, "{{STORY_TYPE}}", g.cfg.StoryType)
		prompt = strings.ReplaceAll(prompt, "{{THEME}}", plan.Theme)
		prompt = strings.ReplaceAll(prompt, "{{STYLE}}", g.cfg.Style)
		prompt = strings.ReplaceAll(prompt, "{{IMAGE_STYLE}}", g.cfg.ImageStyle)
	} else {
		// 使用风格系统的标准化 prompt
		tmplID := g.detectTemplateID()
		prompt = style.AssembleStoryPrompt(style.StoryPromptRequest{
			TemplateID: tmplID,
			Theme:      plan.Theme,
			PageCount:  g.cfg.PageCount,
			Style:      g.cfg.Style,
		})
		prompt = strings.ReplaceAll(prompt, "{{PAGE_COUNT}}", strconv.Itoa(g.cfg.PageCount))
		prompt = strings.ReplaceAll(prompt, "{{STORY_TYPE}}", g.cfg.StoryType)
	}
	prompt += fmt.Sprintf("\n\n角色设定（全文必须严格保持一致）：%s", plan.Characters)
	prompt += fmt.Sprintf("\n\n故事梗概：%s", plan.Summary)
	client := g.getTextClient()
	raw, err := client.Chat(ctx, prompt, 4096)
	if err != nil {
		return "", err
	}
	return raw, nil
}

func (g *Generator) parsePages(raw string) []Page {
	// ... (same as before)
	lines := strings.Split(raw, "\n")
	var pages []Page
	var current *Page
	pageNum := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// 匹配 "第X页" 格式
		re := regexp.MustCompile(`^第\s*(\d+)\s*页[：:]?\s*(.*)$`)
		if m := re.FindStringSubmatch(line); m != nil {
			if current != nil {
				pages = append(pages, *current)
			}
			n, _ := strconv.Atoi(m[1])
			pageNum = n
			text := strings.TrimSpace(m[2])
			current = &Page{PageNum: pageNum, Text: text}
			continue
		}
		if current != nil {
			if current.Text != "" {
				current.Text += "\n"
			}
			current.Text += line
		}
	}
	if current != nil {
		pages = append(pages, *current)
	}
	return pages
}

// buildBook 生成所有页面图片并组装绘本数据
// resumeMode=true 时跳过已存在的 images/N.jpg
const maxImageWorkers = 5

func (g *Generator) buildBook(ctx context.Context, dir string, plan PlanData, pages []Page, resumeMode bool) (*BookData, error) {
	book := &BookData{
		Title:      shortTitle(plan.Title, plan.Theme),
		Author:     "AI Storybook",
		StoryType:  g.cfg.StoryType,
		Theme:      plan.Theme,
		Style:      g.cfg.Style,
		ImageModel: g.cfg.ImageModel,
		ImageStyle: g.cfg.ImageStyle,
		PageCount:  len(pages),
		CreatedAt:  time.Now().Format(time.RFC3339),
		Pages:      make([]Page, len(pages)),
	}

	// 续做模式：扫描已有图片
	existingImages := map[int]bool{}
	if resumeMode {
		for i := range pages {
			pageNo := i + 1
			imgPath := filepath.Join(dir, "images", fmt.Sprintf("%d.jpg", pageNo))
			if _, err := os.Stat(imgPath); err == nil {
				existingImages[pageNo] = true
				book.Pages[i] = Page{
					PageNum: pageNo, Text: pages[i].Text, Subtitle: pages[i].Subtitle,
					Image: fmt.Sprintf("images/%d.jpg", pageNo),
				}
			}
		}
		if len(existingImages) > 0 {
			fmt.Printf("📦 续做：发现 %d 张已有图片，跳过生成\n", len(existingImages))
		}
	}

	fmt.Printf("🚀 图片并发生成: %d workers, 共%d页\n", maxImageWorkers, len(pages))
	var (
		mu   sync.Mutex
		wg   sync.WaitGroup
		sem  = make(chan struct{}, maxImageWorkers)
		errs []error
		done int
	)

	for i, p := range pages {
		pageNo := i + 1

		// 续做模式：已存在的图片直接计入 done
		if existingImages[pageNo] {
			done++
			g.emit(Progress{Phase: "生成绘本插画", Total: len(pages), Done: done, Title: plan.Title})
			continue
		}

		wg.Add(1)
		go func(idx int, page Page) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			// ctx 已取消：不启动任何新工作
			if err := ctx.Err(); err != nil {
				mu.Lock()
				errs = append(errs, &ImageError{Page: pageNo, Stage: "cancelled", Err: err})
				mu.Unlock()
				return
			}

			var data []byte
			var err error

			if idx == 0 {
				data, err = g.createPosterImage(ctx, plan)
			} else {
				data, err = g.createPageImage(ctx, plan, page)
			}

			if err != nil {
				// 区分 ctx 取消和普通失败
				if ctxErr := ctx.Err(); ctxErr != nil {
					mu.Lock()
					errs = append(errs, &ImageError{Page: pageNo, Stage: "cancelled", Err: ctxErr})
					mu.Unlock()
					return
				}
				fmt.Printf("⚠️  第%d页图片生成失败: %v，使用备用方案\n", pageNo, err)
				mu.Lock()
				errs = append(errs, &ImageError{Page: pageNo, Stage: "primary", Err: err})
				mu.Unlock()

				data, err = g.createFallbackImage(ctx, pageNo)
				if err != nil {
					mu.Lock()
					errs = append(errs, &ImageError{Page: pageNo, Stage: "fallback", Err: err})
					mu.Unlock()
					return
				}
			}

			imgFile := fmt.Sprintf("%d.jpg", pageNo)
			imgPath := filepath.Join(dir, "images", imgFile)
			if err := os.WriteFile(imgPath, data, 0644); err != nil {
				mu.Lock()
				errs = append(errs, &ImageError{Page: pageNo, Stage: "write", Err: err})
				mu.Unlock()
				return
			}

			mu.Lock()
			book.Pages[idx] = Page{
				PageNum: pageNo, Text: page.Text, Subtitle: page.Subtitle,
				Image: fmt.Sprintf("images/%s", imgFile),
			}
			done++
			g.emit(Progress{Phase: "生成绘本插画", Total: len(pages), Done: done, Title: plan.Title})
			mu.Unlock()
		}(i, p)
	}

	wg.Wait()

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return book, nil
}

// generateIllustrations 为每页生成纯英文视觉场景描述
func (g *Generator) generateIllustrations(ctx context.Context, plan PlanData, pages []Page) []string {
	var storyLines []string
	for i, p := range pages {
		storyLines = append(storyLines, fmt.Sprintf("Page %d: %s", i+1, p.Text))
	}

	chars := plan.EnglishCharacters
	if chars == "" {
		chars = fallbackIllustrationPrefix
	}

	visualID := g.getVisualID()
	vs := style.GetVisualStyle(visualID)

	prompt := fmt.Sprintf(`You are a children's book illustration describer. For each page below, write a SHORT visual scene description in ENGLISH ONLY.

STYLE (apply to ALL descriptions):
%s
%s

RULES:
- Output ENGLISH ONLY. No Chinese characters at all.
- Describe ONLY visual elements: character actions, expressions, setting, lighting, composition
- NEVER include any text, letters, titles, signs, speech bubbles, or captions in descriptions
- Each description: 1-2 sentences, 20-40 words
- Character appearances MUST stay consistent: %s
- Match the soft flat storybook style described above
- Use pastel colors: soft green, gentle blue, warm beige
- Low shading, clean shapes, friendly atmosphere
- Medium shot with slight low-angle perspective

Story content:
%s

Output format (one description per line, NO numbering, NO prefix):
description for page 1
description for page 2
...`, vs.BasePrompt, vs.ColorScheme, chars, strings.Join(storyLines, "\n"))

	var raw string
	var err error
	client := g.getTextClient()
	for attempt := 1; attempt <= 3; attempt++ {
		if cerr := ctx.Err(); cerr != nil {
			fmt.Printf("⚠️  插画描述被取消: %v\n", cerr)
			return nil
		}
		raw, err = client.Chat(ctx, prompt, 2048)
		if err == nil && strings.TrimSpace(raw) != "" {
			break
		}
		fmt.Printf("⚠️  插画描述生成失败 (第%d次): %v\n", attempt, err)
		if attempt < 3 {
			time.Sleep(3 * time.Second)
		}
	}
	if err != nil || strings.TrimSpace(raw) == "" {
		fmt.Printf("⚠️  插画描述全部失败，将使用简短场景关键词替代\n")
		return nil
	}

	lines := strings.Split(strings.TrimSpace(raw), "\n")
	var result []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// 去掉可能的前缀
		if idx := strings.Index(line, ": "); idx > 0 && idx < 30 {
			prefix := line[:idx]
			// 如果前缀包含 "page" 或数字，去掉
			if strings.Contains(strings.ToLower(prefix), "page") || regexp.MustCompile(`^\d+$`).MatchString(strings.TrimSpace(prefix)) {
				line = strings.TrimSpace(line[idx+2:])
			}
		}
		// 安全网：去除任何残留的中文字符
		line = stripChineseChars(line)
		if line != "" {
			result = append(result, line)
		}
	}
	return result
}

// createPosterImage 生成电影海报风格的封面
func (g *Generator) createPosterImage(ctx context.Context, plan PlanData) ([]byte, error) {
	// 使用风格系统的封面 prompt 组装器
	charIDs := g.detectCharacterIDs(plan)
	visualID := g.getVisualID()
	fullPrompt := style.AssembleCoverPrompt(plan.Title, charIDs, visualID)
	fullPrompt = stripChineseChars(fullPrompt)
	client := g.getImageClient()
	return client.ImageWithExtraRetries(ctx, fullPrompt, api.CoverImageRetries)
}

// createPageImage
func (g *Generator) createPageImage(ctx context.Context, plan PlanData, p Page) ([]byte, error) {
	// 使用风格系统的 prompt 组装器
	charIDs := g.detectCharacterIDs(plan)
	visualID := g.getVisualID()
	sceneID := style.MapSceneFromText(p.Text)
	emotionID := style.MapEmotionFromText(p.Text)

	req := style.ImagePromptRequest{
		CharacterIDs: charIDs,
		SceneID:      sceneID,
		EmotionID:    emotionID,
		VisualID:     visualID,
		Action:       p.Illustration,
		PageNum:      p.PageNum,
	}
	fullPrompt := style.AssembleImagePrompt(req)
	fullPrompt = stripChineseChars(fullPrompt)

	client := g.getImageClient()
	data, err := client.Image(ctx, fullPrompt)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// createFallbackImage 最简备用方案
func (g *Generator) createFallbackImage(ctx context.Context, pageNum int) ([]byte, error) {
	prompt := fmt.Sprintf("A warm colorful children's book illustration, friendly scene, page %d, cute characters, vibrant colors, NO text, NO letters, NO writing, NO Chinese characters, pure illustration only", pageNum)
	client := g.getImageClient()
	return client.ImageWithExtraRetries(ctx, prompt, api.FallbackRetries)
}

func shortTitle(text, fallback string) string {
	s := strings.TrimSpace(text)
	if s == "" {
		return fallback
	}
	s = strings.Split(s, "\n")[0]
	if len([]rune(s)) > 22 {
		s = string([]rune(s)[:22]) + "…"
	}
	return s
}

func (g *Generator) saveJSON(dir string, book *BookData) error {
	f, err := os.Create(filepath.Join(dir, "data.json"))
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(book)
}

func (g *Generator) renderHTML(dir string, book *BookData) (string, error) {
	raw, err := os.ReadFile(filepath.Join("web", "reader.html"))
	if err != nil {
		return "", err
	}
	pagesJSON, _ := json.Marshal(book.Pages)
	out := string(raw)
	out = strings.Replace(out, "{{.Title}}", template.HTMLEscapeString(book.Title), -1)
	out = strings.Replace(out, "{{.StoryType}}", template.HTMLEscapeString(book.StoryType), -1)
	out = strings.Replace(out, "{{.Theme}}", template.HTMLEscapeString(book.Theme), -1)
	out = strings.Replace(out, "{{.ImageModel}}", template.HTMLEscapeString(book.ImageModel), -1)
	out = strings.Replace(out, "{{.PageCount}}", fmt.Sprintf("%d", len(book.Pages)), -1)
	out = strings.Replace(out, "{{.PagesJSON}}", string(pagesJSON), -1)
	outPath := filepath.Join(dir, "index.html")
	return outPath, os.WriteFile(outPath, []byte(out), 0644)
}

func (g *Generator) emit(p Progress) {
	if g.onProgress != nil {
		g.onProgress(p)
	}
}

// stripChineseChars 去除字符串中所有中文字符（安全网）
func stripChineseChars(s string) string {
	var b strings.Builder
	for _, r := range s {
		if !unicode.Is(unicode.Han, r) {
			b.WriteRune(r)
		}
	}
	result := b.String()
	// 清理多余的空格
	result = regexp.MustCompile(`\s+`).ReplaceAllString(result, " ")
	return strings.TrimSpace(result)
}

// isEnglish 检查字符串是否主要为英文
func isEnglish(s string) bool {
	en := 0
	total := 0
	for _, r := range s {
		if unicode.IsLetter(r) {
			total++
			if r < 128 {
				en++
			}
		}
	}
	if total == 0 {
		return false
	}
	return float64(en)/float64(total) > 0.8
}

// detectCharacterIDs 从 plan 中检测角色 ID 列表
func (g *Generator) detectCharacterIDs(plan PlanData) []string {
	chars := plan.Characters + " " + plan.EnglishCharacters
	chars = strings.ToLower(chars)
	var ids []string
	// 检测警察类型
	if strings.Contains(chars, "特警") || strings.Contains(chars, "swat") {
		ids = append(ids, "swat")
	} else if strings.Contains(chars, "武警") || strings.Contains(chars, "armed") {
		ids = append(ids, "armed_police")
	} else if strings.Contains(chars, "刑警") || strings.Contains(chars, "detective") {
		ids = append(ids, "detective")
	} else if strings.Contains(chars, "消防") || strings.Contains(chars, "firefighter") {
		ids = append(ids, "firefighter")
	} else {
		ids = append(ids, "police")
	}
	// 检测儿童
	if strings.Contains(chars, "女孩") || strings.Contains(chars, "girl") {
		ids = append(ids, "girl_child")
	} else {
		ids = append(ids, "boy_child")
	}
	return ids
}

// detectTemplateID 根据 StoryType 和 Style 推测故事模板 ID
func (g *Generator) detectTemplateID() string {
	st := strings.ToLower(g.cfg.StoryType + " " + g.cfg.Style)
	switch {
	case strings.Contains(st, "迷路") || strings.Contains(st, "lost"):
		return "lost_child"
	case strings.Contains(st, "交通") || strings.Contains(st, "traffic"):
		return "traffic_safety"
	case strings.Contains(st, "消防") || strings.Contains(st, "fire"):
		return "fire_rescue"
	case strings.Contains(st, "刑警") || strings.Contains(st, "detective"):
		return "detective_mystery"
	case strings.Contains(st, "特警") || strings.Contains(st, "swat"):
		return "swat_action"
	case strings.Contains(st, "武警") || strings.Contains(st, "armed"):
		return "armed_rescue"
	case strings.Contains(st, "职业") || strings.Contains(st, "dream"):
		return "dream_job"
	default:
		return "lost_child" // 默认迷路救援
	}
}

// getVisualID 获取当前视觉风格 ID
func (g *Generator) getVisualID() string {
	// 从 ImageStyle 中检测风格 ID
	is := strings.ToLower(g.cfg.ImageStyle)
	if strings.Contains(is, "soft_flat") || strings.Contains(is, "soft flat") || strings.Contains(is, "软扁平") {
		return "soft_flat"
	}
	if strings.Contains(is, "warm") || strings.Contains(is, "暖") {
		return "soft_flat_warm"
	}
	if strings.Contains(is, "cool") || strings.Contains(is, "冷") {
		return "soft_flat_cool"
	}
	// 默认使用 soft_flat（Gemini 参考风格）
	return "soft_flat"
}
