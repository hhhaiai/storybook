# 多模型添加与组合使用演示

## ✅ 已支持的功能

1. ✅ **无限添加模型** - 可以添加任意数量的模型
2. ✅ **自动合并显示** - 预设模型 + 自定义模型都在下拉框中
3. ✅ **自由组合** - 文本模型和图片模型独立选择，任意搭配
4. ✅ **供应商标识** - 显示每个模型来自哪个供应商

---

## 📝 使用流程演示

### 第一步：添加多个模型

打开 http://localhost:8080 → 点击「⚙️ 设置」→ 在「模型管理」区域：

#### 添加第 1 个模型（Grok 文本）
```
模型 ID：grok-4.20-0309-non-reasoning
类型：文字对话
名称：Grok 4 高级推理
API 端点：https://your-remote-api.example.com/v1
API 密钥：sk-your-api-key-here
```
点击「➕ 添加」

#### 添加第 2 个模型（Grok 图片）
```
模型 ID：grok-imagine-image-lite
类型：图片生成
名称：Grok Imagine 快速版
API 端点：https://your-remote-api.example.com/v1
API 密钥：sk-your-api-key-here
```
点击「➕ 添加」

#### 添加第 3 个模型（本地 GPT 文本）
```
模型 ID：gpt-5-5
类型：文字对话
名称：GPT-5.5 本地版
API 端点：http://127.0.0.1:3000/v1
API 密钥：your-local-api-key
```
点击「➕ 添加」

#### 添加第 4 个模型（本地 GPT 图片）
```
模型 ID：gpt-image-2
类型：图片生成
名称：GPT Image 2 本地版
API 端点：http://127.0.0.1:3000/v1
API 密钥：your-local-api-key
```
点击「➕ 添加」

#### 添加第 5 个模型（OpenAI 官方）
```
模型 ID：gpt-4o
类型：文字对话
名称：GPT-4o 官方
API 端点：https://api.openai.com/v1
API 密钥：sk-your-openai-key
```
点击「➕ 添加」

---

### 第二步：查看自定义模型列表

添加完成后，在「自定义模型列表」会显示：

```
[模型列表]
┌──────────────────────────────────────────────────────────────┐
│ grok-4.20-0309-non-reasoning [your-remote-api.example.com] 文字对话 ✕ │
│ grok-imagine-image-lite [your-remote-api.example.com]      图片生成 ✕ │
│ gpt-5-5 [127.0.0.1:3000]                                   文字对话 ✕ │
│ gpt-image-2 [127.0.0.1:3000]                               图片生成 ✕ │
│ gpt-4o [api.openai.com]                                    文字对话 ✕ │
└──────────────────────────────────────────────────────────────┘
```

---

### 第三步：选择模型组合

在「模型选择」区域：

#### 📊 故事模型下拉框（文字对话）显示：
```
┌────────────────────────────────────────────┐
│ （使用默认）                                  │
│ ━━━━━━━━━━ 预设模型 ━━━━━━━━━━              │
│ GPT-5.5 (gpt-5-5)                          │
│ Grok 4 (grok-4.20-0309-non-reasoning)      │
│ GPT-4o (gpt-4o)                            │
│ GPT-4o Mini (gpt-4o-mini)                  │
│ Claude Sonnet 4 (claude-sonnet-4-20250514) │
│ Gemini 2.5 Flash (gemini-2.5-flash)       │
│ DeepSeek V3 (deepseek-chat)                │
│ 通义千问 Max (qwen-max)                     │
│ GLM-4 Plus (glm-4-plus)                    │
│ ━━━━━━━━━ 自定义模型 ━━━━━━━━━              │
│ Grok 4 高级推理 [your-remote-api.example.com]  │
│ GPT-5.5 本地版 [127.0.0.1:3000]            │
│ GPT-4o 官方 [api.openai.com]               │
└────────────────────────────────────────────┘
```

#### 🎨 图片模型下拉框（图片生成）显示：
```
┌────────────────────────────────────────────┐
│ （使用默认）                                  │
│ ━━━━━━━━━━ 预设模型 ━━━━━━━━━━              │
│ GPT Image 2 (gpt-image-2)                  │
│ Grok Imagine Lite (grok-imagine-image-lite)│
│ Grok Imagine (grok-imagine-image)          │
│ DALL-E 3 (dall-e-3)                        │
│ DALL-E 2 (dall-e-2)                        │
│ GPT Image 1 (gpt-image-1)                  │
│ Stable Diffusion XL (stable-diffusion-xl)  │
│ Flux Pro (flux-pro)                        │
│ Flux Schnell (flux-schnell)                │
│ CogView 4 (cogview-4)                      │
│ 可灵图片 (kling-image)                      │
│ ━━━━━━━━━ 自定义模型 ━━━━━━━━━              │
│ Grok Imagine 快速版 [your-remote-api.example.com] │
│ GPT Image 2 本地版 [127.0.0.1:3000]        │
└────────────────────────────────────────────┘
```

---

### 第四步：组合使用示例

#### 组合 1：远程文本 + 本地图片（省钱）
```
故事模型：Grok 4 高级推理 [your-remote-api.example.com]
图片模型：GPT Image 2 本地版 [127.0.0.1:3000]
```

#### 组合 2：本地文本 + 远程图片（快速文本）
```
故事模型：GPT-5.5 本地版 [127.0.0.1:3000]
图片模型：Grok Imagine 快速版 [your-remote-api.example.com]
```

#### 组合 3：全本地（完全离线）
```
故事模型：GPT-5.5 本地版 [127.0.0.1:3000]
图片模型：GPT Image 2 本地版 [127.0.0.1:3000]
```

#### 组合 4：全远程（高质量）
```
故事模型：Grok 4 高级推理 [your-remote-api.example.com]
图片模型：Grok Imagine 快速版 [your-remote-api.example.com]
```

#### 组合 5：混合三家（对比测试）
```
故事模型：Claude Sonnet 4 (预设)
图片模型：GPT Image 2 本地版 [127.0.0.1:3000]
```

---

### 第五步：保存并生成

1. 选择好组合后，点击「💾 保存模型选择」
2. 回到首页
3. 填写主题：`小警察帮助迷路的小朋友`
4. 点击「生成」

系统会自动：
- 用 **选择的文本模型** 生成故事文本
- 用 **选择的图片模型** 生成插画
- 自动路由到对应的 API 端点和密钥

---

## 🔄 动态切换测试

你可以快速切换模型对比效果：

### 测试 1：同一主题，不同文本模型
```
主题：小警察帮助迷路的小朋友

第 1 次生成：
  文本 = Grok 4
  图片 = GPT Image 2 本地版

第 2 次生成：
  文本 = GPT-5.5 本地版
  图片 = GPT Image 2 本地版（保持不变）

对比：故事文本风格差异
```

### 测试 2：同一主题，不同图片模型
```
主题：小警察帮助迷路的小朋友

第 1 次生成：
  文本 = Grok 4（保持不变）
  图片 = Grok Imagine 快速版

第 2 次生成：
  文本 = Grok 4（保持不变）
  图片 = GPT Image 2 本地版

对比：插画风格差异
```

---

## 📊 模型管理

### 删除模型
点击模型列表右侧的「✕」按钮即可删除

### 重命名模型
1. 删除旧模型
2. 重新添加，填写新的名称

### 查看所有模型
```bash
curl http://localhost:8080/api/models/custom | jq
```

### 批量删除（重置）
删除数据库文件后重启：
```bash
rm storybook.db
./storybook
```

---

## 🎯 最佳实践

### 1. 命名规范
```
✅ 好的命名：
  - "Grok 4 高级推理"（说明特点）
  - "GPT Image 2 本地版"（说明来源）
  - "DALL-E 3 官方"（说明供应商）

❌ 不好的命名：
  - "model1"（无意义）
  - "测试"（不明确）
```

### 2. 供应商分组
```
远程供应商：
  - Grok 系列（文本 + 图片）
  - OpenAI 系列（文本 + 图片）

本地供应商：
  - GPT 本地版（文本 + 图片）
  - Ollama 本地模型
```

### 3. 成本优化
```
文本生成（便宜） → 用远程高质量模型
图片生成（贵）   → 用本地免费模型
```

---

## 🚨 常见问题

### Q1：添加模型后下拉框看不到？
**A**：检查模型类型是否正确：
- 文本模型 → 只在「故事模型」下拉框显示
- 图片模型 → 只在「图片模型」下拉框显示

### Q2：模型列表太长，怎么快速找到？
**A**：使用浏览器搜索快捷键 `Ctrl+F`（Mac: `Cmd+F`）

### Q3：能添加同名模型吗？
**A**：不能，模型 ID 必须唯一。如果 ID 相同，第二次添加会被忽略。

### Q4：删除模型后能恢复吗？
**A**：不能自动恢复，需要重新添加。建议保存配置到文件。

### Q5：模型配置会持久化吗？
**A**：**目前不会**。重启服务后需要重新添加。
如需持久化，运行：
```bash
# 导出当前配置
curl http://localhost:8080/api/models/custom > my_models.json

# 重启后批量导入（需要写脚本）
cat my_models.json | jq -r '.models[] | @json' | while read m; do
  curl -X POST http://localhost:8080/api/models/custom \
    -H "Content-Type: application/json" \
    -d "$m"
done
```

---

## 🎉 总结

现在你的绘本生成器支持：

✅ 无限添加模型（不限数量）  
✅ 多供应商并存（远程 + 本地）  
✅ 自由组合（文本 + 图片独立选择）  
✅ 实时切换（无需重启）  
✅ 供应商标识（清晰显示来源）  

**一键添加多个供应商**：
```bash
./test_multi_providers.sh
```

**开始使用**：
```
http://localhost:8080 → ⚙️ 设置 → 模型管理 → 添加模型
```
