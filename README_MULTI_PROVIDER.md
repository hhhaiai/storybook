# 多供应商支持说明

## ✨ 新功能

现在支持**多个 API 供应商并存**，每个模型可以独立配置端点和密钥。

---

## 🔧 配置方式

### 方式 1：Web 界面添加（推荐）

1. 启动服务：`./storybook`
2. 访问 http://localhost:8080
3. 点击右上角「⚙️ 设置」
4. 在「模型管理」区域填写：
   - **模型 ID**：如 `grok-4.20-0309-non-reasoning`
   - **类型**：文字对话 / 图片生成 / 识别图片
   - **名称**：显示名称（可选）
   - **API 端点**：如 `https://your-remote-api.example.com/v1`（留空使用全局配置）
   - **API 密钥**：如 `sk-xxx`（留空使用全局配置）
5. 点击「➕ 添加」

### 方式 2：命令行批量添加

运行测试脚本：

```bash
./test_multi_providers.sh
```

或手动执行：

```bash
# 添加 Grok 供应商的文本模型
curl -X POST http://localhost:8080/api/models/custom \
  -H "Content-Type: application/json" \
  -d '{
    "id": "grok-4.20-0309-non-reasoning",
    "name": "Grok 4 (远程)",
    "type": "text",
    "base_url": "https://your-remote-api.example.com/v1",
    "api_key": "sk-your-api-key-here"
  }'

# 添加 Grok 图片模型
curl -X POST http://localhost:8080/api/models/custom \
  -H "Content-Type: application/json" \
  -d '{
    "id": "grok-imagine-image-lite",
    "name": "Grok Imagine Lite (远程)",
    "type": "image",
    "base_url": "https://your-remote-api.example.com/v1",
    "api_key": "sk-your-api-key-here"
  }'

# 添加本地 GPT 文本模型
curl -X POST http://localhost:8080/api/models/custom \
  -H "Content-Type: application/json" \
  -d '{
    "id": "gpt-5-5-local",
    "name": "GPT-5.5 (本地)",
    "type": "text",
    "base_url": "http://127.0.0.1:3000/v1",
    "api_key": "your-local-api-key"
  }'

# 添加本地 GPT 图片模型
curl -X POST http://localhost:8080/api/models/custom \
  -H "Content-Type: application/json" \
  -d '{
    "id": "gpt-image-2-local",
    "name": "GPT Image 2 (本地)",
    "type": "image",
    "base_url": "http://127.0.0.1:3000/v1",
    "api_key": "your-local-api-key"
  }'
```

---

## 🎯 使用流程

1. **添加供应商模型**（见上方）
2. **在设置页选择模型**：
   - 进入「⚙️ 设置」→「模型选择」
   - 故事模型下拉框选择文本模型（如 `Grok 4 [your-remote-api.example.com]`）
   - 图片模型下拉框选择图片模型（如 `gpt-image-2-local [127.0.0.1:3000]`）
   - 点击「💾 保存模型选择」
3. **生成绘本**：回到主页点击「生成」，系统会自动使用所选模型的独立端点和密钥

---

## 📊 工作原理

### 模型查找优先级

1. **自定义模型**（带完整端点+密钥）→ 使用该模型的独立供应商
2. **预设模型**（只有端点，无密钥）→ 使用预设端点 + 全局密钥
3. **全局配置**（.env 中的 `USER_BASE_URL` + `USER_API_KEY`）

### 代码实现

**配置层** (`internal/config/config.go`)：
```go
// GetModelConfig 根据模型 ID 查找端点和密钥
func GetModelConfig(modelID string) (baseURL, apiKey string) {
    // 1. 查找自定义模型（优先）
    for _, m := range current.CustomModels {
        if m.ID == modelID && m.BaseURL != "" && m.APIKey != "" {
            return m.BaseURL, m.APIKey
        }
    }
    // 2. 查找预设模型
    for _, m := range ModelPresets {
        if m.ID == modelID && m.Base != "" {
            return m.Base, current.APIKey
        }
    }
    // 3. 回退到全局配置
    return current.APIBaseURL, current.APIKey
}
```

**生成层** (`internal/generator/generator.go`)：
```go
// 动态创建客户端，自动路由到对应供应商
func (g *Generator) getTextClient() *api.RuntimeClient {
    baseURL, apiKey := config.GetModelConfig(g.cfg.TextModel)
    return api.NewRuntimeClient(baseURL, apiKey, g.cfg.TextModel, "", 120*time.Second)
}

func (g *Generator) getImageClient() *api.RuntimeClient {
    baseURL, apiKey := config.GetModelConfig(g.cfg.ImageModel)
    return api.NewRuntimeClient(baseURL, apiKey, "", g.cfg.ImageModel, 120*time.Second)
}
```

---

## 🔍 示例配置

### 场景 1：混合使用远程和本地模型

```
文本模型：grok-4.20-0309-non-reasoning (远程)
  └─ https://your-remote-api.example.com/v1
  └─ sk-your-api-key-here

图片模型：gpt-image-2-local (本地)
  └─ http://127.0.0.1:3000/v1
  └─ your-local-api-key
```

### 场景 2：多个供应商备份

```
主力：
  文本 = grok-4.20 (远程)
  图片 = grok-imagine-image-lite (远程)

备用：
  文本 = gpt-5-5-local (本地)
  图片 = gpt-image-2-local (本地)
```

---

## ✅ 测试验证

### 1. 验证模型已添加

```bash
curl http://localhost:8080/api/models/custom | jq
```

### 2. 验证选择生效

```bash
curl http://localhost:8080/api/config | jq '.text_model, .image_model'
```

### 3. 生成测试绘本

在 Web 界面点击「生成」，观察日志中是否调用了对应端点。

---

## 🚨 注意事项

1. **端点和密钥留空** = 使用全局配置（`.env` 中的 `USER_BASE_URL` + `USER_API_KEY`）
2. **端点填写但密钥留空** = 使用该端点 + 全局密钥（适合同一供应商多个模型）
3. **删除模型** = 点击模型列表右侧的 `✕` 按钮
4. **密钥安全** = 自定义模型的密钥存储在内存中（未持久化到磁盘），重启后需重新添加

---

## 📁 相关文件

- `internal/config/config.go:10-17` - ModelEntry 扩展字段
- `internal/config/config.go:912-960` - GetModelConfig 查找逻辑
- `internal/generator/generator.go:102-123` - 动态客户端创建
- `cmd/server/main.go:326-356` - 添加模型 API
- `web/admin.html:308-337` - 添加模型表单
- `web/admin.html:728-750` - 添加模型 JS 函数

---

## 🎉 完成

现在你可以自由切换不同供应商的模型，实现：
- **成本优化**：文本用 Grok，图片用本地
- **高可用**：主力故障自动切换备用
- **功能对比**：快速测试不同模型效果
