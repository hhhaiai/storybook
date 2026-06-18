package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Protocol 定义支持的 API 协议类型
type Protocol string

const (
	ProtocolOpenAI    Protocol = "openai"    // OpenAI Chat Completions
	ProtocolResponses Protocol = "responses" // 新 Responses 协议
	ProtocolClaude    Protocol = "claude"    // Anthropic Claude Messages
	ProtocolGemini    Protocol = "gemini"    // Google Gemini
)

// NormalizeProtocol 接受 UI/配置里的常见别名，并规整成内部协议名。
func NormalizeProtocol(protocol string) Protocol {
	switch strings.ToLower(strings.TrimSpace(protocol)) {
	case "", "auto", "openai", "openai-chat", "openaichat", "chat", "chat-completions", "openai-chat-completions":
		return ProtocolOpenAI
	case "responses", "openai-response", "openai-responses", "openairesponse", "response":
		return ProtocolResponses
	case "claude", "anthropic":
		return ProtocolClaude
	case "gemini", "google", "google-gemini":
		return ProtocolGemini
	default:
		return ProtocolOpenAI
	}
}

// ProtocolAdapter 协议适配器接口
type ProtocolAdapter interface {
	// ChatCompletion 发送文本对话请求
	ChatCompletion(ctx context.Context, model, prompt string) (string, error)
	// ImageGeneration 发送图片生成请求
	ImageGeneration(ctx context.Context, model, prompt string) (string, error)
	// ListModels 获取模型列表
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

// DetectProtocol 自动探测 API 支持的协议
func DetectProtocol(baseURL, apiKey string) (Protocol, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 按优先级尝试各种协议
	protocols := []Protocol{ProtocolOpenAI, ProtocolResponses, ProtocolClaude, ProtocolGemini}
	for _, proto := range protocols {
		adapter := NewAdapter(proto, baseURL, apiKey, 5*time.Second)
		_, err := adapter.ListModels(ctx)
		if err == nil {
			return proto, nil
		}
	}
	// 默认返回 OpenAI 协议
	return ProtocolOpenAI, nil
}

// ========== OpenAI 协议适配器 ==========

type OpenAIAdapter struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

func (a *OpenAIAdapter) ChatCompletion(ctx context.Context, model, prompt string) (string, error) {
	if strings.TrimSpace(model) == "" {
		model = "gpt-4"
	}
	reqBody := map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"stream": false,
	}
	data, _ := json.Marshal(reqBody)
	req, _ := http.NewRequestWithContext(ctx, "POST", a.baseURL+"/chat/completions", bytes.NewReader(data))
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no response from API")
	}
	return result.Choices[0].Message.Content, nil
}

func (a *OpenAIAdapter) ImageGeneration(ctx context.Context, model, prompt string) (string, error) {
	if strings.TrimSpace(model) == "" {
		model = "dall-e-3"
	}
	reqBody := map[string]interface{}{
		"model":  model,
		"prompt": prompt,
		"n":      1,
	}
	data, _ := json.Marshal(reqBody)
	req, _ := http.NewRequestWithContext(ctx, "POST", a.baseURL+"/images/generations", bytes.NewReader(data))
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data []struct {
			URL string `json:"url"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if len(result.Data) == 0 {
		return "", fmt.Errorf("no image URL")
	}
	return result.Data[0].URL, nil
}

func (a *OpenAIAdapter) ListModels(ctx context.Context) ([]ModelInfo, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", a.baseURL+"/models", nil)
	req.Header.Set("Authorization", "Bearer "+a.apiKey)

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	models := make([]ModelInfo, len(result.Data))
	for i, m := range result.Data {
		models[i] = ModelInfo{ID: m.ID}
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
		model = "grok-4"
	}
	// 支持两种输入格式：字符串和消息数组
	reqBody := map[string]interface{}{
		"model":  model,
		"input":  prompt,
		"stream": false,
	}
	data, _ := json.Marshal(reqBody)
	req, _ := http.NewRequestWithContext(ctx, "POST", a.baseURL+"/responses", bytes.NewReader(data))
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Output string `json:"output"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	return result.Output, nil
}

func (a *ResponsesAdapter) ImageGeneration(ctx context.Context, model, prompt string) (string, error) {
	return "", fmt.Errorf("responses protocol does not support image generation")
}

func (a *ResponsesAdapter) ListModels(ctx context.Context) ([]ModelInfo, error) {
	// Responses 协议通常没有单独的 /models 端点，尝试 OpenAI 兼容端点
	req, _ := http.NewRequestWithContext(ctx, "GET", a.baseURL+"/models", nil)
	req.Header.Set("Authorization", "Bearer "+a.apiKey)

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return []ModelInfo{{ID: "unknown", Name: "Unknown Model"}}, nil
	}

	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return []ModelInfo{{ID: "unknown", Name: "Unknown Model"}}, nil
	}

	models := make([]ModelInfo, len(result.Data))
	for i, m := range result.Data {
		models[i] = ModelInfo{ID: m.ID}
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
		model = "claude-3-opus"
	}
	reqBody := map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"max_tokens": 4096,
	}
	data, _ := json.Marshal(reqBody)
	req, _ := http.NewRequestWithContext(ctx, "POST", a.baseURL+"/messages", bytes.NewReader(data))
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := a.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if len(result.Content) == 0 {
		return "", fmt.Errorf("no response")
	}
	return result.Content[0].Text, nil
}

func (a *ClaudeAdapter) ImageGeneration(ctx context.Context, model, prompt string) (string, error) {
	return "", fmt.Errorf("claude protocol does not support image generation")
}

func (a *ClaudeAdapter) ListModels(ctx context.Context) ([]ModelInfo, error) {
	// Claude 没有公开的 models 端点，返回常见模型
	return []ModelInfo{
		{ID: "claude-3-opus", Name: "Claude 3 Opus"},
		{ID: "claude-3-sonnet", Name: "Claude 3 Sonnet"},
		{ID: "claude-3-haiku", Name: "Claude 3 Haiku"},
	}, nil
}

// ========== Gemini 协议适配器 ==========

type GeminiAdapter struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

func (a *GeminiAdapter) ChatCompletion(ctx context.Context, model, prompt string) (string, error) {
	if strings.TrimSpace(model) == "" {
		model = "gemini-pro"
	}
	model = strings.TrimPrefix(model, "models/")
	reqBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"role": "user",
				"parts": []map[string]string{
					{"text": prompt},
				},
			},
		},
	}
	data, _ := json.Marshal(reqBody)
	// Gemini 端点格式：/v1beta/models/{model}:generateContent
	url := a.baseURL + "/models/" + model + ":generateContent"
	req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
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
	return result.Candidates[0].Content.Parts[0].Text, nil
}

func (a *GeminiAdapter) ImageGeneration(ctx context.Context, model, prompt string) (string, error) {
	return "", fmt.Errorf("gemini protocol does not support image generation yet")
}

func (a *GeminiAdapter) ListModels(ctx context.Context) ([]ModelInfo, error) {
	// Gemini models 端点
	req, _ := http.NewRequestWithContext(ctx, "GET", a.baseURL+"/models", nil)
	req.Header.Set("Authorization", "Bearer "+a.apiKey)

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		// 返回默认模型列表
		return []ModelInfo{
			{ID: "gemini-pro", Name: "Gemini Pro"},
			{ID: "gemini-pro-vision", Name: "Gemini Pro Vision"},
		}, nil
	}

	var result struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return []ModelInfo{{ID: "gemini-pro"}}, nil
	}

	models := make([]ModelInfo, len(result.Models))
	for i, m := range result.Models {
		models[i] = ModelInfo{ID: m.Name}
	}
	return models, nil
}
