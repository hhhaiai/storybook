package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log/slog"
	"math"
	"net/http"
	"strings"
	"time"
)

// logAPICall 记录 API 调用监控信息（结构化日志）
// 替代 main 里的 LogAPICall（避免循环依赖）
func logAPICall(kind, model string, dur time.Duration, ok bool, err error) {
	attrs := []any{
		slog.String("kind", kind),
		slog.String("model", model),
		slog.Int64("duration_ms", dur.Milliseconds()),
		slog.Bool("ok", ok),
	}
	if err != nil {
		attrs = append(attrs, slog.String("err", err.Error()))
	}
	slog.Default().Info("api_call", attrs...)
}

const (
	MinImageBytes     = 40 * 1024        // 40KB — solid-color fallbacks are ~13KB
	MaxImageRetries   = 3                // 默认图片重试次数
	CoverImageRetries = 4                // 封面海报重试次数（关键页面多试）
	FallbackRetries   = 1                // 兜底方案重试次数
	MaxResponseBytes  = 20 * 1024 * 1024 // 20MB — 图片下载最大字节数，防止 OOM
)

type RuntimeClient struct {
	BaseURL      string
	APIKey       string
	TextModel    string
	ImageModel   string
	Protocol     string // 协议类型：openai/responses/claude/gemini
	Timeout      time.Duration
	ImageTimeout time.Duration
	adapter      ProtocolAdapter // 协议适配器
}

func maxDuration(a, b time.Duration) time.Duration {
	if a > b {
		return a
	}
	return b
}

func NewRuntimeClient(baseURL, apiKey, textModel, imageModel string, timeout time.Duration, protocol string) *RuntimeClient {
	if protocol == "" {
		protocol = "openai" // 默认使用 OpenAI 协议
	}
	// 创建协议适配器
	protoType := NormalizeProtocol(protocol)

	adapter := NewAdapter(protoType, baseURL, apiKey, timeout)

	return &RuntimeClient{
		BaseURL:      baseURL,
		APIKey:       apiKey,
		TextModel:    textModel,
		ImageModel:   imageModel,
		Protocol:     protocol,
		Timeout:      timeout,
		ImageTimeout: maxDuration(90*time.Second, timeout),
		adapter:      adapter,
	}
}

const maxChatRetries = 3

func (c *RuntimeClient) Chat(ctx context.Context, prompt string, maxTokens int) (string, error) {
	var lastErr error
	for attempt := 1; attempt <= maxChatRetries; attempt++ {
		result, err := c.chatOnce(ctx, prompt, maxTokens)
		if err == nil {
			return result, nil
		}
		// 上下文已取消：立即返回，不重试
		if ctxErr := ctx.Err(); ctxErr != nil {
			return "", fmt.Errorf("chat cancelled: %w", ctxErr)
		}
		lastErr = err
		if attempt < maxChatRetries {
			time.Sleep(time.Duration(attempt*3) * time.Second)
		}
	}
	return "", fmt.Errorf("chat failed after %d attempts: %w", maxChatRetries, lastErr)
}

func (c *RuntimeClient) chatOnce(ctx context.Context, prompt string, maxTokens int) (result string, err error) {
	start := time.Now()
	defer func() {
		logAPICall("chat", c.TextModel, time.Since(start), err == nil, err)
	}()

	// 使用协议适配器
	if c.adapter != nil {
		result, err = c.adapter.ChatCompletion(ctx, c.TextModel, prompt)
		if err == nil && strings.TrimSpace(result) == "" {
			err = fmt.Errorf("chat returned empty content (model=%s, protocol=%s)", c.TextModel, c.Protocol)
		}
		if err == nil {
			fmt.Printf("  📡 API响应: %v (model=%s, protocol=%s, tokens≈%d)\n", time.Since(start).Round(time.Millisecond), c.TextModel, c.Protocol, len(result)/2)
		}
		return result, err
	}

	// 回退到原生 OpenAI 实现（如果适配器未初始化）
	payload := map[string]any{
		"model": c.TextModel,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"max_tokens": maxTokens,
		"stream":     false,
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", strings.TrimRight(c.BaseURL, "/")+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: c.Timeout}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("chat %d: %s", resp.StatusCode, string(raw))
	}
	// 检测 SSE：第一行以 "data:" 开头
	if isSSEResponse(raw) {
		var b strings.Builder
		for _, ln := range strings.Split(string(raw), "\n") {
			ln = strings.TrimSpace(ln)
			if !strings.HasPrefix(ln, "data:") {
				continue
			}
			j := strings.TrimSpace(strings.TrimPrefix(ln, "data:"))
			if j == "" || j == "[DONE]" {
				continue
			}
			var delta struct {
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
				} `json:"choices"`
			}
			if json.Unmarshal([]byte(j), &delta) == nil && len(delta.Choices) > 0 {
				b.WriteString(delta.Choices[0].Delta.Content)
			}
		}
		if b.Len() > 0 {
			return b.String(), nil
		}
		// SSE detected but extracted no content — still SSE, don't fall through to JSON
		return "", fmt.Errorf("SSE response contained no content (raw: %.200s)", string(raw))
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("chat decode failed: %w (raw: %.200s)", err, string(raw))
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("chat empty response (raw: %.300s)", string(raw))
	}
	result = out.Choices[0].Message.Content
	if strings.TrimSpace(result) == "" {
		return "", fmt.Errorf("chat returned empty content (model=%s, raw: %.300s)", c.TextModel, string(raw))
	}
	fmt.Printf("  📡 API响应: %v (model=%s, tokens≈%d)\n", time.Since(start).Round(time.Millisecond), c.TextModel, len(result)/2)
	return result, nil
}

func (c *RuntimeClient) Image(ctx context.Context, prompt string) ([]byte, error) {
	return c.imageWithRetries(ctx, prompt, MaxImageRetries)
}

// ImageWithExtraRetries 用更多重试次数生成图片（用于封面海报等关键页面）
func (c *RuntimeClient) ImageWithExtraRetries(ctx context.Context, prompt string, maxAttempts int) ([]byte, error) {
	return c.imageWithRetries(ctx, prompt, maxAttempts)
}

// imageWithRetries 统一的重试逻辑：ctx 取消立即返回，HTTP 错误/空图/纯色图都重试
func (c *RuntimeClient) imageWithRetries(ctx context.Context, prompt string, maxAttempts int) ([]byte, error) {
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		// 上下文已取消：立即返回
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("image cancelled: %w", err)
		}
		data, err := c.imageOnce(ctx, prompt)
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return nil, fmt.Errorf("image cancelled: %w", ctxErr)
			}
			lastErr = fmt.Errorf("attempt %d/%d: %w", attempt, maxAttempts, err)
			time.Sleep(time.Duration(attempt*2) * time.Second)
			continue
		}
		if !isValidImage(data) {
			lastErr = fmt.Errorf("attempt %d/%d: invalid format (size=%d)", attempt, maxAttempts, len(data))
			time.Sleep(time.Duration(attempt*2) * time.Second)
			continue
		}
		if len(data) < MinImageBytes {
			lastErr = fmt.Errorf("attempt %d/%d: too small (%d < %d)", attempt, maxAttempts, len(data), MinImageBytes)
			time.Sleep(time.Duration(attempt*2) * time.Second)
			continue
		}
		// 纯色图片检测：采样像素判断色彩是否过于单一
		if solid, reason := DetectSolidColor(data); solid {
			lastErr = fmt.Errorf("attempt %d/%d: solid-color image detected (%s, size=%d)", attempt, maxAttempts, reason, len(data))
			time.Sleep(time.Duration(attempt*2) * time.Second)
			continue
		}
		return data, nil
	}
	return nil, fmt.Errorf("image failed after %d attempts: %w", maxAttempts, lastErr)
}

// ValidateImage 验证图片是否为有效非纯色图片
func ValidateImage(data []byte) (bool, string) {
	if !isValidImage(data) {
		return false, "invalid format"
	}
	if len(data) < MinImageBytes {
		return false, fmt.Sprintf("too small (%d < %d)", len(data), MinImageBytes)
	}
	if solid, reason := DetectSolidColor(data); solid {
		return false, fmt.Sprintf("solid-color: %s", reason)
	}
	return true, ""
}

func (c *RuntimeClient) imageOnce(ctx context.Context, prompt string) (data []byte, err error) {
	start := time.Now()
	defer func() {
		logAPICall("image", c.ImageModel, time.Since(start), err == nil, err)
	}()

	// 使用协议适配器
	if c.adapter != nil {
		urlStr, err := c.adapter.ImageGeneration(ctx, c.ImageModel, prompt)
		if err != nil {
			return nil, err
		}
		if data, ok, err := decodeDataURL(urlStr); ok || err != nil {
			if err != nil {
				return nil, err
			}
			fmt.Printf("  🖼 图片返回: %v (base64 size=%dKB, protocol=%s)\n", time.Since(start).Round(time.Millisecond), len(data)/1024, c.Protocol)
			return data, nil
		}
		// 下载图片
		downloadReq, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
		if err != nil {
			return nil, fmt.Errorf("download: %w", err)
		}
		imgResp, err := (&http.Client{Timeout: c.ImageTimeout}).Do(downloadReq)
		if err != nil {
			return nil, fmt.Errorf("download: %w", err)
		}
		defer imgResp.Body.Close()
		if imgResp.StatusCode != 200 {
			b, _ := io.ReadAll(io.LimitReader(imgResp.Body, 1024*1024))
			return nil, fmt.Errorf("download %d: %s", imgResp.StatusCode, string(b))
		}
		data, _ = io.ReadAll(io.LimitReader(imgResp.Body, MaxResponseBytes))
		fmt.Printf("  🖼 图片下载: %v (size=%dKB, protocol=%s)\n", time.Since(start).Round(time.Millisecond), len(data)/1024, c.Protocol)
		return data, nil
	}

	// 回退到原生 OpenAI 实现
	payload := map[string]any{
		"model":  c.ImageModel,
		"prompt": prompt,
		"n":      1,
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", strings.TrimRight(c.BaseURL, "/")+"/images/generations", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: c.Timeout}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("api %d: %s", resp.StatusCode, sanitizeAPIErrorBody(string(raw)))
	}
	var out struct {
		Data []struct {
			URL     string `json:"url"`
			B64JSON string `json:"b64_json"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	if len(out.Data) == 0 {
		return nil, fmt.Errorf("empty response")
	}
	if out.Data[0].B64JSON != "" {
		data, err := base64.StdEncoding.DecodeString(out.Data[0].B64JSON)
		if err != nil {
			return nil, fmt.Errorf("decode b64_json: %w", err)
		}
		fmt.Printf("  🖼 图片返回: %v (base64 size=%dKB)\n", time.Since(start).Round(time.Millisecond), len(data)/1024)
		return data, nil
	}
	if out.Data[0].URL == "" {
		return nil, fmt.Errorf("image response has no url or b64_json")
	}
	downloadReq, err := http.NewRequestWithContext(ctx, "GET", out.Data[0].URL, nil)
	if err != nil {
		return nil, fmt.Errorf("download: %w", err)
	}
	imgResp, err := (&http.Client{Timeout: c.ImageTimeout}).Do(downloadReq)
	if err != nil {
		return nil, fmt.Errorf("download: %w", err)
	}
	defer imgResp.Body.Close()
	if imgResp.StatusCode != 200 {
		b, _ := io.ReadAll(io.LimitReader(imgResp.Body, 1024*1024))
		return nil, fmt.Errorf("download %d: %s", imgResp.StatusCode, string(b))
	}
	data, _ = io.ReadAll(io.LimitReader(imgResp.Body, MaxResponseBytes))
	fmt.Printf("  🖼 图片下载: %v (size=%dKB)\n", time.Since(start).Round(time.Millisecond), len(data)/1024)
	return data, nil
}

func decodeDataURL(s string) ([]byte, bool, error) {
	if !strings.HasPrefix(s, "data:") {
		return nil, false, nil
	}
	idx := strings.Index(s, ",")
	if idx < 0 {
		return nil, true, fmt.Errorf("invalid data url")
	}
	meta := s[:idx]
	payload := s[idx+1:]
	if !strings.Contains(meta, ";base64") {
		return nil, true, fmt.Errorf("unsupported data url encoding")
	}
	data, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return nil, true, fmt.Errorf("decode data url: %w", err)
	}
	return data, true, nil
}

// isSSEResponse 检测响应是否为 SSE 流格式（第一行非空以 "data:" 开头）
func isSSEResponse(raw []byte) bool {
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		return strings.HasPrefix(line, "data:")
	}
	return false
}

func isValidImage(data []byte) bool {
	if len(data) < 8 {
		return false
	}
	if data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
		return true
	}
	if data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47 {
		return true
	}
	return false
}

// DetectSolidColor 检测图片是否为纯色/近纯色
// 采样网格像素点，计算颜色标准差，如果过低则判定为纯色
func DetectSolidColor(data []byte) (bool, string) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return false, ""
	}

	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()
	if w < 10 || h < 10 {
		return false, ""
	}

	// 采样 10x10 网格
	sampleN := 10
	type rgb struct{ r, g, b float64 }
	samples := make([]rgb, 0, sampleN*sampleN)
	for sy := 0; sy < sampleN; sy++ {
		for sx := 0; sx < sampleN; sx++ {
			x := bounds.Min.X + (sx*w+w/2)/sampleN
			y := bounds.Min.Y + (sy*h+h/2)/sampleN
			r, g, b, _ := img.At(x, y).RGBA()
			samples = append(samples, rgb{
				r: float64(r) / 65535.0,
				g: float64(g) / 65535.0,
				b: float64(b) / 65535.0,
			})
		}
	}

	// 计算 RGB 各通道的均值和标准差
	var sumR, sumG, sumB float64
	for _, s := range samples {
		sumR += s.r
		sumG += s.g
		sumB += s.b
	}
	n := float64(len(samples))
	meanR := sumR / n
	meanG := sumG / n
	meanB := sumB / n

	var varR, varG, varB float64
	for _, s := range samples {
		varR += (s.r - meanR) * (s.r - meanR)
		varG += (s.g - meanG) * (s.g - meanG)
		varB += (s.b - meanB) * (s.b - meanB)
	}
	stdR := math.Sqrt(varR / n)
	stdG := math.Sqrt(varG / n)
	stdB := math.Sqrt(varB / n)

	// 综合标准差：如果所有通道标准差都很低，说明颜色单一
	maxStd := math.Max(stdR, math.Max(stdG, stdB))
	avgStd := (stdR + stdG + stdB) / 3.0

	// 阈值：标准差 < 0.03 认为是纯色（相当于 RGB 差异 < 8/255）
	if maxStd < 0.03 && avgStd < 0.02 {
		return true, fmt.Sprintf("uniform color (avgStd=%.4f, maxStd=%.4f, RGB=(%.0f,%.0f,%.0f))",
			avgStd, maxStd, meanR*255, meanG*255, meanB*255)
	}

	// 也检测渐变但色域很窄的情况（如蓝色渐变背景）
	colorRange := math.Max(meanR, math.Max(meanG, meanB)) - math.Min(meanR, math.Min(meanG, meanB))
	if maxStd < 0.05 && avgStd < 0.04 && colorRange < 0.15 {
		return true, fmt.Sprintf("narrow gradient (avgStd=%.4f, range=%.4f)", avgStd, colorRange)
	}

	return false, ""
}

func (c *RuntimeClient) ListModels(ctx context.Context) ([]string, error) {
	// 优先使用协议适配器
	if c.adapter != nil {
		models, err := c.adapter.ListModels(ctx)
		if err != nil {
			return nil, err
		}
		var ids []string
		for _, m := range models {
			ids = append(ids, m.ID)
		}
		return ids, nil
	}

	// 回退到原生实现
	req, err := http.NewRequestWithContext(ctx, "GET", strings.TrimRight(c.BaseURL, "/")+"/models", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	resp, err := (&http.Client{Timeout: c.Timeout}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("models %d: %s", resp.StatusCode, string(raw))
	}
	var out struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	var ids []string
	for _, m := range out.Data {
		ids = append(ids, m.ID)
	}
	return ids, nil
}
