package config

import (
	"encoding/json"
	"mystorybook/internal/api"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// ModelEntry 模型条目
type ModelEntry struct {
	ID       string `json:"id"`                 // 模型 ID
	Model    string `json:"model,omitempty"`    // 上游真实模型 ID；为空时等于 ID
	Name     string `json:"name"`               // 显示名称
	Type     string `json:"type"`               // text / image / vision
	BaseURL  string `json:"base_url,omitempty"` // 独立端点（可选）
	APIKey   string `json:"api_key,omitempty"`  // 独立密钥（可选）
	Protocol string `json:"protocol,omitempty"` // 协议类型：openai/responses/claude/gemini
	Provider string `json:"provider,omitempty"` // 所属渠道 ID
}

// Provider 渠道配置（API 供应商）
type Provider struct {
	ID       string   `json:"id"`                 // 渠道唯一 ID
	Name     string   `json:"name"`               // 渠道名称（如 "Grok 远程"）
	BaseURL  string   `json:"base_url"`           // API 端点
	APIKey   string   `json:"api_key"`            // API 密钥
	Protocol string   `json:"protocol,omitempty"` // 优先协议（openai/responses/claude/gemini）
	Models   []string `json:"models,omitempty"`   // 已拉取的模型 ID 列表
}

// ProviderModelEntryID 返回渠道模型在本地配置中的唯一 ID。
// 这样 A 渠道和 B 渠道都有同名上游模型时，仍可分别选择。
func ProviderModelEntryID(providerID, modelID string) string {
	if providerID == "" {
		return modelID
	}
	return providerID + "::" + modelID
}

// InferModelType 根据模型名粗略判断用途，供自动拉取模型时落入正确下拉框。
func InferModelType(modelID string) string {
	lower := strings.ToLower(modelID)
	if strings.Contains(lower, "image") ||
		strings.Contains(lower, "dall") ||
		strings.Contains(lower, "imagine") ||
		strings.Contains(lower, "flux") ||
		strings.Contains(lower, "stable") ||
		strings.Contains(lower, "midjourney") ||
		strings.Contains(lower, "paint") ||
		strings.Contains(lower, "draw") ||
		strings.Contains(lower, "pic") {
		return "image"
	}
	if strings.Contains(lower, "vision") ||
		strings.Contains(lower, "ocr") ||
		strings.Contains(lower, "vl") {
		return "vision"
	}
	return "text"
}

// StylePreset 风格预设
type StylePreset struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Icon   string `json:"icon"`
	Desc   string `json:"desc"`
	Prompt string `json:"prompt"`
}

// RuntimeConfig 运行时配置
type RuntimeConfig struct {
	// 基础设置（不再直接使用，通过 CurrentProvider 切换）
	ProtocolType string `json:"protocol_type"`
	APIBaseURL   string `json:"api_base_url"`
	APIKey       string `json:"api_key"`
	OutputPath   string `json:"output_path"`

	// 当前选中的渠道 ID
	CurrentProvider string `json:"current_provider"`

	// 模型设置
	TextModel  string `json:"text_model"`
	ImageModel string `json:"image_model"`

	// 自定义模型列表
	CustomModels []ModelEntry `json:"custom_models"`

	// 渠道列表（API 供应商）
	Providers []Provider `json:"providers"`

	// 图片风格：选中的风格 ID 列表
	ImageStyles []string `json:"image_styles"`
	StoryStyles []string `json:"story_styles"`

	// 故事设置
	WorkType            string `json:"work_type"` // 选中的作品类型 ID
	StoryType           string `json:"story_type"`
	Theme               string `json:"theme"`
	Style               string `json:"style"`
	ImageStyle          string `json:"image_style"`
	NegativeImagePrompt string `json:"negative_image_prompt"`
	PromptTemplate      string `json:"prompt_template"`
	BatchCount          int    `json:"batch_count"`
	PageCount           int    `json:"page_count"`
	Port                int    `json:"port"`
}

// ImageStylePresets 图片风格预设
var ImageStylePresets = []StylePreset{
	// === Gemini 参考风格（推荐） ===
	{
		ID: "soft_flat", Name: "🎨 软扁平绘本（推荐）", Icon: "🎨",
		Desc:   "Gemini 参考风格：软扁平、低光影、高亲和力、温暖色调",
		Prompt: "children's storybook illustration, soft flat style, low shading, clean shapes, friendly warm atmosphere, emotion-first design, simple geometric forms, smooth edges, gentle gradients, pastel color palette with soft green, gentle blue, warm beige, low contrast, high brightness, no harsh shadows, medium shot, slight low-angle perspective, interactive composition, clean uncluttered background",
	},
	{
		ID: "soft_flat_warm", Name: "🌅 暖色软扁平", Icon: "🌅",
		Desc:   "偏暖色调的软扁平风格，金色阳光感",
		Prompt: "children's storybook illustration, soft flat warm style, low shading, clean friendly shapes, warm inviting atmosphere, emotion-first design, smooth edges, warm pastel palette with golden yellows, soft oranges, gentle pinks, cream whites, warm beige backgrounds, low contrast, high brightness, no harsh shadows",
	},
	{
		ID: "soft_flat_cool", Name: "🌊 冷色软扁平", Icon: "🌊",
		Desc:   "偏冷色调的软扁平风格，清新宁静感",
		Prompt: "children's storybook illustration, soft flat cool style, low shading, clean shapes, calm safe atmosphere, emotion-first design, smooth edges, cool pastel palette with soft blues, gentle teals, light lavenders, cool grays, mint accents, low contrast, high brightness, no harsh shadows",
	},

	// === 经典画风 ===
	{
		ID: "classic", Name: "经典绘本", Icon: "🎨",
		Desc:   "柔和水彩、温暖色调、翻书即入童话",
		Prompt: "classic children's picture book style, soft watercolor textures, warm gentle tones, hand-painted feel, traditional illustration, gentle color palette",
	},
	{
		ID: "watercolor", Name: "水彩手绘", Icon: "🖌️",
		Desc:   "水彩晕染、笔触可见、纸纹质感",
		Prompt: "hand-painted watercolor style, visible brush strokes, color bleeding and wet-on-wet effects, artistic and organic, paper texture visible, translucent layers",
	},
	{
		ID: "crayon", Name: "蜡笔童趣", Icon: "🖍️",
		Desc:   "蜡笔涂鸦感、稚拙可爱、孩子视角",
		Prompt: "crayon drawing style, waxy crayon texture, childlike charm, bold chunky strokes, vibrant saturated colors, slightly rough edges, crayon grain visible",
	},
	{
		ID: "colored-pencil", Name: "彩铅细腻", Icon: "✏️",
		Desc:   "彩色铅笔、细腻柔和、温润质感",
		Prompt: "colored pencil illustration, soft pencil strokes, delicate shading, warm paper tone, gentle layering, fine detail work, cozy and intimate feeling",
	},
	// === 数字画风 ===
	{
		ID: "cartoon", Name: "卡通动漫", Icon: "😜",
		Desc:   "线条鲜明、表情夸张、色彩饱满活泼",
		Prompt: "colorful cartoon style, bright vivid colors, bold clean outlines, exaggerated expressions, fun and playful, dynamic poses, animated feel",
	},
	{
		ID: "flat", Name: "扁平现代", Icon: "📐",
		Desc:   "几何色块、简洁利落、杂志感设计",
		Prompt: "modern flat design illustration, geometric shapes, clean edges, bold color blocks, minimalist, vector art feel, limited color palette, graphic design inspired",
	},
	{
		ID: "3d", Name: "3D 皮克斯", Icon: "🧊",
		Desc:   "立体圆润、光泽质感、动画电影感",
		Prompt: "3D rendered style, Pixar-like quality, soft rounded shapes, subsurface scattering on skin, volumetric lighting, cute and polished, smooth surfaces, depth of field",
	},
	{
		ID: "comic", Name: "漫画风", Icon: "💬",
		Desc:   "粗线条、速度线、动感分镜",
		Prompt: "comic book style, bold outlines, dynamic action lines, dramatic angles, manga-influenced, expressive faces, screen tones, high contrast",
	},
	// === 艺术风格 ===
	{
		ID: "fantasy", Name: "梦幻童话", Icon: "✨",
		Desc:   "魔法光晕、星星闪烁、仙境氛围",
		Prompt: "dreamy fantasy illustration, magical glowing effects, soft ethereal lighting, sparkles and stars, fairy tale atmosphere, whimsical, floating particles of light",
	},
	{
		ID: "ink", Name: "水墨国风", Icon: "🏔️",
		Desc:   "墨色浓淡、留白意境、东方诗韵",
		Prompt: "Chinese ink wash painting style, traditional brushwork, elegant composition, soft ink gradients, Eastern aesthetic, poetic atmosphere, mist and mountains",
	},
	{
		ID: "paper-cut", Name: "剪纸拼贴", Icon: "✂️",
		Desc:   "层层剪纸、民间艺术、立体层次",
		Prompt: "paper cut art style, flat layered shapes, folk art inspired, clean silhouettes, decorative patterns, collage feel, shadow box depth effect",
	},
	{
		ID: "vintage", Name: "复古怀旧", Icon: "📜",
		Desc:   "泛黄纸张、旧时光感、60年代画风",
		Prompt: "vintage illustration style, retro color palette, muted warm tones, nostalgic feel, classic 1960s picture book aesthetic, slightly faded colors, aged paper look",
	},
	// === 写实风格 ===
	{
		ID: "realistic", Name: "写实插画", Icon: "📷",
		Desc:   "细腻光影、接近真实、绘本写实派",
		Prompt: "realistic illustration style, detailed rendering, natural lighting and shadows, lifelike proportions, rich textures, careful attention to detail, atmospheric perspective",
	},
	{
		ID: "realistic-real", Name: "真人写实", Icon: "📸",
		Desc:   "照片级质感、真实人物、电影画面",
		Prompt: "photorealistic style, real human proportions, detailed skin textures and pores, natural lighting, cinematic quality, DSLR photo look, realistic facial features, true-to-life colors, depth of field, bokeh background",
	},
	// === 特殊风格 ===
	{
		ID: "sticker", Name: "贴纸风格", Icon: "🏷️",
		Desc:   "白边贴纸、可爱扁平、表情包感",
		Prompt: "sticker style illustration, white outline border, cute and flat design, emoji-like expressions, clean simple shapes, bright pop colors, die-cut sticker look",
	},
	{
		ID: "pixel", Name: "像素复古", Icon: "👾",
		Desc:   "像素点阵、8-bit风、游戏感",
		Prompt: "pixel art style, 8-bit retro, blocky pixels, limited color palette, nostalgic game aesthetic, crisp pixel edges, sprite-like characters",
	},
	{
		ID: "clay", Name: "黏土定格", Icon: "🫠",
		Desc:   "黏土质感、定格动画、手作温度",
		Prompt: "clay animation style, stop-motion look, plasticine texture, handmade feel, slight fingerprints visible, Aardman animation inspired, warm lighting on clay surfaces",
	},
}

// DefaultImageStyles 默认选中的风格
// StoryStylePresets 故事风格预设
var StoryStylePresets = []StylePreset{
	// === 情感类 ===
	{
		ID: "warm-growth", Name: "温暖成长", Icon: "🌱",
		Desc:   "情感细腻、正面引导、治愈系结局",
		Prompt: "温暖成长叙事，情感细腻真实，正面积极的价值观引导，结局温馨治愈",
	},
	{
		ID: "friendship", Name: "友情万岁", Icon: "💕",
		Desc:   "朋友互助、误会和解、友谊升华",
		Prompt: "关于友情的故事，从陌生到信任，经历误会与和解，最终友谊更加牢固",
	},
	{
		ID: "family-love", Name: "亲情暖心", Icon: "🏠",
		Desc:   "家人陪伴、亲情纽带、家的温暖",
		Prompt: "亲情故事，家人之间的关爱与陪伴，温馨的家庭氛围，感受家的温暖",
	},
	{
		ID: "lonely-together", Name: "从孤独到陪伴", Icon: "🤝",
		Desc:   "孤独角色、找到伙伴、不再孤单",
		Prompt: "从孤独到被理解的故事，角色从独自一人到找到真正的朋友，学会敞开心扉",
	},
	// === 冒险类 ===
	{
		ID: "adventure", Name: "冒险探索", Icon: "🗺️",
		Desc:   "未知旅程、重重挑战、勇气成长",
		Prompt: "充满挑战的冒险旅程，面对未知不退缩，每一步都是成长，最终收获勇气",
	},
	{
		ID: "treasure-hunt", Name: "寻宝之旅", Icon: "💎",
		Desc:   "神秘地图、线索解谜、宝藏发现",
		Prompt: "寻宝冒险故事，跟随神秘线索一步步前进，解谜过程紧张刺激，结局出人意料",
	},
	{
		ID: "rescue-mission", Name: "紧急救援", Icon: "🚨",
		Desc:   "危机时刻、争分夺秒、成功营救",
		Prompt: "紧急救援故事，危急关头挺身而出，争分夺秒克服困难，最终成功营救",
	},
	// === 职业类 ===
	{
		ID: "police-hero", Name: "警察故事", Icon: "🚔",
		Desc:   "维护正义、勇敢果断、保护弱者",
		Prompt: "警察英雄故事，维护正义保护弱者，勇敢果断面对危险，展现责任与担当",
	},
	{
		ID: "firefighter", Name: "消防救援", Icon: "🚒",
		Desc:   "烈火逆行、舍己救人、英雄本色",
		Prompt: "消防员的英勇故事，面对火场毫不退缩，用智慧和勇气保护大家的安全",
	},
	{
		ID: "doctor-heal", Name: "白衣天使", Icon: "👨‍⚕️",
		Desc:   "救死扶伤、温暖关怀、生命奇迹",
		Prompt: "医生治病救人的故事，温暖关怀每一位病人，展现生命的脆弱与坚强",
	},
	// === 悬疑/科幻 ===
	{
		ID: "suspense", Name: "悬疑推理", Icon: "🔎",
		Desc:   "蛛丝马迹、层层剥茧、真相浮现",
		Prompt: "悬疑推理故事，从细微线索入手，层层推进拨开迷雾，最终真相大白",
	},
	{
		ID: "sci-fi", Name: "科幻想象", Icon: "🚀",
		Desc:   "未来科技、星际旅行、奇妙发明",
		Prompt: "充满想象力的科幻故事，未来科技改变生活，星际冒险开拓视野，激发好奇心",
	},
	// === 趣味类 ===
	{
		ID: "humor", Name: "爆笑趣事", Icon: "😂",
		Desc:   "乌龙不断、笑料百出、欢乐收场",
		Prompt: "轻松搞笑的趣味故事，意外连连笑料不断，角色笨拙但善良，结局皆大欢喜",
	},
	{
		ID: "magic-fantasy", Name: "魔法奇遇", Icon: "🪄",
		Desc:   "魔法失控、奇幻冒险、意外收获",
		Prompt: "魔法世界的奇妙故事，一次意外触发神奇冒险，过程中学会魔法的真正意义",
	},
	{
		ID: "animal-friend", Name: "动物奇缘", Icon: "🐾",
		Desc:   "拟人动物、森林冒险、团队协作",
		Prompt: "拟人化动物的故事，在森林草原海洋中展开冒险，展现团队合作的力量",
	},
	// === 成长类 ===
	{
		ID: "first-day", Name: "第一次挑战", Icon: "🎒",
		Desc:   "上学第一天、害怕到适应、交到朋友",
		Prompt: "面对人生第一次的紧张与期待，从害怕到勇敢尝试，最终发现新世界很精彩",
	},
	{
		ID: "overcome-fear", Name: "战胜恐惧", Icon: "💪",
		Desc:   "害怕某事、逐步面对、变得勇敢",
		Prompt: "克服内心恐惧的故事，从害怕到一步步面对，发现自己比想象中更勇敢",
	},
	{
		ID: "nature-explore", Name: "自然探秘", Icon: "🌿",
		Desc:   "走进自然、发现奥秘、敬畏生命",
		Prompt: "探索大自然的奇妙旅程，发现动植物的生存智慧，培养对自然的敬畏与热爱",
	},
}

// DefaultStoryStyles 默认选中的故事风格
// WorkTypePreset 作品类型预设
type WorkTypePreset struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Icon       string `json:"icon"`
	Desc       string `json:"desc"`
	StoryType  string `json:"story_type"`  // 写入 {{STORY_TYPE}}
	PageRange  string `json:"page_range"`  // 建议页数范围
	ArtHint    string `json:"art_hint"`    // 画风补充提示
	LayoutHint string `json:"layout_hint"` // 布局提示
}

var WorkTypePresets = []WorkTypePreset{
	{
		ID: "kids-book", Name: "儿童绘本", Icon: "📖",
		Desc:      "3-8岁，大图少字，温馨简洁",
		StoryType: "儿童成长绘本", PageRange: "12-20",
		ArtHint:    "children picture book, large illustrations, simple compositions, warm and friendly",
		LayoutHint: "full page illustration with short text overlay",
	},
	{
		ID: "kids-story", Name: "儿童故事书", Icon: "📚",
		Desc:      "6-12岁，图文并茂，情节丰富",
		StoryType: "儿童故事绘本", PageRange: "16-30",
		ArtHint:    "children story book illustration, detailed scenes, expressive characters, story-driven compositions",
		LayoutHint: "illustration with text panel on side",
	},
	{
		ID: "comic-strip", Name: "连环漫画", Icon: "💬",
		Desc:      "分格漫画，对话气泡，动感十足",
		StoryType: "连环漫画", PageRange: "8-20",
		ArtHint:    "comic panel style, bold outlines, dynamic action, speech bubble ready, manga-influenced",
		LayoutHint: "multi-panel comic layout with clear gutters",
	},
	{
		ID: "4koma", Name: "四格漫画", Icon: "🔲",
		Desc:      "四格搞笑，节奏明快，结尾反转",
		StoryType: "四格漫画", PageRange: "6-12",
		ArtHint:    "4-koma manga style, simple clean lines, exaggerated expressions, comedic timing",
		LayoutHint: "four vertical panels per page, top to bottom reading",
	},
	{
		ID: "manga", Name: "少年漫画", Icon: "⚔️",
		Desc:      "热血冒险，分镜感强，少年向",
		StoryType: "少年漫画", PageRange: "12-24",
		ArtHint:    "shonen manga style, dynamic angles, speed lines, dramatic poses, detailed action scenes",
		LayoutHint: "varied panel sizes, dramatic splash pages",
	},
	{
		ID: "webtoon", Name: "条漫", Icon: "📱",
		Desc:      "竖屏长条，适合手机阅读",
		StoryType: "条漫", PageRange: "10-20",
		ArtHint:    "webtoon style, vertical scroll format, clean digital art, vibrant colors, cinematic framing",
		LayoutHint: "single tall vertical strip per page, scroll-friendly",
	},
	{
		ID: "educational", Name: "科普绘本", Icon: "🔬",
		Desc:      "知识融入故事，寓教于乐",
		StoryType: "科普教育绘本", PageRange: "14-24",
		ArtHint:    "educational illustration, clear informative visuals, labeled elements, bright engaging colors",
		LayoutHint: "illustration with informative callouts",
	},
	{
		ID: "fairy-tale", Name: "童话绘本", Icon: "🧚",
		Desc:      "经典童话风格，梦幻唯美",
		StoryType: "童话绘本", PageRange: "12-20",
		ArtHint:    "fairy tale illustration, enchanted atmosphere, magical lighting, storybook classic feel",
		LayoutHint: "full page fairy tale illustration",
	},
	{
		ID: "picture-book", Name: "无字绘本", Icon: "🖼️",
		Desc:      "纯图片叙事，几乎无文字",
		StoryType: "无字绘本", PageRange: "12-20",
		ArtHint:    "wordless picture book, visual storytelling, expressive scenes, narrative through imagery alone",
		LayoutHint: "full bleed illustration, minimal or no text",
	},
	{
		ID: "photo-story", Name: "真人绘本", Icon: "📸",
		Desc:      "写实真人风格，贴近生活",
		StoryType: "真人写实绘本", PageRange: "12-20",
		ArtHint:    "photorealistic illustration, real human proportions, realistic environments, cinematic quality, DSLR photo look",
		LayoutHint: "photo-realistic full page with text overlay",
	},
}
var DefaultStoryStyles = []string{"warm-growth"}

var DefaultImageStyles = []string{"soft_flat"}

// ModelPreset 模型预设
type ModelPreset struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"` // text / image / vision
	Desc string `json:"desc"`
	Base string `json:"base"` // 基础 API 地址
}

// ModelPresets 常用模型预设
var ModelPresets = []ModelPreset{
	// 图片生成模型
	{ID: "gpt-image-2", Name: "GPT Image 2", Type: "image", Desc: "GPT 图片生成模型 v2", Base: "http://47.85.40.209:3000/v1"},
	{ID: "grok-imagine-image-lite", Name: "Grok Imagine Lite", Type: "image", Desc: "xAI 图片生成（轻量）", Base: "https://your-api-endpoint.com/v1"},
	{ID: "grok-imagine-image", Name: "Grok Imagine", Type: "image", Desc: "xAI 图片生成（标准）", Base: "https://your-api-endpoint.com/v1"},
	{ID: "dall-e-3", Name: "DALL-E 3", Type: "image", Desc: "OpenAI 图片生成", Base: "https://api.openai.com/v1"},
	{ID: "dall-e-2", Name: "DALL-E 2", Type: "image", Desc: "OpenAI 图片生成（旧版）", Base: "https://api.openai.com/v1"},
	{ID: "gpt-image-1", Name: "GPT Image 1", Type: "image", Desc: "OpenAI 最新图片模型", Base: "https://api.openai.com/v1"},
	{ID: "stable-diffusion-xl", Name: "Stable Diffusion XL", Type: "image", Desc: "开源高质量图片生成", Base: ""},
	{ID: "flux-pro", Name: "Flux Pro", Type: "image", Desc: "Black Forest Labs 高质量", Base: ""},
	{ID: "flux-schnell", Name: "Flux Schnell", Type: "image", Desc: "Flux 快速生成", Base: ""},
	{ID: "cogview-4", Name: "CogView 4", Type: "image", Desc: "智谱 AI 图片生成", Base: "https://open.bigmodel.cn/api/paas/v4"},
	{ID: "kling-image", Name: "可灵图片", Type: "image", Desc: "快手可灵图片生成", Base: ""},
	// 文字对话模型
	{ID: "gpt-5-5", Name: "GPT-5.5", Type: "text", Desc: "GPT 对话模型", Base: "http://47.85.40.209:3000/v1"},
	{ID: "grok-4.20-0309-non-reasoning", Name: "Grok 4", Type: "text", Desc: "xAI 最新对话模型", Base: "https://your-api-endpoint.com/v1"},
	{ID: "gpt-4o", Name: "GPT-4o", Type: "text", Desc: "OpenAI 多模态模型", Base: "https://api.openai.com/v1"},
	{ID: "gpt-4o-mini", Name: "GPT-4o Mini", Type: "text", Desc: "OpenAI 轻量模型", Base: "https://api.openai.com/v1"},
	{ID: "claude-sonnet-4-20250514", Name: "Claude Sonnet 4", Type: "text", Desc: "Anthropic 最新模型", Base: "https://api.anthropic.com/v1"},
	{ID: "gemini-2.5-flash", Name: "Gemini 2.5 Flash", Type: "text", Desc: "Google 快速模型", Base: "https://generativelanguage.googleapis.com/v1beta"},
	{ID: "deepseek-chat", Name: "DeepSeek V3", Type: "text", Desc: "深度求索对话模型", Base: "https://api.deepseek.com/v1"},
	{ID: "qwen-max", Name: "通义千问 Max", Type: "text", Desc: "阿里通义大模型", Base: "https://dashscope.aliyuncs.com/compatible-mode/v1"},
	{ID: "glm-4-plus", Name: "GLM-4 Plus", Type: "text", Desc: "智谱 AI 对话模型", Base: "https://open.bigmodel.cn/api/paas/v4"},
	// 视觉/识别模型
	{ID: "gpt-4o", Name: "GPT-4o Vision", Type: "vision", Desc: "OpenAI 图片识别", Base: "https://api.openai.com/v1"},
	{ID: "claude-sonnet-4-20250514", Name: "Claude Vision", Type: "vision", Desc: "Anthropic 图片识别", Base: "https://api.anthropic.com/v1"},
}

var (
	current = RuntimeConfig{
		ProtocolType:        envOr("PROTOCOL_TYPE", "openai"),
		APIBaseURL:          envOr("USER_BASE_URL", ""),
		APIKey:              envOr("USER_API_KEY", ""),
		OutputPath:          envOr("OUTPUT_PATH", "outputs"),
		CurrentProvider:     "", // 启动后从持久化配置加载
		TextModel:           envOr("TEXT_MODEL", "gpt-5-5"),
		ImageModel:          envOr("IMAGE_MODEL", "gpt-image-2"),
		CustomModels:        []ModelEntry{},
		Providers:           []Provider{},
		ImageStyles:         DefaultImageStyles,
		StoryStyles:         DefaultStoryStyles,
		WorkType:            envOr("WORK_TYPE", "kids-book"),
		StoryType:           envOr("STORY_TYPE", "儿童成长绘本"),
		Theme:               envOr("STORY_THEME", ""),
		Style:               envOr("STORY_STYLE", "温暖成长叙事"),
		ImageStyle:          envOr("STORY_IMAGE_STYLE", buildImageStyleFromPresets(DefaultImageStyles)),
		NegativeImagePrompt: envOr("STORY_NEGATIVE_IMAGE_PROMPT", "text, letters, Chinese characters, Japanese characters, Korean characters, CJK characters, any writing, any words, any numbers, watermarks, signatures, speech bubbles, captions, titles, signs, labels, book covers with text, chibi, super-deformed, oversized head, toddler style"),
		PromptTemplate:      envOr("STORY_PROMPT_TEMPLATE", defaultPromptTemplate()),
		BatchCount:          envIntOr("BATCH_COUNT", 1),
		PageCount:           envIntOr("PAGE_COUNT", 20),
		Port:                envIntOr("PORT", 8080),
	}
	mu sync.RWMutex
)

// buildImageStyleFromPresets 根据选中的风格 ID 列表构建图片风格提示词
// buildStoryStyleFromPresets 根据选中的故事风格 ID 列表构建故事风格描述
func buildStoryStyleFromPresets(ids []string) string {
	idSet := map[string]bool{}
	for _, id := range ids {
		idSet[id] = true
	}
	var parts []string
	for _, p := range StoryStylePresets {
		if idSet[p.ID] {
			parts = append(parts, p.Prompt)
		}
	}
	if len(parts) == 0 {
		return "温暖成长叙事"
	}
	return strings.Join(parts, "，")
}

func buildImageStyleFromPresets(ids []string) string {
	idSet := map[string]bool{}
	for _, id := range ids {
		idSet[id] = true
	}
	var parts []string
	for _, p := range ImageStylePresets {
		if idSet[p.ID] {
			parts = append(parts, p.Prompt)
		}
	}
	if len(parts) == 0 {
		return "children's book illustration style, vibrant colors, clean lines, suitable for ages 6-10"
	}
	base := "children's book illustration, natural body proportions, suitable for ages 6-10, no chibi, no super-deformed, no oversized heads"
	return strings.Join(parts, ", ") + ", " + base
}

// GetImageStyle 根据当前选中的风格 ID 返回完整的图片风格提示词
func GetImageStyle() string {
	mu.RLock()
	defer mu.RUnlock()
	if len(current.ImageStyles) > 0 {
		return buildImageStyleFromPresets(current.ImageStyles)
	}
	return current.ImageStyle
}

func Get() RuntimeConfig {
	mu.RLock()
	defer mu.RUnlock()
	return current
}

// AddCustomModel 添加自定义模型
func AddCustomModel(entry ModelEntry) ([]ModelEntry, error) {
	mu.Lock()
	defer mu.Unlock()
	if entry.Model == "" {
		entry.Model = entry.ID
	}
	if entry.Name == "" {
		entry.Name = entry.Model
	}
	if entry.Protocol != "" {
		entry.Protocol = string(api.NormalizeProtocol(entry.Protocol))
	}
	for _, m := range current.CustomModels {
		if m.ID == entry.ID {
			return current.CustomModels, nil
		}
	}
	current.CustomModels = append(current.CustomModels, entry)
	return current.CustomModels, savePersistentConfigLocked()
}

// RemoveCustomModel 删除自定义模型
func RemoveCustomModel(id string) ([]ModelEntry, error) {
	mu.Lock()
	defer mu.Unlock()
	var result []ModelEntry
	for _, m := range current.CustomModels {
		if m.ID != id {
			result = append(result, m)
		}
	}
	current.CustomModels = result
	return result, savePersistentConfigLocked()
}

func defaultPromptTemplate() string {
	return PromptPresets["police"]
}

// PromptPresets 预设提示词模板
var PromptPresets = map[string]string{
	"police":      policePrompt(),
	"rescue":      rescuePrompt(),
	"catchThief":  catchThiefPrompt(),
	"armedRescue": armedRescuePrompt(),
	"armedCatch":  armedCatchPrompt(),
	"swat":        swatPrompt(),
	"detective":   detectivePrompt(),
	"adventure":   adventurePrompt(),
	"friendship":  friendshipPrompt(),
}

func policePrompt() string {
	return strings.Join([]string{
		"你是一名儿童绘本生成引擎，请严格执行以下规则。",
		"",
		"## 【核心任务】",
		"",
		"生成一本 {{PAGE_COUNT}} 页的{{STORY_TYPE}}，主题为：{{THEME}}。",
		"整体风格：{{STYLE}}。",
		"",
		"⚠️ 输出必须为纯中文绘本正文内容，禁止任何说明性或系统性语言。",
		"",
		"## 【强约束】",
		"",
		"1. 只允许输出中文",
		"2. 禁止出现英文、绘图提示词、画面描述等元信息",
		"3. 只输出故事正文，不解释、不总结",
		"4. 主角必须为：男警官（正面、温暖、可靠、勇敢）",
		"",
		"## 【分页输出格式】",
		"",
		"第1页",
		"（正文内容）",
		"",
		"第2页",
		"（正文内容）",
		"一直到第{{PAGE_COUNT}}页",
		"",
		"## 【单页内容规则】",
		"",
		"- 每页 1-3段落，每段 1-3句，每句 8-18 字",
		"- 语言自然，适合 5-10 岁儿童",
		"- 禁止重复句式（连续页不可相同结构）",
		"",
		"## 【语言与表达要求】",
		"",
		"情绪词（至少全部出现一次）：害怕、紧张、安心、开心、勇敢、担心、放松、感激",
		"动作词（至少全部出现一次）：走、看、抱、说、跑、拉、挡、护、安慰",
		"- 每页至少包含 1 个情绪或动作表达",
		"- 表达必须自然融入句子，不可堆砌",
		"",
		"## 【故事结构】",
		"",
		"- 前段（1-20%）：问题出现",
		"- 中段（20-70%）：困难逐步加深",
		"- 转折（70-85%）：出现帮助或关键行动",
		"- 结尾（85-100%）：问题解决 + 情绪回收",
		"",
		"## 【收尾要求】",
		"",
		"- 情绪转为安心/放松/开心",
		"- 至少出现一次感激或安慰",
		"- 整体氛围温暖、有安全感",
	}, "\n")
}

func rescuePrompt() string {
	return strings.Join([]string{
		"你是一名儿童绘本生成引擎，请生成一本 {{PAGE_COUNT}} 页的应急救援故事。",
		"主题：男武警在危险环境中救出被困的人。风格：紧张有力量、强调责任感。",
		"",
		"## 规则",
		"- 只输出中文故事正文",
		"- 禁止英文、提示词、画面描述",
		"- 每页 1-3段落，每段 1-3句，每句 8-18 字",
		"- 适合 5-10 岁儿童",
		"",
		"## 输出格式",
		"第1页",
		"（正文）",
		"一直到第{{PAGE_COUNT}}页",
		"",
		"## 故事结构",
		"开头：突发危险（如山路受阻）",
		"发展：被困人员害怕",
		"转折：男武警展开救援",
		"结尾：成功救出，感激放松",
	}, "\n")
}

func catchThiefPrompt() string {
	return strings.Join([]string{
		"你是一名儿童绘本生成引擎，请生成一本 {{PAGE_COUNT}} 页的正义行动故事。",
		"主题：男警官抓住偷东西的坏人。风格：紧张但不恐怖、节奏清晰。",
		"",
		"## 规则",
		"- 只输出中文故事正文",
		"- 禁止英文、提示词、画面描述",
		"- 每页 1-3段落，每段 1-3句，每句 8-18 字",
		"- 适合 5-10 岁儿童",
		"",
		"## 输出格式",
		"第1页",
		"（正文）",
		"一直到第{{PAGE_COUNT}}页",
		"",
		"## 故事结构",
		"开头：发生偷窃事件",
		"发展：坏人逃跑，大家紧张担心",
		"转折：男警官追赶并判断路线",
		"结尾：成功抓住坏人，大家安心",
	}, "\n")
}

func armedRescuePrompt() string {
	return strings.Join([]string{
		"你是一名儿童绘本生成引擎，请生成一本 {{PAGE_COUNT}} 页的应急救援故事。",
		"主题：男武警在危险环境中救出被困的人。风格：紧张有力量、强调责任感。",
		"",
		"## 规则",
		"- 只输出中文故事正文",
		"- 禁止英文、提示词、画面描述",
		"- 每页 1-3段落，每段 1-3句，每句 8-18 字",
		"- 适合 5-10 岁儿童",
		"",
		"## 输出格式",
		"第1页",
		"（正文）",
		"一直到第{{PAGE_COUNT}}页",
		"",
		"## 故事结构",
		"开头：突发危险",
		"发展：局势紧张",
		"转折：男武警展开救援",
		"结尾：成功救出，感激放松",
	}, "\n")
}

func armedCatchPrompt() string {
	return strings.Join([]string{
		"你是一名儿童绘本生成引擎，请生成一本 {{PAGE_COUNT}} 页的行动执行故事。",
		"主题：男武警抓捕危险坏人。风格：冷静果断、节奏紧凑。",
		"",
		"## 规则",
		"- 只输出中文故事正文",
		"- 禁止英文、提示词、画面描述",
		"- 每页 1-3段落，每段 1-3句，每句 8-18 字",
		"- 适合 5-10 岁儿童",
		"",
		"## 输出格式",
		"第1页",
		"（正文）",
		"一直到第{{PAGE_COUNT}}页",
		"",
		"## 故事结构",
		"开头：发现危险行为",
		"发展：局势紧张",
		"转折：男武警组织行动",
		"结尾：成功控制局面，大家安心",
	}, "\n")
}

func swatPrompt() string {
	return strings.Join([]string{
		"你是一名儿童绘本生成引擎，请生成一本 {{PAGE_COUNT}} 页的快速行动故事。",
		"主题：男特警迅速制服危险坏人。风格：节奏快、紧张感强。",
		"",
		"## 规则",
		"- 只输出中文故事正文",
		"- 禁止英文、提示词、画面描述",
		"- 每页 1-3段落，每段 1-3句，每句 8-18 字",
		"- 适合 5-10 岁儿童",
		"",
		"## 输出格式",
		"第1页",
		"（正文）",
		"一直到第{{PAGE_COUNT}}页",
		"",
		"## 故事结构",
		"开头：突发紧急事件",
		"发展：局势迅速升级",
		"转折：男特警快速行动",
		"结尾：成功制服，大家放松安心",
	}, "\n")
}

func detectivePrompt() string {
	return strings.Join([]string{
		"你是一名儿童绘本生成引擎，请生成一本 {{PAGE_COUNT}} 页的侦查推理故事。",
		"主题：男刑警通过线索找出真相。风格：冷静理性、层层推进。",
		"",
		"## 规则",
		"- 只输出中文故事正文",
		"- 禁止英文、提示词、画面描述",
		"- 每页 1-3段落，每段 1-3句，每句 8-18 字",
		"- 适合 5-10 岁儿童",
		"",
		"## 输出格式",
		"第1页",
		"（正文）",
		"一直到第{{PAGE_COUNT}}页",
		"",
		"## 故事结构",
		"开头：发生异常事件（物品丢失）",
		"发展：线索混乱，让人担心",
		"转折：男刑警观察并分析关键细节",
		"结尾：找出真相，大家感激安心",
	}, "\n")
}

func adventurePrompt() string {
	return strings.Join([]string{
		"你是一名儿童绘本生成引擎，请生成一本 {{PAGE_COUNT}} 页的{{STORY_TYPE}}。",
		"主题：{{THEME}}。风格：{{STYLE}}。",
		"",
		"## 规则",
		"- 只输出中文故事正文",
		"- 禁止英文、提示词、画面描述",
		"- 每页 1-3段落，每段 1-3句，每句 8-18 字",
		"- 适合 5-10 岁儿童",
		"",
		"## 输出格式",
		"第1页",
		"（正文）",
		"一直到第{{PAGE_COUNT}}页",
		"",
		"## 故事要求",
		"- 主角是一个勇敢的小探险家",
		"- 必须有明确的冒险目标和障碍",
		"- 结尾必须温暖正面，体现成长",
	}, "\n")
}

func friendshipPrompt() string {
	return strings.Join([]string{
		"你是一名儿童绘本生成引擎，请生成一本 {{PAGE_COUNT}} 页的{{STORY_TYPE}}。",
		"主题：{{THEME}}。风格：{{STYLE}}。",
		"",
		"## 规则",
		"- 只输出中文故事正文",
		"- 禁止英文、提示词、画面描述",
		"- 每页 1-3段落，每段 1-3句，每句 8-18 字",
		"- 适合 5-10 岁儿童",
		"",
		"## 输出格式",
		"第1页",
		"（正文）",
		"一直到第{{PAGE_COUNT}}页",
		"",
		"## 故事要求",
		"- 主角是一只善良的小动物",
		"- 必须有关于友谊的冲突和和解",
		"- 结尾必须体现友谊的力量",
	}, "\n")
}

// loadEnvFile 从 .env 文件加载环境变量（不覆盖已存在的）
func loadEnvFile() {
	data, err := os.ReadFile(".env")
	if err != nil {
		return // .env 不存在，静默跳过
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		kv := strings.SplitN(line, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.TrimSpace(kv[0])
		val := strings.TrimSpace(kv[1])
		if os.Getenv(key) == "" {
			os.Setenv(key, val)
		}
	}
}

var envOnce sync.Once

func ensureEnvLoaded() {
	envOnce.Do(func() {
		loadEnvFile()
	})
}

func envOr(key, fallback string) string {
	ensureEnvLoaded()
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envIntOr(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

// ConfigPatch 用于 API 更新配置。指针字段区分"未传"(nil) 和"传空"(非nil)。
// 未传的字段保持当前值不变，明确传入的字段（包括零值/空字符串）会覆盖。
type ConfigPatch struct {
	ProtocolType        *string      `json:"protocol_type"`
	APIBaseURL          *string      `json:"api_base_url"`
	APIKey              *string      `json:"api_key"`
	OutputPath          *string      `json:"output_path"`
	TextModel           *string      `json:"text_model"`
	ImageModel          *string      `json:"image_model"`
	CustomModels        []ModelEntry `json:"custom_models"`
	ImageStyles         []string     `json:"image_styles"`
	StoryStyles         []string     `json:"story_styles"`
	WorkType            *string      `json:"work_type"`
	StoryType           *string      `json:"story_type"`
	Theme               *string      `json:"theme"`
	Style               *string      `json:"style"`
	ImageStyle          *string      `json:"image_style"`
	NegativeImagePrompt *string      `json:"negative_image_prompt"`
	PromptTemplate      *string      `json:"prompt_template"`
	BatchCount          *int         `json:"batch_count"`
	PageCount           *int         `json:"page_count"`
	Port                *int         `json:"port"`
}

// ApplyPatch 将 ConfigPatch 应用到当前配置。
// nil 字段保持不变，非 nil 字段（包括零值）覆盖当前值。
func ApplyPatch(patch ConfigPatch) RuntimeConfig {
	mu.Lock()
	defer mu.Unlock()
	shouldPersist := false
	if patch.ProtocolType != nil {
		current.ProtocolType = *patch.ProtocolType
	}
	if patch.APIBaseURL != nil {
		current.APIBaseURL = *patch.APIBaseURL
	}
	if patch.APIKey != nil {
		current.APIKey = *patch.APIKey
	}
	if patch.OutputPath != nil {
		current.OutputPath = *patch.OutputPath
	}
	if patch.TextModel != nil {
		current.TextModel = *patch.TextModel
		shouldPersist = true
	}
	if patch.ImageModel != nil {
		current.ImageModel = *patch.ImageModel
		shouldPersist = true
	}
	if patch.CustomModels != nil {
		current.CustomModels = patch.CustomModels
		shouldPersist = true
	}
	if patch.ImageStyles != nil {
		current.ImageStyles = patch.ImageStyles
		current.ImageStyle = buildImageStyleFromPresets(patch.ImageStyles)
	}
	if patch.StoryStyles != nil {
		current.StoryStyles = patch.StoryStyles
		current.Style = buildStoryStyleFromPresets(patch.StoryStyles)
	}
	if patch.WorkType != nil {
		current.WorkType = *patch.WorkType
		for _, wt := range WorkTypePresets {
			if wt.ID == *patch.WorkType {
				current.StoryType = wt.StoryType
				break
			}
		}
	}
	if patch.StoryType != nil {
		current.StoryType = *patch.StoryType
	}
	if patch.Theme != nil {
		current.Theme = *patch.Theme
	}
	if patch.Style != nil {
		current.Style = *patch.Style
	}
	if patch.ImageStyle != nil && patch.ImageStyles == nil {
		current.ImageStyle = *patch.ImageStyle
	}
	if patch.NegativeImagePrompt != nil {
		current.NegativeImagePrompt = *patch.NegativeImagePrompt
	}
	if patch.PromptTemplate != nil {
		current.PromptTemplate = *patch.PromptTemplate
	}
	if patch.BatchCount != nil {
		current.BatchCount = *patch.BatchCount
	}
	if patch.PageCount != nil {
		if *patch.PageCount >= 12 && *patch.PageCount <= 30 {
			current.PageCount = *patch.PageCount
		}
	}
	if patch.Port != nil {
		current.Port = *patch.Port
	}
	if shouldPersist {
		_ = savePersistentConfigLocked()
	}
	return current
}

// GetModelConfig 获取模型的完整配置（端点、密钥、协议和实际请求模型 ID）
// 优先级：模型所属渠道 > CustomModels > CurrentProvider > 全局配置
func GetModelConfig(modelID string) (baseURL, apiKey, protocol, requestModel string) {
	mu.RLock()
	defer mu.RUnlock()

	// 1. 查找自定义模型（最高优先级）
	for _, m := range current.CustomModels {
		if m.ID == modelID {
			requestModel = m.Model
			if requestModel == "" {
				requestModel = m.ID
			}
			// 如果模型指定了 Provider，使用该渠道的配置
			if m.Provider != "" {
				for _, p := range current.Providers {
					if p.ID == m.Provider {
						baseURL := m.BaseURL
						if baseURL == "" {
							baseURL = p.BaseURL
						}
						apiKey := m.APIKey
						if apiKey == "" {
							apiKey = p.APIKey
						}
						protocol := m.Protocol
						if protocol == "" {
							protocol = p.Protocol
						}
						if protocol == "" {
							protocol = "openai"
						}
						return baseURL, apiKey, string(api.NormalizeProtocol(protocol)), requestModel
					}
				}
			}
			// 模型独立配置
			if m.BaseURL != "" && m.APIKey != "" {
				proto := m.Protocol
				if proto == "" {
					proto = "openai"
				}
				return m.BaseURL, m.APIKey, string(api.NormalizeProtocol(proto)), requestModel
			}
			break
		}
	}

	// 2. 使用当前选中的渠道
	if current.CurrentProvider != "" {
		for _, p := range current.Providers {
			if p.ID == current.CurrentProvider {
				proto := p.Protocol
				if proto == "" {
					proto = "openai"
				}
				return p.BaseURL, p.APIKey, string(api.NormalizeProtocol(proto)), modelID
			}
		}
	}

	// 3. 查找预设模型
	for _, m := range ModelPresets {
		if m.ID == modelID && m.Base != "" {
			// 预设模型使用其 Base 端点 + 全局密钥
			proto := current.ProtocolType
			if proto == "" {
				proto = "openai"
			}
			return m.Base, current.APIKey, string(api.NormalizeProtocol(proto)), modelID
		}
	}

	// 4. 回退到全局配置
	proto := current.ProtocolType
	if proto == "" {
		proto = "openai"
	}
	return current.APIBaseURL, current.APIKey, string(api.NormalizeProtocol(proto)), modelID
}

// GetAllAvailableModels 返回所有可用模型（预设 + 自定义）
func GetAllAvailableModels() []ModelEntry {
	mu.RLock()
	defer mu.RUnlock()

	var models []ModelEntry

	// 预设模型转为 ModelEntry
	for _, p := range ModelPresets {
		models = append(models, ModelEntry{
			ID:      p.ID,
			Name:    p.Name,
			Type:    p.Type,
			BaseURL: p.Base,
			APIKey:  "", // 预设模型不暴露密钥
		})
	}

	// 自定义模型
	models = append(models, current.CustomModels...)

	return models
}

// ========== 配置文件持久化 ==========

const configFilePath = "storybook_config.json"

// PersistentConfig 持久化配置（仅保存用户自定义内容）
type PersistentConfig struct {
	CustomModels    []ModelEntry `json:"custom_models,omitempty"`
	Providers       []Provider   `json:"providers,omitempty"`
	CurrentProvider string       `json:"current_provider,omitempty"` // 当前选中的渠道 ID
	TextModel       string       `json:"text_model,omitempty"`       // 当前选中的文本模型
	ImageModel      string       `json:"image_model,omitempty"`      // 当前选中的图片模型
}

// LoadPersistentConfig 从配置文件加载自定义模型和渠道
func LoadPersistentConfig() error {
	data, err := os.ReadFile(configFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // 文件不存在是正常的
		}
		return err
	}

	var pc PersistentConfig
	if err := json.Unmarshal(data, &pc); err != nil {
		return err
	}

	mu.Lock()
	defer mu.Unlock()
	current.CustomModels = pc.CustomModels
	current.Providers = pc.Providers
	current.CurrentProvider = pc.CurrentProvider
	if pc.TextModel != "" {
		current.TextModel = pc.TextModel
	}
	if pc.ImageModel != "" {
		current.ImageModel = pc.ImageModel
	}
	return nil
}

// SavePersistentConfig 保存自定义模型和渠道到配置文件
func SavePersistentConfig() error {
	mu.RLock()
	pc := PersistentConfig{
		CustomModels:    current.CustomModels,
		Providers:       current.Providers,
		CurrentProvider: current.CurrentProvider,
		TextModel:       current.TextModel,
		ImageModel:      current.ImageModel,
	}
	mu.RUnlock()

	data, err := json.MarshalIndent(pc, "", "  ")
	if err != nil {
		return err
	}

	// 确保目录存在
	if dir := filepath.Dir(configFilePath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	return os.WriteFile(configFilePath, data, 0644)
}

// savePersistentConfigLocked 在已持有锁的情况下保存配置（内部使用）
func savePersistentConfigLocked() error {
	pc := PersistentConfig{
		CustomModels:    current.CustomModels,
		Providers:       current.Providers,
		CurrentProvider: current.CurrentProvider,
		TextModel:       current.TextModel,
		ImageModel:      current.ImageModel,
	}

	data, err := json.MarshalIndent(pc, "", "  ")
	if err != nil {
		return err
	}

	if dir := filepath.Dir(configFilePath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	return os.WriteFile(configFilePath, data, 0644)
}

// AddProvider 添加新渠道
func AddProvider(p Provider) error {
	mu.Lock()
	defer mu.Unlock()
	current.Providers = append(current.Providers, p)
	return savePersistentConfigLocked()
}

// RemoveProvider 删除渠道
func RemoveProvider(id string) error {
	mu.Lock()
	defer mu.Unlock()
	filtered := []Provider{}
	for _, p := range current.Providers {
		if p.ID != id {
			filtered = append(filtered, p)
		}
	}
	current.Providers = filtered
	return savePersistentConfigLocked()
}

// UpdateProvider 更新渠道信息
func UpdateProvider(id string, updater func(*Provider)) error {
	mu.Lock()
	defer mu.Unlock()
	for i := range current.Providers {
		if current.Providers[i].ID == id {
			updater(&current.Providers[i])
			return savePersistentConfigLocked()
		}
	}
	return nil
}

// GetProviders 获取所有渠道
func GetProviders() []Provider {
	mu.RLock()
	defer mu.RUnlock()
	return append([]Provider{}, current.Providers...)
}
