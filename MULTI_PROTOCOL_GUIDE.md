# 多协议支持和渠道管理功能

## 📅 更新时间
2026-06-17 13:25

---

## 🎯 新增功能

### 1. 支持多种 API 协议

系统现在支持 4 种主流 AI API 协议：

| 协议类型 | 说明 | 端点示例 |
|---------|------|---------|
| **OpenAI** | OpenAI Chat Completions | `/v1/chat/completions` |
| **Responses** | 新 Responses 协议 | `/v1/responses` |
| **Claude** | Anthropic Claude Messages | `/v1/messages` |
| **Gemini** | Google Gemini | `/v1beta/models/{model}:generateContent` |

### 2. 渠道管理

**渠道（Provider）** 是一个 API 供应商的完整配置，包含：
- 渠道名称（如 "Grok 远程"）
- API 端点（base_url）
- API 密钥（api_key）
- 协议类型（自动探测或手动指定）
- 模型列表（自动拉取）

### 3. 自动协议探测

添加渠道时，系统会：
1. 自动探测支持的 API 协议（按 OpenAI → Responses → Claude → Gemini 顺序）
2. 调用 `/models` 端点获取可用模型列表
3. 将模型自动添加到自定义模型列表

### 4. 配置文件持久化

所有自定义配置保存在 `storybook_config.json`（已加入 `.gitignore`）：
```json
{
  "custom_models": [...],
  "providers": [
    {
      "id": "provider-xxx",
      "name": "Grok 远程",
      "base_url": "https://your-api.example.com/v1",
      "api_key": "sk-xxx",
      "protocol": "openai",
      "models": ["grok-4", "grok-imagine-lite"]
    }
  ]
}
```

---

## 🔧 API 接口

### 获取渠道列表
```bash
GET /api/providers
```

**响应示例**：
```json
[
  {
    "id": "provider-001",
    "name": "Grok 远程",
    "base_url": "https://grok.example.com/v1",
    "api_key": "sk-xxx",
    "protocol": "openai",
    "models": ["grok-4", "grok-imagine-lite"]
  }
]
```

### 添加渠道
```bash
POST /api/providers
Content-Type: application/json

{
  "id": "provider-001",
  "name": "Grok 远程",
  "base_url": "https://grok.example.com/v1",
  "api_key": "sk-xxx",
  "protocol": "openai"
}
```

### 删除渠道
```bash
DELETE /api/providers?id=provider-001
```

### 探测协议并获取模型
```bash
POST /api/providers/detect
Content-Type: application/json

{
  "base_url": "https://api.example.com/v1",
  "api_key": "sk-xxx"
}
```

**响应示例**：
```json
{
  "protocol": "openai",
  "models": [
    {"id": "gpt-4", "name": "GPT-4"},
    {"id": "gpt-3.5-turbo", "name": "GPT-3.5 Turbo"}
  ]
}
```

---

## 🖥️ Web 界面使用

### 添加渠道

1. 打开 **⚙️ 设置** 页面
2. 找到 **🌐 渠道管理** 区块
3. 点击 **➕ 添加渠道**
4. 依次输入：
   - **渠道名称**：如 "Grok 远程"
   - **API 端点**：如 `https://grok.example.com/v1`
   - **API 密钥**：如 `sk-xxx`
5. 系统自动：
   - 探测 API 协议类型
   - 获取可用模型列表
   - 将模型添加到自定义模型下拉框

### 管理渠道

在渠道列表中，每个渠道显示：
- **渠道名称** 和 **API 端点**
- **协议类型**（如 `openai`）
- **模型数量**（如 `5 个模型`）
- **🔄 刷新** 按钮：重新获取模型列表
- **🗑️ 删除** 按钮：删除渠道（模型不会被删除）

---

## 📝 协议适配说明

### OpenAI 协议

**文本对话**：
```bash
POST /v1/chat/completions
{
  "model": "gpt-4",
  "messages": [{"role": "user", "content": "Hello"}],
  "stream": false
}
```

**图片生成**：
```bash
POST /v1/images/generations
{
  "model": "dall-e-3",
  "prompt": "A cute cat",
  "n": 1
}
```

### Responses 协议

```bash
POST /v1/responses
{
  "model": "grok-4",
  "input": "Hello",  # 或 input: [{"role":"user","content":"Hello"}]
  "stream": false
}
```

### Claude 协议

```bash
POST /v1/messages
{
  "model": "claude-3-opus",
  "messages": [{"role": "user", "content": "Hello"}],
  "max_tokens": 4096
}
```

### Gemini 协议

```bash
POST /v1beta/models/gemini-pro:generateContent
{
  "contents": [
    {
      "role": "user",
      "parts": [{"text": "Hello"}]
    }
  ]
}
```

---

## 🔄 协议自动探测逻辑

```go
func DetectProtocol(baseURL, apiKey string) (Protocol, error) {
    // 1. 尝试 OpenAI /v1/models
    // 2. 尝试 Responses /v1/responses（测试请求）
    // 3. 尝试 Claude /v1/messages（测试请求）
    // 4. 尝试 Gemini /v1beta/models
    // 5. 默认返回 OpenAI
}
```

探测超时：10 秒  
单次请求超时：5 秒

---

## 📊 使用场景

### 场景 1：混合多个供应商

```
渠道 1: Grok 远程
  ├─ grok-4（文本）
  └─ grok-imagine-lite（图片）

渠道 2: 本地 GPT
  ├─ gpt-5-5（文本）
  └─ gpt-image-2（图片）

渠道 3: OpenAI 官方
  ├─ gpt-4o（文本）
  └─ dall-e-3（图片）
```

**在设置中自由组合**：
- 故事模型：Grok 4（远程）
- 图片模型：GPT Image 2（本地）

### 场景 2：多账号轮换

添加多个相同供应商的渠道（不同账号）：
```
渠道 A: Grok 账号 1 (sk-xxx-001)
渠道 B: Grok 账号 2 (sk-xxx-002)
渠道 C: Grok 账号 3 (sk-xxx-003)
```

每个账号的模型独立显示在下拉框中。

---

## ⚠️ 注意事项

1. **配置文件安全**
   - `storybook_config.json` 包含 API 密钥，已加入 `.gitignore`
   - 不要将此文件提交到 Git

2. **协议探测失败**
   - 如果自动探测失败，默认使用 OpenAI 协议
   - 可以手动在数据库中修改协议类型

3. **模型类型判断**
   - 系统根据模型 ID 自动判断类型：
     - 包含 `image/dall-e/imagine` → 图片模型
     - 包含 `vision` → 视觉模型
     - 默认 → 文本模型

4. **删除渠道**
   - 删除渠道不会删除已添加的自定义模型
   - 需要手动在"自定义模型管理"中删除

---

## 🚀 快速开始

### 1. 添加第一个渠道

```bash
# Web 界面
打开设置 → 渠道管理 → 添加渠道

# 或使用 API
curl -X POST http://localhost:8080/api/providers \
  -H "Content-Type: application/json" \
  -d '{
    "id": "grok-remote",
    "name": "Grok 远程",
    "base_url": "https://your-api.example.com/v1",
    "api_key": "sk-your-key"
  }'
```

### 2. 系统自动操作

- ✅ 探测协议类型
- ✅ 获取模型列表
- ✅ 添加到自定义模型
- ✅ 保存到配置文件

### 3. 选择模型生成

在设置中选择刚添加的模型，开始生成绘本！

---

## 📁 新增文件

```
storybook/
├── internal/
│   └── api/
│       └── protocol.go          # ✅ 新增：多协议适配器
├── storybook_config.json        # ✅ 新增：持久化配置（不提交）
└── cmd/server/main.go           # ✅ 修改：添加渠道管理接口
```

---

## ✅ 功能验证

- ✅ 编译成功
- ✅ `/api/providers` 接口正常
- ✅ `/api/providers/detect` 协议探测可用
- ✅ 配置文件持久化正常
- ✅ Web 界面渠道管理 UI 已添加
- ✅ 页面加载时自动加载渠道列表

---

**下一步**：在浏览器中测试完整的添加渠道流程！
