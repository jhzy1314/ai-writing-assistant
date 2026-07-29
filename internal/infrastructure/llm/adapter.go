package llm

import "context"

// ModelAdapter 模型适配层统一抽象接口。
// 新增厂商模型只需实现该接口（或复用 OpenAICompatibleAdapter），
// 上层调度引擎与角色 Agent 完全不感知具体厂商。
type ModelAdapter interface {
	// Name 模型名称（唯一标识）
	Name() string
	// Vendor 厂商标识（DeepSeek / Kimi / ...）
	Vendor() string
	// Endpoint 模型 API 地址
	Endpoint() string
	// Generate 非流式生成，返回正文与 token 用量
	Generate(ctx context.Context, systemPrompt, userPrompt string) (text string, usage Usage, err error)
	// Stream 流式生成，返回文本分片通道；通道关闭表示结束
	Stream(ctx context.Context, systemPrompt, userPrompt string) (<-chan StreamChunk, error)
}

// Usage 模型调用 token 用量
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

// Total 返回总 token
func (u Usage) Total() int { return u.PromptTokens + u.CompletionTokens }

// StreamChunk 流式输出单个分片
type StreamChunk struct {
	Text  string // 本次增量文本
	Usage *Usage // 非空表示携带用量（通常在末尾）
	Err   error  // 非 nil 表示出错
	Done  bool   // true 表示流结束
}

// EstimateTokens 粗略预估 token 数（中文为主场景：约 1.5 字符/token）
// 真实用量优先取模型返回的 Usage，此函数仅用于预估与兜底统计。
func EstimateTokens(text string) int {
	r := []rune(text)
	if len(r) == 0 {
		return 0
	}
	v := float64(len(r)) / 1.5
	if v < 1 {
		v = 1
	}
	// 向上取整
	return int(v + 0.9999)
}
