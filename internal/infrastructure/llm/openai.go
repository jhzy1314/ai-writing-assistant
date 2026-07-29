package llm

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// OpenAICompatibleAdapter 基于 Eino OpenAI 协议的通用适配器。
// DeepSeek 与 Kimi（月之暗面）均兼容 OpenAI Chat Completions 协议，
// 仅 base_url / model / api_key 不同，故复用同一实现作为两个厂商示例。
type OpenAICompatibleAdapter struct {
	chatModel model.ChatModel
	name      string
	vendor    string
	endpoint  string
}

// NewOpenAICompatible 构造适配器。
//   - vendor: 厂商标识，如 "DeepSeek" / "Kimi"
//   - apiKey / baseURL / modelName 来自 models 表配置（由 config.yaml 管理，禁止硬编码）
func NewOpenAICompatible(ctx context.Context, vendor, apiKey, baseURL, modelName string) (*OpenAICompatibleAdapter, error) {
	cm, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		APIKey:  apiKey,
		BaseURL: baseURL,
		Model:   modelName,
	})
	if err != nil {
		return nil, fmt.Errorf("初始化 %s 模型 %s 失败: %w", vendor, modelName, err)
	}
	return &OpenAICompatibleAdapter{
		chatModel: cm,
		name:      modelName,
		vendor:    vendor,
		endpoint:  baseURL,
	}, nil
}

func (a *OpenAICompatibleAdapter) Name() string     { return a.name }
func (a *OpenAICompatibleAdapter) Vendor() string   { return a.vendor }
func (a *OpenAICompatibleAdapter) Endpoint() string { return a.endpoint }

// Generate 非流式生成
func (a *OpenAICompatibleAdapter) Generate(ctx context.Context, systemPrompt, userPrompt string) (string, Usage, error) {
	msgs := []*schema.Message{
		schema.SystemMessage(systemPrompt),
		schema.UserMessage(userPrompt),
	}
	resp, err := a.chatModel.Generate(ctx, msgs)
	if err != nil {
		return "", Usage{}, fmt.Errorf("%s 调用失败: %w", a.name, err)
	}
	if resp == nil || resp.Content == "" {
		return "", Usage{}, fmt.Errorf("%s 返回空响应", a.name)
	}

	usage := extractUsage(resp, systemPrompt, userPrompt, resp.Content)
	return resp.Content, usage, nil
}

// Stream 流式生成
func (a *OpenAICompatibleAdapter) Stream(ctx context.Context, systemPrompt, userPrompt string) (<-chan StreamChunk, error) {
	msgs := []*schema.Message{
		schema.SystemMessage(systemPrompt),
		schema.UserMessage(userPrompt),
	}
	sr, err := a.chatModel.Stream(ctx, msgs)
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
