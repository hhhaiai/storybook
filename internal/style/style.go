package style

import (
	"fmt"
	"strings"
)

// ============================================================
// 角色库 (Character Library)
// 统一描述，确保 AI 生成图片时人物一致
// ============================================================

// Character 角色定义
type Character struct {
	ID          string // 唯一标识
	Name        string // 中文名
	NameEN      string // 英文名
	Description string // 英文描述（用于图片 prompt）
	Traits      string // 性格特征
}

// Characters 角色库
var Characters = map[string]Character{
	"police": {
		ID:     "police",
		Name:   "警察叔叔",
		NameEN: "Police Officer Wang",
		Description: `a kind male police officer in his 30s, tall and upright posture, 
short neat black hair, warm gentle smile, dark blue simplified police uniform with silver badge, 
no complex details, friendly and approachable appearance, soft facial features, 
consistent character design throughout all pages`,
		Traits: "可靠、耐心、专业、温和",
	},
	"armed_police": {
		ID:     "armed_police",
		Name:   "武警叔叔",
		NameEN: "Armed Police Officer Li",
		Description: `a strong male armed police officer in his 30s, athletic build, 
short military haircut, determined but kind expression, olive green simplified military uniform, 
no complex equipment details, heroic but gentle appearance, soft facial features,
consistent character design throughout all pages`,
		Traits: "勇敢、果断、责任感强",
	},
	"swat": {
		ID:     "swat",
		Name:   "特警叔叔",
		NameEN: "SWAT Officer Zhang",
		Description: `a fit male SWAT officer in his 30s, strong but not intimidating, 
short black hair, focused yet calm expression, dark tactical uniform simplified, 
no complex gear details, protective and reliable appearance, soft facial features,
consistent character design throughout all pages`,
		Traits: "迅速、冷静、保护者",
	},
	"detective": {
		ID:     "detective",
		Name:   "刑警叔叔",
		NameEN: "Detective Chen",
		Description: `a smart male detective in his 30s, average build, 
neat short hair with slight parting, observant gentle eyes, casual dark coat over simple clothes, 
no complex details, intelligent and approachable appearance, soft facial features,
consistent character design throughout all pages`,
		Traits: "冷静、理性、善于观察",
	},
	"boy_child": {
		ID:     "boy_child",
		Name:   "小男孩",
		NameEN: "Little Boy",
		Description: `a cute little boy around 5-6 years old, round face, big sparkling eyes, 
short messy black hair, chubby cheeks, wearing a simple colorful t-shirt and shorts, 
small backpack, holding a teddy bear, expressive face showing clear emotions,
childlike proportions with slightly oversized head, soft features`,
		Traits: "好奇、信任、活泼",
	},
	"girl_child": {
		ID:     "girl_child",
		Name:   "小女孩",
		NameEN: "Little Girl",
		Description: `a cute little girl around 5-6 years old, round face, big bright eyes, 
two small pigtails with ribbons, chubby cheeks, wearing a simple colorful dress, 
small backpack, holding a stuffed bunny, expressive face showing clear emotions,
childlike proportions with slightly oversized head, soft features`,
		Traits: "天真、勇敢、善良",
	},
	"mother": {
		ID:     "mother",
		Name:   "妈妈",
		NameEN: "Mother",
		Description: `a kind mother in her early 30s, gentle face, warm smile, 
shoulder-length dark hair, wearing a simple casual outfit, 
caring and worried expression, soft warm appearance`,
		Traits: "温柔、关爱、担心",
	},
	"firefighter": {
		ID:     "firefighter",
		Name:   "消防员叔叔",
		NameEN: "Firefighter Liu",
		Description: `a brave male firefighter in his 30s, strong build, 
short hair under a simplified yellow helmet, determined kind face, 
yellow simplified firefighter jacket, no complex details, heroic but friendly appearance,
consistent character design throughout all pages`,
		Traits: "勇敢、迅速、保护者",
	},
}

// GetCharacter 获取角色
func GetCharacter(id string) (Character, bool) {
	c, ok := Characters[id]
	return c, ok
}

// GetCharacterPrompt 获取角色的英文描述（用于图片生成）
func GetCharacterPrompt(id string) string {
	if c, ok := Characters[id]; ok {
		return c.Description
	}
	return ""
}

// ============================================================
// 视觉风格系统 (Visual Style System)
// ============================================================

// VisualStyle 视觉风格定义
type VisualStyle struct {
	ID          string
	Name        string
	Icon        string
	Desc        string
	BasePrompt  string // 基础风格 prompt
	ColorScheme string // 色彩体系
	Composition string // 构图规则
	Shading     string // 光影规则
}

// VisualStyles 视觉风格库
var VisualStyles = []VisualStyle{
	{
		ID:   "soft_flat",
		Name: "软扁平绘本",
		Icon: "🎨",
		Desc: "Gemini 参考风格：软扁平、低光影、高亲和力",
		BasePrompt: `children's storybook illustration, soft flat style, 
low shading, clean shapes, friendly and warm atmosphere, 
emotion-first design, simple geometric forms, 
no complex textures, smooth edges, gentle gradients`,
		ColorScheme: `pastel color palette dominated by soft green (safety/nature), 
gentle blue (police/trust), warm beige (warmth), light coral accents, 
low contrast, high brightness, no harsh shadows, 
muted warm tones throughout`,
		Composition: `medium shot with characters occupying 60% of frame, 
slight low-angle perspective to enhance protector feeling, 
interactive composition showing hand-holding or eye contact between characters,
clean uncluttered background, characters centered`,
		Shading: `minimal shading, soft ambient occlusion only, 
no dramatic lighting, no harsh shadows, 
gentle gradient fills, flat color areas with subtle transitions`,
	},
	{
		ID:   "soft_flat_warm",
		Name: "暖色软扁平",
		Icon: "🌅",
		Desc: "偏暖色调的软扁平风格",
		BasePrompt: `children's storybook illustration, soft flat warm style, 
low shading, clean friendly shapes, warm inviting atmosphere, 
emotion-first design, simple forms, smooth edges`,
		ColorScheme: `warm pastel palette with golden yellows, soft oranges, 
gentle pinks, cream whites, warm beige backgrounds, 
low contrast, high brightness, no harsh shadows`,
		Composition: `medium shot, characters centered, 60% frame occupation, 
slight low-angle, interactive poses, clean background`,
		Shading: `minimal warm shading, soft golden ambient light, 
no dramatic shadows, gentle color transitions`,
	},
	{
		ID:   "soft_flat_cool",
		Name: "冷色软扁平",
		Icon: "🌊",
		Desc: "偏冷色调的软扁平风格",
		BasePrompt: `children's storybook illustration, soft flat cool style, 
low shading, clean shapes, calm and safe atmosphere, 
emotion-first design, simple forms, smooth edges`,
		ColorScheme: `cool pastel palette with soft blues, gentle teals, 
light lavenders, cool grays, mint accents, 
low contrast, high brightness, no harsh shadows`,
		Composition: `medium shot, characters centered, 60% frame occupation, 
slight low-angle, interactive poses, clean background`,
		Shading: `minimal cool shading, soft blue ambient light, 
no dramatic shadows, gentle color transitions`,
	},
}

// GetVisualStyle 获取视觉风格
func GetVisualStyle(id string) *VisualStyle {
	for i := range VisualStyles {
		if VisualStyles[i].ID == id {
			return &VisualStyles[i]
		}
	}
	return &VisualStyles[0] // 默认 soft_flat
}

// ============================================================
// 场景库 (Scene Library)
// ============================================================

// Scene 场景定义
type Scene struct {
	ID          string
	Name        string
	Description string // 英文场景描述
}

// Scenes 标准场景库
var Scenes = map[string]Scene{
	"park": {
		ID:   "park",
		Name: "公园",
		Description: `a peaceful children's park with soft green grass, 
a few simple trees with round canopies, a gentle blue sky with fluffy white clouds, 
a wooden bench, a small playground in the background, 
clean and uncluttered, pastel colors, warm sunlight`,
	},
	"street": {
		ID:   "street",
		Name: "街道",
		Description: `a friendly neighborhood street with clean sidewalks, 
simple low buildings with warm-colored facades, a few parked cars, 
green trees along the road, bright daylight, 
safe and welcoming atmosphere, minimal details`,
	},
	"school": {
		ID:   "school",
		Name: "学校门口",
		Description: `a cheerful school entrance with a colorful gate, 
simple school building facade, a small garden area, 
clean walkway, warm afternoon light, 
safe and inviting atmosphere, child-friendly design`,
	},
	"home": {
		ID:   "home",
		Name: "家门口",
		Description: `a cozy home entrance with a warm wooden door, 
a small front garden with flowers, a welcome mat, 
warm interior light glowing from inside, 
comfortable and safe feeling, soft pastel tones`,
	},
	"forest": {
		ID:   "forest",
		Name: "小树林",
		Description: `a gentle forest clearing with soft dappled sunlight, 
round friendly trees, green grass carpet, 
a few butterflies, gentle breeze effect, 
magical but safe atmosphere, pastel greens and yellows`,
	},
	"station": {
		ID:   "station",
		Name: "警察局",
		Description: `a friendly police station exterior with a blue awning, 
clean simple building, a welcoming entrance, 
flowers by the door, bright daylight, 
safe and professional atmosphere, simplified design`,
	},
}

// GetScene 获取场景
func GetScene(id string) (Scene, bool) {
	s, ok := Scenes[id]
	return s, ok
}

// ============================================================
// 情绪映射 (Emotion Mapping)
// ============================================================

// EmotionTag 情绪标签
type EmotionTag struct {
	ID       string // 情绪 ID
	Name     string // 中文名
	Modifier string // 图片 prompt 修饰词
}

// Emotions 情绪库
var Emotions = map[string]EmotionTag{
	"calm":      {ID: "calm", Name: "平静", Modifier: "calm and peaceful expression, relaxed atmosphere"},
	"worried":   {ID: "worried", Name: "担心", Modifier: "slightly worried expression, gentle tension, not scary"},
	"scared":    {ID: "scared", Name: "害怕", Modifier: "mildly frightened but not terrified, seeking comfort"},
	"relieved":  {ID: "relieved", Name: "安心", Modifier: "relieved and grateful expression, warm feeling"},
	"happy":     {ID: "happy", Name: "开心", Modifier: "happy and joyful expression, bright smile"},
	"admiring":  {ID: "admiring", Name: "崇拜", Modifier: "admiring look, eyes shining with respect and gratitude"},
	"determined": {ID: "determined", Name: "坚定", Modifier: "determined and focused expression, confident posture"},
	"caring":    {ID: "caring", Name: "关爱", Modifier: "caring and gentle expression, protective gesture"},
}

// GetEmotion 获取情绪
func GetEmotion(id string) (EmotionTag, bool) {
	e, ok := Emotions[id]
	return e, ok
}

// ============================================================
// Prompt 组装器 (Prompt Assembler)
// ============================================================

// ImagePromptRequest 图片生成请求
type ImagePromptRequest struct {
	CharacterIDs []string // 角色 ID 列表
	SceneID      string   // 场景 ID
	EmotionID    string   // 情绪 ID
	VisualID     string   // 视觉风格 ID
	Action       string   // 动作描述（英文）
	ExtraDetail  string   // 额外细节（英文）
	PageNum      int      // 页码（用于日志）
}

// AssembleImagePrompt 组装完整的图片生成 prompt
func AssembleImagePrompt(req ImagePromptRequest) string {
	var parts []string

	// 1. 视觉风格（最前面，锁死风格）
	style := GetVisualStyle(req.VisualID)
	parts = append(parts, style.BasePrompt)

	// 2. 色彩体系
	parts = append(parts, style.ColorScheme)

	// 3. 场景
	if scene, ok := GetScene(req.SceneID); ok {
		parts = append(parts, fmt.Sprintf("Scene: %s.", scene.Description))
	}

	// 4. 角色描述
	var charDescs []string
	for _, cid := range req.CharacterIDs {
		if c, ok := Characters[cid]; ok {
			charDescs = append(charDescs, c.Description)
		}
	}
	if len(charDescs) > 0 {
		parts = append(parts, fmt.Sprintf("Characters: %s.", strings.Join(charDescs, " and ")))
	}

	// 5. 动作
	if req.Action != "" {
		parts = append(parts, fmt.Sprintf("Action: %s.", req.Action))
	}

	// 6. 情绪
	if emotion, ok := GetEmotion(req.EmotionID); ok {
		parts = append(parts, fmt.Sprintf("Emotion: %s.", emotion.Modifier))
	}

	// 7. 构图
	parts = append(parts, fmt.Sprintf("Composition: %s.", style.Composition))

	// 8. 光影
	parts = append(parts, fmt.Sprintf("Lighting: %s.", style.Shading))

	// 9. 额外细节
	if req.ExtraDetail != "" {
		parts = append(parts, req.ExtraDetail)
	}

	// 10. 无文字约束（最后，最强）
	parts = append(parts, `CRITICAL: absolutely NO text, NO letters, NO Chinese characters, 
NO Japanese characters, NO Korean characters, NO words, NO writing, NO numbers, 
NO watermarks, NO signatures, NO speech bubbles, NO captions, NO signs, 
NO labels, NO book covers with text anywhere in the image. 
Pure illustration only. The image must contain zero readable text of any kind.`)

	return strings.Join(parts, " ")
}

// AssembleCoverPrompt 组装封面 prompt
func AssembleCoverPrompt(title string, characterIDs []string, visualID string) string {
	var parts []string

	style := GetVisualStyle(visualID)
	parts = append(parts, style.BasePrompt)
	parts = append(parts, style.ColorScheme)

	// 角色
	var charDescs []string
	for _, cid := range characterIDs {
		if c, ok := Characters[cid]; ok {
			charDescs = append(charDescs, c.Description)
		}
	}
	if len(charDescs) > 0 {
		parts = append(parts, fmt.Sprintf("Characters: %s.", strings.Join(charDescs, " and ")))
	}

	parts = append(parts, `Cover illustration: a warm and heroic scene, 
characters in a protective pose, poster-style composition, 
vibrant yet gentle colors, inspiring and safe feeling,
centered main characters, cinematic poster layout`)

	parts = append(parts, fmt.Sprintf("Composition: %s.", style.Composition))
	parts = append(parts, fmt.Sprintf("Lighting: %s.", style.Shading))

	// 无文字约束
	parts = append(parts, `CRITICAL: absolutely NO text, NO letters, NO Chinese characters, 
NO Japanese characters, NO Korean characters, NO words, NO writing, NO numbers, 
NO watermarks, NO signatures, NO speech bubbles, NO captions, NO signs, 
NO labels, NO book covers with text anywhere in the image. 
Pure illustration only. The image must contain zero readable text of any kind.`)

	return strings.Join(parts, " ")
}

// ============================================================
// 故事结构模板 (Story Structure Templates)
// ============================================================

// StoryTemplate 故事模板
type StoryTemplate struct {
	ID           string
	Name         string
	Theme        string
	Structure    string // 5段式结构
	EmotionCurve string // 情绪曲线
	CharacterIDs []string // 默认角色
	SceneIDs     []string // 默认场景
}

// StoryTemplates 故事模板库
var StoryTemplates = []StoryTemplate{
	{
		ID:    "lost_child",
		Name:  "迷路救援",
		Theme: "安全教育（走失、迷路）",
		Structure: `1. 场景引入：小朋友在公园开心玩耍（轻松、安全）
2. 问题出现：不小心走散了，找不到妈妈（轻微紧张）
3. 警察介入：警察叔叔出现，温柔安慰并询问情况（专业+温和）
4. 问题解决：通过对讲机联系，找到妈妈（正向结果）
5. 情感收束：小朋友感谢警察，梦想长大也要当警察（信任+成长）`,
		EmotionCurve: "平静 → 轻微紧张 → 放松 → 温暖+崇拜",
		CharacterIDs: []string{"police", "boy_child", "mother"},
		SceneIDs:     []string{"park", "street", "station"},
	},
	{
		ID:    "traffic_safety",
		Name:  "交通安全",
		Theme: "社会规则（交通、秩序）",
		Structure: `1. 场景引入：小朋友准备过马路（轻松日常）
2. 问题出现：红灯亮了但想冲过去（小冲突）
3. 警察介入：警察叔叔拦住并耐心讲解（专业引导）
4. 问题解决：等绿灯安全通过（规则遵守）
5. 情感收束：小朋友学会了交通规则，想教给朋友（成长+责任）`,
		EmotionCurve: "平静 → 紧张 → 安心 → 自豪",
		CharacterIDs: []string{"police", "boy_child"},
		SceneIDs:     []string{"street", "school"},
	},
	{
		ID:    "fire_rescue",
		Name:  "火灾救援",
		Theme: "帮助他人（救助）",
		Structure: `1. 场景引入：消防员叔叔在站里值班（平静日常）
2. 问题出现：警铃响起，有紧急情况（紧张）
3. 救援行动：迅速出动，展开救援（专业+勇敢）
4. 问题解决：成功救出被困的人（正向结果）
5. 情感收束：大家感激消防员，小朋友想当英雄（崇拜+梦想）`,
		EmotionCurve: "平静 → 紧张 → 坚定 → 感激+崇拜",
		CharacterIDs: []string{"firefighter", "boy_child"},
		SceneIDs:     []string{"station", "home"},
	},
	{
		ID:    "detective_mystery",
		Name:  "刑警破案",
		Theme: "勇敢与责任",
		Structure: `1. 场景引入：小朋友发现玩具不见了（日常小烦恼）
2. 问题出现：线索混乱，不知道怎么回事（困惑）
3. 刑警介入：刑警叔叔细心观察分析（冷静理性）
4. 问题解决：找到真相，玩具被风吹到了角落（真相大白）
5. 情感收束：小朋友学到观察力的重要性（成长+崇拜）`,
		EmotionCurve: "平静 → 困惑 → 期待 → 开心+崇拜",
		CharacterIDs: []string{"detective", "boy_child"},
		SceneIDs:     []string{"home", "park"},
	},
	{
		ID:    "swat_action",
		Name:  "特警出击",
		Theme: "勇敢与责任",
		Structure: `1. 场景引入：社区里的安静日常（平静）
2. 问题出现：发现危险情况（紧张但不恐怖）
3. 特警介入：特警叔叔迅速行动（果断+专业）
4. 问题解决：安全制服危险（正向结果）
5. 情感收束：大家安心，感谢特警叔叔（安全感+崇拜）`,
		EmotionCurve: "平静 → 紧张 → 坚定 → 安心+崇拜",
		CharacterIDs: []string{"swat", "boy_child"},
		SceneIDs:     []string{"street", "station"},
	},
	{
		ID:    "armed_rescue",
		Name:  "武警救援",
		Theme: "帮助他人（救助）",
		Structure: `1. 场景引入：户外活动的快乐场景（轻松）
2. 问题出现：突发危险情况（紧张）
3. 武警介入：武警叔叔展开救援（勇敢+果断）
4. 问题解决：成功救出被困者（正向结果）
5. 情感收束：感激武警叔叔，想成为保护者（崇拜+梦想）`,
		EmotionCurve: "平静 → 紧张 → 坚定 → 感激+崇拜",
		CharacterIDs: []string{"armed_police", "boy_child"},
		SceneIDs:     []string{"forest", "station"},
	},
	{
		ID:    "dream_job",
		Name:  "职业认知",
		Theme: "梦想与职业认知",
		Structure: `1. 场景引入：小朋友看到警察叔叔在巡逻（好奇）
2. 问题出现：小朋友有很多问题想问（好奇+期待）
3. 互动交流：警察叔叔耐心解答并展示装备（专业+亲切）
4. 体验感受：小朋友试戴警帽，开心极了（兴奋）
5. 情感收束：小朋友说长大也要当警察保护大家（梦想+崇拜）`,
		EmotionCurve: "好奇 → 期待 → 兴奋 → 崇拜+梦想",
		CharacterIDs: []string{"police", "boy_child"},
		SceneIDs:     []string{"station", "street"},
	},
}

// GetStoryTemplate 获取故事模板
func GetStoryTemplate(id string) *StoryTemplate {
	for i := range StoryTemplates {
		if StoryTemplates[i].ID == id {
			return &StoryTemplates[i]
		}
	}
	return nil
}

// ============================================================
// 故事文本 Prompt 生成器
// ============================================================

// StoryPromptRequest 故事文本请求
type StoryPromptRequest struct {
	TemplateID string // 故事模板 ID
	Theme      string // 自定义主题（覆盖模板）
	PageCount  int    // 页数
	Style      string // 风格
}

// AssembleStoryPrompt 组装故事文本生成 prompt
func AssembleStoryPrompt(req StoryPromptRequest) string {
	var sb strings.Builder

	// 核心定位
	sb.WriteString("你是一名儿童绘本生成引擎，请严格执行以下规则。\n\n")
	sb.WriteString("核心定位：儿童职业启蒙 + 安全感教育 + 正向价值观\n\n")

	// 主题和结构
	tmpl := GetStoryTemplate(req.TemplateID)
	if tmpl != nil {
		sb.WriteString(fmt.Sprintf("## 主题\n%s\n\n", tmpl.Theme))
		sb.WriteString(fmt.Sprintf("## 故事结构（5段式）\n%s\n\n", tmpl.Structure))
		sb.WriteString(fmt.Sprintf("## 情绪曲线\n%s\n\n", tmpl.EmotionCurve))
	}

	if req.Theme != "" {
		sb.WriteString(fmt.Sprintf("## 用户指定主题\n%s\n\n", req.Theme))
	}

	// 强约束
	sb.WriteString("## 强约束\n")
	sb.WriteString("1. 只允许输出中文\n")
	sb.WriteString("2. 禁止出现英文\n")
	sb.WriteString("3. 禁止出现绘图提示词\n")
	sb.WriteString("4. 禁止出现\"画面描述\"\"说明\"等元信息\n")
	sb.WriteString("5. 只输出故事正文，不解释、不总结\n\n")

	// 输出格式
	sb.WriteString(fmt.Sprintf("## 输出格式\n生成一本 %d 页的绘本。\n\n", req.PageCount))
	sb.WriteString("每一页格式：\n")
	sb.WriteString("第1页\n（正文内容）\n\n")
	sb.WriteString("第2页\n（正文内容）\n\n")
	sb.WriteString(fmt.Sprintf("一直到第%d页\n\n", req.PageCount))

	// 单页规则
	sb.WriteString("## 单页内容规则\n")
	sb.WriteString("- 每页 1-3 句\n")
	sb.WriteString("- 每句 8-18 字\n")
	sb.WriteString("- 语言自然，适合 3-8 岁儿童\n")
	sb.WriteString("- 短句、口语化、高可读\n")
	sb.WriteString("- 禁止重复句式（连续页不可相同结构）\n\n")

	// 语言风格
	sb.WriteString("## 语言风格要求\n")
	sb.WriteString("- 短句为主\n")
	sb.WriteString("- 口语化\n")
	sb.WriteString("- 高可读性\n")
	sb.WriteString("- 示例：\"小朋友，不用怕，我在这里。\"\n")
	sb.WriteString("- 示例：\"警察叔叔会帮你找到妈妈。\"\n\n")

	// 情绪词覆盖
	sb.WriteString("## 必须覆盖的表达\n")
	sb.WriteString("### 情绪词（至少全部出现一次）：\n")
	sb.WriteString("害怕、紧张、安心、开心、勇敢、担心、放松、感激\n\n")
	sb.WriteString("### 动作词（至少全部出现一次）：\n")
	sb.WriteString("走、看、抱、说、跑、拉、挡、护、安慰\n\n")
	sb.WriteString("要求：每页至少包含1个情绪或动作表达，自然融入句子。\n\n")

	// 收尾要求
	sb.WriteString("## 收尾要求\n")
	sb.WriteString("- 结尾必须体现安心/放松/开心\n")
	sb.WriteString("- 至少出现一次\"感激\"或\"安慰\"\n")
	sb.WriteString("- 整体氛围温暖、有安全感\n")
	sb.WriteString("- 体现信任与成长\n\n")

	// 禁止内容
	sb.WriteString("## 禁止内容\n")
	sb.WriteString("- 暴力细节\n")
	sb.WriteString("- 恐惧氛围\n")
	sb.WriteString("- 复杂人性冲突\n")

	return sb.String()
}

// ============================================================
// 场景映射器 (Scene Mapper)
// 根据故事内容自动选择合适的场景
// ============================================================

// MapSceneFromText 根据文本内容推测合适的场景
func MapSceneFromText(text string) string {
	text = strings.ToLower(text)
	
	// 关键词 -> 场景映射
	mappings := []struct {
		keywords []string
		sceneID  string
	}{
		{[]string{"公园", "玩耍", "滑梯", "秋千", "草地"}, "park"},
		{[]string{"马路", "街道", "过马路", "红绿灯", "交通"}, "street"},
		{[]string{"学校", "门口", "放学", "上学"}, "school"},
		{[]string{"家", "门口", "家里", "房间"}, "home"},
		{[]string{"森林", "树林", "户外", "山上"}, "forest"},
		{[]string{"警察局", "局里", "派出所"}, "station"},
	}

	for _, m := range mappings {
		for _, kw := range m.keywords {
			if strings.Contains(text, kw) {
				return m.sceneID
			}
		}
	}

	return "park" // 默认公园
}

// MapEmotionFromText 根据文本内容推测情绪
func MapEmotionFromText(text string) string {
	// 情绪关键词映射
	mappings := []struct {
		keywords []string
		emotionID string
	}{
		{[]string{"害怕", "恐惧", "怕", "吓"}, "scared"},
		{[]string{"担心", "着急", "紧张", "慌"}, "worried"},
		{[]string{"安心", "放心", "安全", "松了口气"}, "relieved"},
		{[]string{"开心", "高兴", "快乐", "笑"}, "happy"},
		{[]string{"感谢", "感激", "谢谢"}, "admiring"},
		{[]string{"勇敢", "坚定", "保护", "冲"}, "determined"},
		{[]string{"安慰", "温柔", "抱", "关心"}, "caring"},
	}

	for _, m := range mappings {
		for _, kw := range m.keywords {
			if strings.Contains(text, kw) {
				return m.emotionID
			}
		}
	}

	return "calm"
}
