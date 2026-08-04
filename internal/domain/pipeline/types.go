package pipeline

import (
	"github.com/ai-novel/studio/internal/infrastructure/llm"
)

// RunMode 运行模式（对应 run_mode 字段）
type RunMode string

const (
	ModeAuto         RunMode = "auto"         // 自动判定
	ModeDraft        RunMode = "draft"        // 快速草稿（跳过Thinker+Verifier）
	ModeCollab       RunMode = "collab"       // 多Agent协同闭环创作
	ModeOrchestrated RunMode = "orchestrated" // 手动指派每个Agent的模型+跑完整流水线
	ModeStrict       RunMode = "strict"       // 严谨模式
	ModeArt          RunMode = "art"          // 文艺创作模式
	ModeLight        RunMode = "light"        // 轻量化快速模式
	ModeManual       RunMode = "manual"       // 手动模式，直接调用指定模型
)

// PipelineName 流水线名称
type PipelineName string

// ChapterBreakMarker 分章标记：后端分段生成时在段间插入，前端据此拆分为独立章节
const ChapterBreakMarker = "\n\n[=====AI-NOVEL-CHAPTER-BREAK=====]\n\n"

const (
    PipelineStandard    PipelineName = "standard"    // 标准通用创作
    PipelineDraft       PipelineName = "draft"       // 快速草稿（Worker直出）
    PipelineCollab      PipelineName = "collab"      // 多Agent协同闭环
    PipelineOrchestrated PipelineName = "orchestrated" // 手动指派Agent模型+完整流水线
    PipelineStrict      PipelineName = "strict"      // 严谨模式
	PipelineArt      PipelineName = "art"      // 文艺创作模式
	PipelineLight    PipelineName = "light"    // 轻量化快速模式
	PipelineManual   PipelineName = "manual"   // 手动直调
)

// GenerateRequest 创作请求（POST /api/generate 请求体）
type GenerateRequest struct {
	ProjectID         string  `json:"project_id"`
	ChapterID         string  `json:"chapter_id"`
	UserDemand        string  `json:"user_demand"`
	SelectedText      string  `json:"selected_text"`
	Outline           string  `json:"outline"`           // 用户手填的章节/全书大纲
	RewriteOutline    bool    `json:"rewrite_outline"`   // 用户大纲是否让规划师完善（true=规划师读大纲补充细节；false=完全按用户大纲直接写）
	WorldSetting      string  `json:"world_setting"`
	CharacterSetting  string  `json:"character_setting"`
	HistoryContent    string  `json:"history_content"`
	MaterialText      string  `json:"material_text"`
	TargetWord        int     `json:"target_word"`
	RunMode           RunMode `json:"run_mode"`
	ModelName         string  `json:"model_name"`
	ModelConfigID     string  `json:"model_config_id"`
	CursorPosition    int     `json:"cursor_position"`
	NoRewrite         bool    `json:"no_rewrite"`
	ContextScope      string  `json:"context_scope"`
	PreviousSummaries string  `json:"previous_summaries"`
	SkipWordCheck     bool              `json:"skip_word_check"`     // 用户临时关闭字数校验
	RoleModels        map[string]string `json:"role_models"`         // orchestrated模式：手动指派每个Agent的模型（key:thinker/worker/verifier/helper, value:model_name）
	RoleThinking      map[string]bool   `json:"role_thinking"`       // 每个角色是否开启深度思考（key:thinker/worker/verifier/helper；缺省/未指定=开）
	WebSearch         bool              `json:"web_search"`          // 用户手动开启：联网搜索辅助各 Agent
	WebInfo           string            `json:"-"`                   // 后端填充：检索到的联网参考信息（已格式化）
	StyleSampleIDs    []string          `json:"style_sample_ids"`    // 文风样本库样本 ID（本地知识库风格参考）
}

// ContextBundle 注入到所有子任务的共享上下文（世界观/人物卡/前文/素材）
type ContextBundle struct {
	WorldSetting      string
	CharacterSetting  string
	HistoryContent    string
	MaterialText      string
	PreviousSummaries string
}

// AssembledText 将上下文拼装为可注入文本块。
// 顺序按「稳定性优先」排列：人物卡/世界观在同项目内基本不变，放在最前，
// 使 DeepSeek 等提供商的前缀缓存可命中（官方缓存命中/未命中差价约 40~120 倍，
// 缓存默认开启、仅前缀完全匹配命中）；变化频繁的【历史前文】【参考素材】置于后缀。
func (c ContextBundle) AssembledText() string {
	var b []byte
	appendSection := func(title, content string) {
		content = trimSpace(content)
		if content == "" {
			return
		}
		b = append(b, "【"...)
		b = append(b, title...)
		b = append(b, "】\n"...)
		b = append(b, content...)
		b = append(b, '\n')
	}
	appendSection("人物卡", c.CharacterSetting)
	appendSection("世界观设定", c.WorldSetting)
	appendSection("历史前文", c.HistoryContent)
	appendSection("参考素材", c.MaterialText)
	return string(b)
}

// HasContext 是否携带任何上下文
func (c ContextBundle) HasContext() bool {
	return trimSpace(c.WorldSetting) != "" ||
		trimSpace(c.CharacterSetting) != "" ||
		trimSpace(c.HistoryContent) != "" ||
		trimSpace(c.MaterialText) != ""
}

// EventType SSE 事件类型
type EventType string

const (
	EventPlan     EventType = "plan"     // 流水线计划
	EventStage    EventType = "stage"    // 阶段进度
	EventToken    EventType = "token"    // 正文增量分片
	EventWarning  EventType = "warning"  // 校验缺陷/降级提示
	EventError    EventType = "error"    // 错误
	EventDone     EventType = "done"     // 完成
	EventEstimate EventType = "estimate" // token 预估
)

// ProgressEvent 推送给前端的进度事件
type ProgressEvent struct {
	Type      EventType `json:"type"`
	Pipeline  string    `json:"pipeline,omitempty"`
	Stage     string    `json:"stage,omitempty"`      // 阶段描述
	Role      string    `json:"role,omitempty"`       // 当前角色
	Model     string    `json:"model,omitempty"`      // 当前模型
	Text      string    `json:"text,omitempty"`       // 增量文本/消息
	Iteration int       `json:"iteration,omitempty"`  // 迭代轮次
	Issues    []string  `json:"issues,omitempty"`     // 校验问题清单
	FinalText string    `json:"final_text,omitempty"` // 终稿（done 时）
	WordCount  int       `json:"word_count,omitempty"`  // 终稿字数（done 时）
	Tokens     int       `json:"tokens,omitempty"`      // 预估/实际 token
	Degraded  bool      `json:"degraded,omitempty"`   // 是否发生降级
	DurationMs int64     `json:"duration_ms,omitempty"` // 本 agent 调用耗时（毫秒），stage/done 事件携带
	Reset       bool                `json:"reset,omitempty"`        // true=清空已渲染文本（微调重写前）
	OutlineWords *OutlineWordEstimate `json:"outline_words,omitempty"` // 大纲字数校验结果
}

// OutlineWordEstimate 大纲字数校验（Thinker 产出后由调度中枢分析）
type OutlineWordEstimate struct {
	SuggestedTotal int     `json:"suggested_total"` // 大纲各节点汇总建议字数
	TargetWord     int     `json:"target_word"`     // 用户设定目标字数
	Mismatch       bool    `json:"mismatch"`        // 是否超出±30%阈值
	Ratio          float64 `json:"ratio"`           // suggested/target 比例
	Advice         string  `json:"advice"`          // 建议文案
}

// Usage 累计用量（调度过程汇总）
type Usage struct {
	PromptTokens     int
	CompletionTokens int
}

// Add 累加一次调用的用量
func (u *Usage) Add(a llm.Usage) {
	u.PromptTokens += a.PromptTokens
	u.CompletionTokens += a.CompletionTokens
}

// Total 总 token
func (u *Usage) Total() int { return u.PromptTokens + u.CompletionTokens }
