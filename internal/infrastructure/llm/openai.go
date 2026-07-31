package llm

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// OpenAICompatibleAdapter 基于 Eino OpenAI 协议的通用适配器。
// DeepSeek 与 Kimi（月之暗面）均兼容 OpenAI Chat Completions 协议，
// 仅 base_url / model / api_key 不同，故复用同一实现作为两个厂商示例。
type OpenAICompatibleAdapter struct {
	chatModel      model.ChatModel
	chatModelNoThink model.ChatModel // thinking 关闭实例（懒创建）
	name           string
	vendor         string
	endpoint       string
	maxTokens      int
	apiKey         string
	baseURL        string
	supportsThinking bool // 该模型是否支持 thinking 参数（仅 DeepSeek v4-flash/pro）
	mu             sync.RWMutex
	thinkingEnabled bool // 当前请求是否开启思考（默认开=高质量）
}

// NewOpenAICompatible 构造适配器。
//   - vendor: 厂商标识，如 "DeepSeek" / "Kimi"
//   - apiKey / baseURL / modelName 来自 models 表配置（由 config.yaml 管理，禁止硬编码）
//   - maxTokens: 单次最大输出 token（<=0 时用默认 4096）
func NewOpenAICompatible(ctx context.Context, vendor, apiKey, baseURL, modelName string, maxTokens int) (*OpenAICompatibleAdapter, error) {
	cm, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		APIKey:  apiKey,
		BaseURL: baseURL,
		Model:   modelName,
	})
	if err != nil {
		return nil, fmt.Errorf("初始化 %s 模型 %s 失败: %w", vendor, modelName, err)
	}
	if maxTokens <= 0 {
		maxTokens = 4096
	}
	// DeepSeek 推理模型（deepseek-v4-flash/pro）支持 thinking 参数；官方 deepseek-chat 不支持
	supportsThinking := modelName == "deepseek-v4-flash" || modelName == "deepseek-v4-pro"
	return &OpenAICompatibleAdapter{
		chatModel:        cm,
		name:             modelName,
		vendor:           vendor,
		endpoint:         baseURL,
		maxTokens:        maxTokens,
		apiKey:           apiKey,
		baseURL:          baseURL,
		supportsThinking: supportsThinking,
		thinkingEnabled:  true, // 默认开启思考（高质量）
	}, nil
}

// SetThinking 设置当前请求是否启用深度思考（线程安全）
func (a *OpenAICompatibleAdapter) SetThinking(enabled bool) {
	a.mu.Lock()
	a.thinkingEnabled = enabled
	a.mu.Unlock()
}

// budget 返回本次调用的输出 token 预算：
// 推理模型（deepseek-v4-flash/pro）预算无上限——API 实测接受 max_tokens=64000（官方 reasoner 上限），
// 不再用 8192/16384 限制思考输出，防止推理占满预算导致正文截断/空响应。
// 其他模型（官方 chat/kimi/minimax 等）按各自配置（各厂商上限不同，不能统一放开）。
func (a *OpenAICompatibleAdapter) budget() int {
	if a.supportsThinking {
		return 64000
	}
	return a.maxTokens
}

// thinkingModel 返回当前应使用的 chatModel 实例
func (a *OpenAICompatibleAdapter) thinkingModel(ctx context.Context) (model.ChatModel, error) {
	a.mu.RLock()
	enabled := a.thinkingEnabled
	a.mu.RUnlock()
	if enabled || !a.supportsThinking {
		return a.chatModel, nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.chatModelNoThink != nil {
		return a.chatModelNoThink, nil
	}
	cm, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		APIKey:      a.apiKey,
		BaseURL:     a.baseURL,
		Model:       a.name,
		ExtraFields: map[string]any{"thinking": map[string]any{"type": "disabled"}},
	})
	if err != nil {
		return a.chatModel, err
	}
	a.chatModelNoThink = cm
	return cm, nil
}

func (a *OpenAICompatibleAdapter) Name() string     { return a.name }
func (a *OpenAICompatibleAdapter) Vendor() string   { return a.vendor }
func (a *OpenAICompatibleAdapter) Endpoint() string { return a.endpoint }

// Generate 非流式生成
func (a *OpenAICompatibleAdapter) Generate(ctx context.Context, systemPrompt, userPrompt string) (string, Usage, error) {
	cm, err := a.thinkingModel(ctx)
	if err != nil {
		return "", Usage{}, err
	}
	msgs := []*schema.Message{
		schema.SystemMessage(systemPrompt),
		schema.UserMessage(userPrompt),
	}
	// 推理模型（deepseek-v4-flash/pro 等）偶发“思考占满预算导致正文为空”，重试一次缓解
	for attempt := 1; attempt <= 2; attempt++ {
		resp, err := cm.Generate(ctx, msgs, model.WithMaxTokens(a.budget()))
		if err != nil {
			return "", Usage{}, fmt.Errorf("%s 调用失败: %w", a.name, err)
		}
		if resp != nil && resp.Content != "" {
			usage := extractUsage(resp, systemPrompt, userPrompt, resp.Content)
			return resp.Content, usage, nil
		}
		if attempt == 1 {
			time.Sleep(1500 * time.Millisecond)
		}
	}
	return "", Usage{}, fmt.Errorf("%s 返回空响应（已重试）", a.name)
}

// Stream 流式生成
func (a *OpenAICompatibleAdapter) Stream(ctx context.Context, systemPrompt, userPrompt string) (<-chan StreamChunk, error) {
	cm, err := a.thinkingModel(ctx)
	if err != nil {
		return nil, err
	}
	msgs := []*schema.Message{
		schema.SystemMessage(systemPrompt),
		schema.UserMessage(userPrompt),
	}
	sr, err := cm.Stream(ctx, msgs, model.WithMaxTokens(a.budget()))
	if err != nil {
		return nil, fmt.Errorf("%s 流式调用失败: %w", a.name, err)
	}

	out := make(chan StreamChunk, 16)
	go func() {
		defer close(out)
		defer sr.Close()
		for {
			msg, err := sr.Recv()
			if err != nil {
				// io.EOF / context.Canceled 表示流正常结束或客户端取消，不算错误
				if errors.Is(err, io.EOF) || errors.Is(err, context.Canceled) || err == io.EOF {
					return
				}
				out <- StreamChunk{Err: err}
				return
			}
			if msg == nil {
				continue
			}
			if msg.Content != "" {
				out <- StreamChunk{Text: msg.Content}
			}
			// 部分模型在末尾分片携带 usage
			if msg.ResponseMeta != nil && msg.ResponseMeta.Usage != nil {
				u := msg.ResponseMeta.Usage
				out <- StreamChunk{Usage: &Usage{PromptTokens: u.PromptTokens, CompletionTokens: u.CompletionTokens}}
			}
		}
	}()
	return out, nil
}

// extractUsage 优先取模型返回的真实用量，缺失时回退到估算
func extractUsage(resp *schema.Message, systemPrompt, userPrompt, output string) Usage {
	if resp.ResponseMeta != nil && resp.ResponseMeta.Usage != nil {
		u := resp.ResponseMeta.Usage
		pt, ct := u.PromptTokens, u.CompletionTokens
		if pt == 0 {
			pt = EstimateTokens(systemPrompt + userPrompt)
		}
		if ct == 0 {
			ct = EstimateTokens(output)
		}
		return Usage{PromptTokens: pt, CompletionTokens: ct}
	}
	return Usage{
		PromptTokens:     EstimateTokens(systemPrompt + userPrompt),
		CompletionTokens: EstimateTokens(output),
	}
}
