package roles

import (
	"context"

	"github.com/ai-novel/studio/internal/infrastructure/llm"
)

// RolePrompt 返回角色的基础系统提示词
func RolePrompt(role llm.Role) string {
	switch role {
	case llm.RoleThinker:
		return ThinkerPrompt
	case llm.RoleWorker:
		return WorkerPrompt
	case llm.RoleVerifier:
		return VerifierPrompt
	case llm.RoleHelper:
		return HelperPrompt
	default:
		return ""
	}
}

// BuildSystemPrompt 组装某角色在某流水线下的完整系统提示词：
// 全局隐式前缀 + 角色基础提示词 + 流水线附加提示词
func BuildSystemPrompt(role llm.Role, pipeline string) string {
	return GlobalImplicitPrefix + "\n\n" + RolePrompt(role) + "\n\n" + PipelineSuffix(pipeline)
}

// RoleAgent 职能子 Agent：绑定角色与系统提示词，调用给定适配器执行一次模型调用。
// 适配器选择与备用模型降级由调度中枢（pipeline.Dispatcher）负责，角色 Agent 不感知。
type RoleAgent struct {
	Role         llm.Role
	SystemPrompt string
}

// NewRoleAgent 构造某角色在某流水线下的 Agent
func NewRoleAgent(role llm.Role, pipeline string) *RoleAgent {
	return &RoleAgent{
		Role:         role,
		SystemPrompt: BuildSystemPrompt(role, pipeline),
	}
}

// Generate 非流式调用单个适配器
func (a *RoleAgent) Generate(ctx context.Context, adapter llm.ModelAdapter, userPrompt string) (string, llm.Usage, error) {
	return adapter.Generate(ctx, a.SystemPrompt, userPrompt)
}

// Stream 流式调用单个适配器
func (a *RoleAgent) Stream(ctx context.Context, adapter llm.ModelAdapter, userPrompt string) (<-chan llm.StreamChunk, error) {
	return adapter.Stream(ctx, a.SystemPrompt, userPrompt)
}
