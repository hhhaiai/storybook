package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	urlpkg "net/url"
	"strings"
	"time"
)

// Protocol 定义支持的 API 协议类型
type Protocol string

const (
	ProtocolOpenAI    Protocol = "openai"    // OpenAI Chat Completions / 兼容协议
	ProtocolResponses Protocol = "responses" // OpenAI Responses 协议
	ProtocolClaude    Protocol = "claude"    // Anthropic Claude Messages
	ProtocolGemini    Protocol = "gemini"    // Google Gemini
)

// NormalizeProtocol 接受 UI/配置里的常见别名，并规整成内部协议名。
func NormalizeProtocol(protocol string) Protocol {
	switch strings.ToLower(strings.TrimSpace(protocol)) {
	case "", "auto", "openai", "openai-compatible", "openai-chat", "openaichat", "chat", "chat-completions", "openai-chat-completions":
		return ProtocolOpenAI
	case "responses", "openai-response", "openai-responses", "openairesponse", "response":
		return ProtocolResponses
	case "claude", "anthropic", "anthropic-messages", "claude-messages":
		return ProtocolClaude
	case "gemini", "google", "google-gemini", "google-ai", "generativelanguage":
		return ProtocolGemini
	default:
		return ProtocolOpenAI
	}
}

// ProtocolAdapter 协议适配器接口
type ProtocolAdapter interface {
	ChatCompletion(ctx context.Context, model, prompt string) (string, error)
	ImageGeneration(ctx context.Context, model, prompt string) (string, error)
	ListModels(ctx context.Context) ([]ModelInfo, error)
}

// ModelInfo 模型信息
type ModelInfo struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
	Type string `json:"type,omitempty"` // text / image / vision
}

// NewAdapter 创建协议适配器
func NewAdapter(protocol Protocol, baseURL, apiKey string, timeout time.Duration) ProtocolAdapter {
	protocol = NormalizeProtocol(string(protocol))
	client := &http.Client{Timeout: timeout}
	switch protocol {
	case ProtocolOpenAI:
		return &OpenAIAdapter{baseURL: baseURL, apiKey: apiKey, client: client}
	case ProtocolResponses:
		return &ResponsesAdapter{baseURL: baseURL, apiKey: apiKey, client: client}
	case ProtocolClaude:
		return &ClaudeAdapter{baseURL: baseURL, apiKey: apiKey, client: client}
	case ProtocolGemini:
		return &GeminiAdapter{baseURL: baseURL, apiKey: apiKey, client: client}
	default:
		return &OpenAIAdapter{baseURL: baseURL, apiKey: apiKey, client: client}
	}
}

func endpoint(baseURL, path string) string {
	return strings.TrimRight(strings.TrimSpace(baseURL), "/") + path
}

func bearer(req *http.Request, apiKey string) {
	if strings.TrimSpace(apiKey) != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
}

func readAPIError(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	return fmt.Errorf("API error %d: %s", resp.StatusCode, sanitizeAPIErrorBody(string(body)))
}

func sanitizeAPIErrorBody(body string) string {
	var doc any
	if json.Unmarshal([]byte(body), &doc) == nil {
		if scrubbed, ok := scrubSensitiveFields(doc); ok {
			if data, err := json.Marshal(scrubbed); err == nil {
				return string(data)
			}
		}
	}
	return body
}

func scrubSensitiveFields(v any) (any, bool) {
	scrubbed := false
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, val := range x {
			lk := strings.ToLower(k)
			if strings.Contains(lk, "email") || strings.Contains(lk, "key") || strings.Contains(lk, "token") || strings.Contains(lk, "secret") {
				out[k] = "[redacted]"
				scrubbed = true
				continue
			}
			child, changed := scrubSensitiveFields(val)
			out[k] = child
			scrubbed = scrubbed || changed
		}
		return out, scrubbed
	case []any:
		out := make([]any, len(x))
		for i, val := range x {
			child, changed := scrubSensitiveFields(val)
			out[i] = child
			scrubbed = scrubbed || changed
		}
		return out, scrubbed
	default:
		return v, false
	}
}

func rawText(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &parts) == nil {
		var b strings.Builder
		for _, p := range parts {
			if p.Text != "" {
				b.WriteString(p.Text)
			}
		}
		return b.String()
	}
	return ""
}

func collectText(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case []any:
		var b strings.Builder
		for _, item := range x {
			b.WriteString(collectText(item))
		}
		return b.String()
	case map[string]any:
		if s, ok := x["output_text"].(string); ok {
			return s
		}
		if s, ok := x["text"].(string); ok {
			return s
		}
		if content, ok := x["content"]; ok {
			return collectText(content)
		}
	}
	return ""
}

// DetectProtocol 自动探测 API 支持的协议
func DetectProtocol(baseURL, apiKey string) (Protocol, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	protocols := []Protocol{ProtocolOpenAI, ProtocolResponses, ProtocolClaude, ProtocolGemini}
	for _, proto := range protocols {
		adapter := NewAdapter(proto, baseURL, apiKey, 5*time.Second)
		if _, err := adapter.ListModels(ctx); err == nil {
			return proto, nil
		}
	}
	return ProtocolOpenAI, nil
}

// ========== OpenAI 兼容协议适配器 ==========

type OpenAIAdapter struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

func (a *OpenAIAdapter) ChatCompletion(ctx context.Context, model, prompt string) (string, error) {
	if strings.TrimSpace(model) == "" {
		model = "gpt-4o-mini"
	}
	reqBody := map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"stream": false,
	}
	data, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint(a.baseURL, "/chat/completions"), bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	bearer(req, a.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", readAPIError(resp)
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content json.RawMessage `json:"content"`
			} `json:"message"`
			Text string `json:"text"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no response from API")
	}
	text := rawText(result.Choices[0].Message.Content)
	if text == "" {
		text = result.Choices[0].Text
	}
	if strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("empty response from API")
	}
	return text, nil
}

func (a *OpenAIAdapter) ImageGeneration(ctx context.Context, model, prompt string) (string, error) {
	if strings.TrimSpace(model) == "" {
		model = "gpt-image-1"
	}
	reqBody := map[string]any{
		"model":  model,
		"prompt": prompt,
		"n":      1,
	}
	data, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint(a.baseURL, "/images/generations"), bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	bearer(req, a.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", readAPIError(resp)
	}

	var result struct {
		Data []struct {
			URL      string `json:"url"`
			ImageURL string `json:"image_url"`
			B64JSON  string `json:"b64_json"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if len(result.Data) == 0 {
		return "", fmt.Errorf("no image data")
	}
	item := result.Data[0]
	if item.URL != "" {
		return item.URL, nil
	}
	if item.ImageURL != "" {
		return item.ImageURL, nil
	}
	if item.B64JSON != "" {
		return "data:image/png;base64," + item.B64JSON, nil
	}
	return "", fmt.Errorf("image response has no url or b64_json")
}

func (a *OpenAIAdapter) ListModels(ctx context.Context) ([]ModelInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint(a.baseURL, "/models"), nil)
	if err != nil {
		return nil, err
	}
	bearer(req, a.apiKey)

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, readAPIError(resp)
	}

	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	models := make([]ModelInfo, 0, len(result.Data))
	for _, m := range result.Data {
		if m.ID != "" {
			models = append(models, ModelInfo{ID: m.ID})
		}
	}
	return models, nil
}

// ========== Responses 协议适配器 ==========

type ResponsesAdapter struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

func (a *ResponsesAdapter) ChatCompletion(ctx context.Context, model, prompt string) (string, error) {
	if strings.TrimSpace(model) == "" {
		model = "gpt-4o-mini"
	}
	reqBody := map[string]any{
		"model":  model,
		"input":  prompt,
		"stream": false,
	}
	data, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint(a.baseURL, "/responses"), bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	bearer(req, a.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", readAPIError(resp)
	}

	var doc map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return "", err
	}
	text := collectText(doc["output_text"])
	if text == "" {
		text = collectText(doc["output"])
	}
	if text == "" {
		text = collectText(doc["choices"])
	}
	if strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("empty responses output")
	}
	return text, nil
}

func (a *ResponsesAdapter) ImageGeneration(ctx context.Context, model, prompt string) (string, error) {
	// 许多网关把图片仍挂在 OpenAI 兼容 /images/generations；这里优先复用，兼容更多模型。
	return (&OpenAIAdapter{baseURL: a.baseURL, apiKey: a.apiKey, client: a.client}).ImageGeneration(ctx, model, prompt)
}

func (a *ResponsesAdapter) ListModels(ctx context.Context) ([]ModelInfo, error) {
	models, err := (&OpenAIAdapter{baseURL: a.baseURL, apiKey: a.apiKey, client: a.client}).ListModels(ctx)
	if err != nil {
		return []ModelInfo{{ID: "unknown", Name: "Unknown Model"}}, nil
	}
	return models, nil
}

// ========== Claude 协议适配器 ==========

type ClaudeAdapter struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

func (a *ClaudeAdapter) ChatCompletion(ctx context.Context, model, prompt string) (string, error) {
	if strings.TrimSpace(model) == "" {
		model = "claude-3-5-sonnet-latest"
	}
	reqBody := map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"max_tokens": 4096,
	}
	data, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint(a.baseURL, "/messages"), bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(a.apiKey) != "" {
		req.Header.Set("x-api-key", a.apiKey)
		req.Header.Set("Authorization", "Bearer "+a.apiKey) // 兼容代理网关
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := a.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", readAPIError(resp)
	}

	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	var b strings.Builder
	for _, c := range result.Content {
		if c.Text != "" {
			b.WriteString(c.Text)
		}
	}
	if strings.TrimSpace(b.String()) == "" {
		return "", fmt.Errorf("no response")
	}
	return b.String(), nil
}

func (a *ClaudeAdapter) ImageGeneration(ctx context.Context, model, prompt string) (string, error) {
	return "", fmt.Errorf("claude protocol does not support image generation; choose an OpenAI-compatible image model/provider")
}

func (a *ClaudeAdapter) ListModels(ctx context.Context) ([]ModelInfo, error) {
	return []ModelInfo{
		{ID: "claude-3-5-sonnet-latest", Name: "Claude Sonnet", Type: "text"},
		{ID: "claude-3-5-haiku-latest", Name: "Claude Haiku", Type: "text"},
		{ID: "claude-sonnet-4-20250514", Name: "Claude Sonnet 4", Type: "text"},
	}, nil
}

// ========== Gemini 协议适配器 ==========

type GeminiAdapter struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

func (a *GeminiAdapter) geminiURL(path string) string {
	u := endpoint(a.baseURL, path)
	if strings.TrimSpace(a.apiKey) == "" {
		return u
	}
	sep := "?"
	if strings.Contains(u, "?") {
		sep = "&"
	}
	return u + sep + "key=" + urlpkg.QueryEscape(a.apiKey)
}

func (a *GeminiAdapter) ChatCompletion(ctx context.Context, model, prompt string) (string, error) {
	if strings.TrimSpace(model) == "" {
		model = "gemini-2.5-flash"
	}
	model = strings.TrimPrefix(model, "models/")
	reqBody := map[string]any{
		"contents": []map[string]any{
			{
				"role":  "user",
				"parts": []map[string]string{{"text": prompt}},
			},
		},
	}
	data, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, "POST", a.geminiURL("/models/"+model+":generateContent"), bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(a.apiKey) != "" {
		req.Header.Set("x-goog-api-key", a.apiKey)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", readAPIError(resp)
	}

	var result struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("no response")
	}
	var b strings.Builder
	for _, p := range result.Candidates[0].Content.Parts {
		b.WriteString(p.Text)
	}
	if strings.TrimSpace(b.String()) == "" {
		return "", fmt.Errorf("empty response")
	}
	return b.String(), nil
}

func (a *GeminiAdapter) ImageGeneration(ctx context.Context, model, prompt string) (string, error) {
	return "", fmt.Errorf("gemini protocol text is supported; for images choose an OpenAI-compatible image provider such as Imagen/GPT Image/Flux via a compatible gateway")
}

func (a *GeminiAdapter) ListModels(ctx context.Context) ([]ModelInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", a.geminiURL("/models"), nil)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(a.apiKey) != "" {
		req.Header.Set("x-goog-api-key", a.apiKey)
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return []ModelInfo{
			{ID: "gemini-2.5-flash", Name: "Gemini 2.5 Flash", Type: "text"},
			{ID: "gemini-2.5-pro", Name: "Gemini 2.5 Pro", Type: "text"},
		}, nil
	}

	var result struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return []ModelInfo{{ID: "gemini-2.5-flash", Type: "text"}}, nil
	}
	models := make([]ModelInfo, 0, len(result.Models))
	for _, m := range result.Models {
		id := strings.TrimPrefix(m.Name, "models/")
		if id != "" {
			models = append(models, ModelInfo{ID: id})
		}
	}
	return models, nil
}
