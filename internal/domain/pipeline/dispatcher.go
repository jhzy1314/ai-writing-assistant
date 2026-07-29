package pipeline

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ai-novel/studio/internal/domain/roles"
	"github.com/ai-novel/studio/internal/infrastructure/database"
	"github.com/ai-novel/studio/internal/infrastructure/llm"
	"github.com/ai-novel/studio/internal/infrastructure/quota"
)

// Dispatcher 调度中枢 Agent：
// 接收用户请求 → 识别任务类型 → 匹配流水线 → 拆分任务 → 有序调用子 Agent
// → 管控迭代循环 → 处理异常降级 → 汇总最终成品。
// 调度中枢禁止直接生成正文内容，只负责调度与流程控制。
type Dispatcher struct {
	registry *llm.Registry
	store    *database.Store
	limiter  *quota.Limiter
}

// NewDispatcher 构造调度中枢
func NewDispatcher(registry *llm.Registry, store *database.Store, limiter *quota.Limiter) *Dispatcher {
	return &Dispatcher{registry: registry, store: store, limiter: limiter}
}

// Run 执行创作请求，返回进度事件通道（SSE 处理方读取后即推送前端）。
// 通道关闭表示流程结束。ctx 取消可中断生成。
func (d *Dispatcher) Run(ctx context.Context, req GenerateRequest, ip string) <-chan ProgressEvent {
	out := make(chan ProgressEvent, 32)
	go func() {
		defer close(out)
		emit := func(e ProgressEvent) {
			select {
			case out <- e:
			case <-ctx.Done():
			}
		}
		defer func() {
			if r := recover(); r != nil {
				emit(ProgressEvent{Type: EventError, Text: "服务内部异常，请重试"})
			}
		}()

		// 0. 成本与限流校验
		guard, err := d.limiter.AllowRequest(ctx, ip)
		if err != nil {
			emit(ProgressEvent{Type: EventError, Text: err.Error()})
			return
		}
		defer guard.Release()

		// 0.5 如果提供了 model_config_id，根据 ID 解析模型名称
		if strings.TrimSpace(req.ModelConfigID) != "" && strings.TrimSpace(req.ModelName) == "" {
			m, mErr := d.store.GetModel(ctx, req.ModelConfigID)
			if mErr == nil && m != nil {
				req.ModelName = m.Name
			}
		}

		// 1. token 预估 + 长上下文预检
		ctxText := req.UserDemand + req.WorldSetting + req.CharacterSetting + req.HistoryContent + req.MaterialText + req.SelectedText
		estTokens := quota.EstimateRequestTokens(ctxText, req.TargetWord)
		emit(ProgressEvent{Type: EventEstimate, Tokens: estTokens})
		// 预检：单模型上下文窗口上限约65536 tokens（deepseek），超过则警告
		perReqLimit := d.store.GetConfigInt(ctx, "per_request_token_limit", 8000)
		if estTokens > perReqLimit {
			emit(ProgressEvent{Type: EventWarning, Text: fmt.Sprintf("上下文约 %d tokens，已超单请求上限 %d，已自动截断", estTokens, perReqLimit)})
		}

		// 2. 组装共享上下文
		bundle := d.buildContext(ctx, req)

		// 3. 解析流水线
		lightLimit := d.store.GetConfigInt(ctx, "light_input_char_limit", 500)
		pl := resolvePipeline(req, lightLimit)
		maxIter := d.store.GetConfigInt(ctx, "max_iterations", 3)

		// 4. 输出执行计划
		emit(ProgressEvent{Type: EventPlan, Pipeline: string(pl), Stage: pipelineTitle(pl)})

		// 5. 分派执行
		var finalText string
		var execErr error
		switch pl {
		case PipelineDraft:
			finalText, execErr = d.runDraft(ctx, req, bundle, emit)
		case PipelineManual:
			finalText, execErr = d.runManual(ctx, req, bundle, emit)
		case PipelineLight:
			finalText, execErr = d.runLight(ctx, req, bundle, lightLimit, emit)
		case PipelineStrict:
			finalText, execErr = d.runStrict(ctx, req, bundle, maxIter, emit)
		case PipelineArt:
			finalText, execErr = d.runArt(ctx, req, bundle, maxIter, emit)
		default:
			finalText, execErr = d.runStandard(ctx, req, bundle, maxIter, emit)
		}

		if execErr != nil {
			emit(ProgressEvent{Type: EventError, Text: execErr.Error()})
			return
		}

		// 6. 汇总终稿
		emit(ProgressEvent{Type: EventDone, Pipeline: string(pl), FinalText: finalText})
	}()
	return out
}

// friendlyErr 将底层错误转换为友好中文提示（禁止抛出原始 API 错误码）
func friendlyErr(role llm.Role, lastErr error) error {
	if lastErr == nil {
		return nil
	}
	return fmt.Errorf("「%s」角色的全部模型调用均失败，请稍后重试或在后台检查模型配置", roleLabel(role))
}

func roleLabel(role llm.Role) string {
	switch role {
	case llm.RoleThinker:
		return "规划师Thinker"
	case llm.RoleWorker:
		return "创作者Worker"
	case llm.RoleVerifier:
		return "校验官Verifier"
	case llm.RoleHelper:
		return "轻助手Helper"
	default:
		return string(role)
	}
}

func pipelineTitle(pl PipelineName) string {
	switch pl {
	case PipelineStandard:
		return "智能协同创作"
	case PipelineDraft:
		return "快速草稿（直出初稿）"
	case PipelineStrict:
		return "严谨创作模式"
	case PipelineArt:
		return "文艺创作模式"
	case PipelineLight:
		return "轻量辅助模式"
	case PipelineManual:
		return "手动直调模式"
	default:
		return string(pl)
	}
}

// callRole 非流式调用某角色（含备用模型降级 + 调用日志 + 用量记录 + 超时保护）
func (d *Dispatcher) callRole(ctx context.Context, role llm.Role, pl PipelineName, projectID, userPrompt string) (string, llm.Usage, string, bool, error) {
	adapters, err := d.registry.AdaptersForRole(ctx, role)
	if err != nil {
		return "", llm.Usage{}, "", false, fmt.Errorf("「%s」无可用模型：%w", roleLabel(role), err)
	}
	agent := roles.NewRoleAgent(role, string(pl))
	var lastErr error
	degraded := false
	// 每次调用独立超时（防止 API 挂死）
	timeout := 5 * time.Minute
	if role == llm.RoleVerifier || role == llm.RoleHelper {
		timeout = 3 * time.Minute
	}
	for _, ad := range adapters {
		if ctx.Err() != nil {
			return "", llm.Usage{}, "", degraded, ctx.Err()
		}
		roleCtx, cancel := context.WithTimeout(ctx, timeout)
		start := time.Now()
		text, usage, gErr := agent.Generate(roleCtx, ad, userPrompt)
		cancel()
		dur := time.Since(start)
		if gErr == nil {
			d.logCall(ctx, projectID, role, ad.Name(), usage, dur.Milliseconds(), "ok", "")
			_ = d.store.IncrUsage(ctx, ad.Name(), 1, usage.Total())
			return text, usage, ad.Name(), degraded, nil
		}
		d.logCall(ctx, projectID, role, ad.Name(), usage, dur.Milliseconds(), "error", gErr.Error())
		lastErr = gErr
		degraded = true
		if d.isRateLimit(gErr) { d.registry.Mark429(ad.Name()) }
	}
	return "", llm.Usage{}, "", degraded, friendlyErr(role, lastErr)
}

func (d *Dispatcher) isRateLimit(err error) bool {
	s := err.Error()
	return strings.Contains(s, "429") || strings.Contains(s, "rate limit") || strings.Contains(s, "RateLimit")
}

// callRoleStream 流式调用某角色（备用模型降级仅在流启动前生效 + 超时保护）
// textCB 接收每个增量分片（用于实时推送给前端与累积终稿）
func (d *Dispatcher) callRoleStream(ctx context.Context, role llm.Role, pl PipelineName, projectID, userPrompt string, textCB func(string)) (string, llm.Usage, string, bool, error) {
	adapters, err := d.registry.AdaptersForRole(ctx, role)
	if err != nil {
		return "", llm.Usage{}, "", false, fmt.Errorf("「%s」无可用模型：%w", roleLabel(role), err)
	}
	agent := roles.NewRoleAgent(role, string(pl))
	var lastErr error
	degraded := false
	// 流式超时较长（Worker 可能输出长篇内容）
	timeout := 15 * time.Minute
	for _, ad := range adapters {
		if ctx.Err() != nil {
			return "", llm.Usage{}, "", degraded, ctx.Err()
		}
		roleCtx, cancel := context.WithTimeout(ctx, timeout)
		start := time.Now()
		ch, sErr := agent.Stream(roleCtx, ad, userPrompt)
		if sErr != nil {
			cancel()
			d.logCall(ctx, projectID, role, ad.Name(), llm.Usage{}, 0, "error", sErr.Error())
			lastErr = sErr
			degraded = true
			if d.isRateLimit(sErr) { d.registry.Mark429(ad.Name()) }
			continue
		}
		// 流已启动，读取分片
		var buf []byte
		var gotUsage *llm.Usage
		streamErr := error(nil)
		for chunk := range ch {
			if chunk.Err != nil {
				streamErr = chunk.Err
				break
			}
			if chunk.Text != "" {
				buf = append(buf, chunk.Text...)
				if textCB != nil {
					textCB(chunk.Text)
				}
			}
			if chunk.Usage != nil {
				gotUsage = chunk.Usage
			}
		}
		dur := time.Since(start)
		text := string(buf)
		if streamErr != nil && len(buf) == 0 {
			cancel()
			// 无任何输出且出错 → 降级到下一备用
			d.logCall(ctx, projectID, role, ad.Name(), llm.Usage{}, dur.Milliseconds(), "error", streamErr.Error())
			lastErr = streamErr
			degraded = true
			if d.isRateLimit(streamErr) { d.registry.Mark429(ad.Name()) }
			continue
		}
		cancel()
		// 成功（或部分输出后中断，接受部分结果）
		var u llm.Usage
		if gotUsage != nil {
			u = *gotUsage
		}
		if u.CompletionTokens == 0 {
			u.CompletionTokens = llm.EstimateTokens(text)
		}
		if u.PromptTokens == 0 {
			u.PromptTokens = llm.EstimateTokens(agent.SystemPrompt + userPrompt)
		}
		status := "ok"
		if streamErr != nil {
			status = "partial"
			degraded = true // 中途断流接受部分结果，标记降级以便前端告警
		}
		d.logCall(ctx, projectID, role, ad.Name(), u, dur.Milliseconds(), status, errStr(streamErr))
		_ = d.store.IncrUsage(ctx, ad.Name(), 1, u.Total())
		return text, u, ad.Name(), degraded, nil
	}
	return "", llm.Usage{}, "", degraded, friendlyErr(role, lastErr)
}

// logCall 记录一次模型调用日志
func (d *Dispatcher) logCall(ctx context.Context, projectID string, role llm.Role, modelName string, usage llm.Usage, durMs int64, status, errMsg string) {
	_ = d.store.InsertLog(ctx, database.GenerationLog{
		ProjectID:        projectID,
		Role:             string(role),
		ModelName:        modelName,
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
		DurationMs:       int(durMs),
		Status:           status,
		ErrorMsg:         errMsg,
	})
}

func errStr(e error) string {
	if e == nil {
		return ""
	}
	return e.Error()
}
