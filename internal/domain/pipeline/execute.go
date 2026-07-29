package pipeline

import (
	"context"
	"fmt"
	"strings"

	"github.com/ai-novel/studio/internal/domain/roles"
	"github.com/ai-novel/studio/internal/infrastructure/llm"
)

// emitToken 返回一个向前端推送增量分片的回调（模型信息由 stage 事件单独推送）
func emitToken(emit func(ProgressEvent), role string) func(string) {
	return func(text string) {
		emit(ProgressEvent{Type: EventToken, Text: text, Role: role})
	}
}

// runDraft 快速草稿模式：跳过 Thinker + Verifier，Worker 直接出初稿
// 适合快速产出、后期再深度优化的场景
func (d *Dispatcher) runDraft(ctx context.Context, req GenerateRequest, bundle ContextBundle, emit func(ProgressEvent)) (string, error) {
	emit(ProgressEvent{Type: EventStage, Stage: "快速草稿 — Worker 直接撰写中", Role: string(llm.RoleWorker)})
	userPrompt := fmt.Sprintf("【快速草稿模式】跳过规划与校验，直接创作。\n字数要求：约%d字。\n创作需求：%s\n\n请直接撰写正文：", req.TargetWord, req.UserDemand)
	if req.SelectedText != "" {
		userPrompt = fmt.Sprintf("【快速草稿模式】基于以下文字续写，直接创作。\n字数要求：约%d字。\n选中文字：%s\n\n请直接撰写正文：", req.TargetWord, req.SelectedText)
	}
	text, _, _, wDegraded, err := d.callRoleStream(ctx, llm.RoleWorker, PipelineDraft, req.ProjectID, userPrompt, emitToken(emit, string(llm.RoleWorker)))
	if err != nil {
		return "", err
	}
	if wDegraded {
		emit(ProgressEvent{Type: EventWarning, Text: "创作者主模型异常，已降级到备用模型", Degraded: true})
	}
	return text, nil
}

// runManual 手动模式：跳过流水线，直接调用用户指定的单个模型
func (d *Dispatcher) runManual(ctx context.Context, req GenerateRequest, bundle ContextBundle, emit func(ProgressEvent)) (string, error) {
	if strings.TrimSpace(req.ModelName) == "" {
		return "", fmt.Errorf("手动模式需指定 model_name")
	}
	ad, err := d.registry.AdapterByName(ctx, req.ModelName)
	if err != nil {
		return "", fmt.Errorf("指定的模型 %s 不可用：%w", req.ModelName, err)
	}
	emit(ProgressEvent{Type: EventStage, Stage: "手动调用模型 " + req.ModelName, Role: "manual", Model: req.ModelName})

	system := roles.GlobalImplicitPrefix + "\n\n你是由用户直接指定的创作模型，请根据需求生成内容。"
	userPrompt := buildManualUserPrompt(req, bundle)
	text, _, err := d.streamDirect(ctx, ad, system, userPrompt, emit, "manual", req.ModelName, req.ProjectID)
	return text, err
}

// runLight 轻量化快速模式：直接调用 Helper，不启动多轮串联流水线
func (d *Dispatcher) runLight(ctx context.Context, req GenerateRequest, bundle ContextBundle, lightLimit int, emit func(ProgressEvent)) (string, error) {
	emit(ProgressEvent{Type: EventStage, Stage: "轻助手处理中", Role: string(llm.RoleHelper)})
	userPrompt := buildHelperUserPrompt(req, bundle, lightLimit)
	text, _, _, degraded, err := d.callRoleStream(ctx, llm.RoleHelper, PipelineLight, req.ProjectID, userPrompt, emitToken(emit, string(llm.RoleHelper)))
	if degraded {
		emit(ProgressEvent{Type: EventWarning, Text: "主模型异常，已自动降级到备用模型", Degraded: true})
	}
	return text, err
}

// runStandard 标准通用创作：Thinker大纲 → Worker撰写 → Verifier校验 → 微调迭代
func (d *Dispatcher) runStandard(ctx context.Context, req GenerateRequest, bundle ContextBundle, maxIter int, emit func(ProgressEvent)) (string, error) {
	// 1. 构思分步搭建大纲
	emit(ProgressEvent{Type: EventStage, Stage: "正在为你的故事搭建框架…", Role: string(llm.RoleThinker), Iteration: 1})
	outline, _, tModel, tDegraded, err := d.callRole(ctx, llm.RoleThinker, PipelineStandard, req.ProjectID, buildThinkerUserPrompt(req, bundle, PipelineStandard))
	if err != nil {
		return "", err
	}
	emit(ProgressEvent{Type: EventStage, Stage: "框架搭建完成", Role: string(llm.RoleThinker), Model: tModel, Text: outline})
	if tDegraded {
		emit(ProgressEvent{Type: EventWarning, Text: "规划师主模型异常，已降级到备用模型", Degraded: true})
	}

	// 第二层校验：大纲建议字数 vs 目标字数（±30% 阈值）
	if oe := CheckOutlineWordCount(outline, req.TargetWord, req.SkipWordCheck); oe != nil {
		emit(ProgressEvent{Type: EventWarning, Text: oe.Advice, OutlineWords: oe})
	}

	// 2. 创作者撰写正文（支持分段）
	finalText, err := d.workerWrite(ctx, req, bundle, outline, PipelineStandard, emit)
	if err != nil {
		return "", err
	}

	// 3. 校验官审查 + 微调迭代
	finalText, err = d.verifyAndRevise(ctx, req, bundle, finalText, PipelineStandard, maxIter, emit)
	if err != nil {
		return "", err
	}
	return finalText, nil
}

// runStrict 严谨模式：Thinker独立初稿 → Worker轻度润色 → Verifier高标准校验
func (d *Dispatcher) runStrict(ctx context.Context, req GenerateRequest, bundle ContextBundle, maxIter int, emit func(ProgressEvent)) (string, error) {
	emit(ProgressEvent{Type: EventStage, Stage: "严谨模式 — 分析需求，搭建初稿框架…", Role: string(llm.RoleThinker)})
	outline, _, tModel, tDegraded, err := d.callRole(ctx, llm.RoleThinker, PipelineStrict, req.ProjectID, buildThinkerUserPrompt(req, bundle, PipelineStrict))
	if err != nil {
		return "", err
	}
	emit(ProgressEvent{Type: EventStage, Stage: "初稿框架完成", Role: string(llm.RoleThinker), Model: tModel, Text: outline})
	if tDegraded {
		emit(ProgressEvent{Type: EventWarning, Text: "规划师主模型异常，已降级到备用模型", Degraded: true})
	}

	// 第二层校验
	if oe := CheckOutlineWordCount(outline, req.TargetWord, req.SkipWordCheck); oe != nil {
		emit(ProgressEvent{Type: EventWarning, Text: oe.Advice, OutlineWords: oe})
	}

	// Worker 仅轻度润色（严禁改动逻辑/框架/核心观点）
	emit(ProgressEvent{Type: EventStage, Stage: "创作者 Worker 正在轻度润色", Role: string(llm.RoleWorker)})
	finalText, err := d.workerWrite(ctx, req, bundle, outline, PipelineStrict, emit)
	if err != nil {
		return "", err
	}

	finalText, err = d.verifyAndRevise(ctx, req, bundle, finalText, PipelineStrict, maxIter, emit)
	return finalText, err
}

// runArt 文艺创作模式：Thinker极简框架 → Worker高度自由创作 → Verifier宽松审查
func (d *Dispatcher) runArt(ctx context.Context, req GenerateRequest, bundle ContextBundle, maxIter int, emit func(ProgressEvent)) (string, error) {
	emit(ProgressEvent{Type: EventStage, Stage: "文艺模式 — 构思极简框架与关键剧情…", Role: string(llm.RoleThinker)})
	outline, _, tModel, tDegraded, err := d.callRole(ctx, llm.RoleThinker, PipelineArt, req.ProjectID, buildThinkerUserPrompt(req, bundle, PipelineArt))
	if err != nil {
		return "", err
	}
	emit(ProgressEvent{Type: EventStage, Stage: "框架构思完成", Role: string(llm.RoleThinker), Model: tModel, Text: outline})
	if tDegraded {
		emit(ProgressEvent{Type: EventWarning, Text: "规划师主模型异常，已降级到备用模型", Degraded: true})
	}

	// 第二层校验
	if oe := CheckOutlineWordCount(outline, req.TargetWord, req.SkipWordCheck); oe != nil {
		emit(ProgressEvent{Type: EventWarning, Text: oe.Advice, OutlineWords: oe})
	}

	emit(ProgressEvent{Type: EventStage, Stage: "创作者 Worker 高度自由创作中", Role: string(llm.RoleWorker)})
	finalText, err := d.workerWrite(ctx, req, bundle, outline, PipelineArt, emit)
	if err != nil {
		return "", err
	}

	finalText, err = d.verifyAndRevise(ctx, req, bundle, finalText, PipelineArt, maxIter, emit)
	return finalText, err
}

// workerWrite 创作者撰写正文（目标>4000字自动分段）
func (d *Dispatcher) workerWrite(ctx context.Context, req GenerateRequest, bundle ContextBundle, outline string, pl PipelineName, emit func(ProgressEvent)) (string, error) {
	if needsSegmentation(req) {
		n := numSegments(req)
		var full strings.Builder
		for i := 1; i <= n; i++ {
			if ctx.Err() != nil {
				return full.String(), ctx.Err()
			}
			emit(ProgressEvent{Type: EventStage, Stage: fmt.Sprintf("创作者 Worker 撰写第 %d/%d 段", i, n), Role: string(llm.RoleWorker), Iteration: i})
			seg, _, _, wDegraded, err := d.callRoleStream(ctx, llm.RoleWorker, pl, req.ProjectID,
				buildWorkerUserPrompt(req, bundle, outline, i, n), emitToken(emit, string(llm.RoleWorker)))
			if err != nil {
				return full.String(), err
			}
			if wDegraded {
				emit(ProgressEvent{Type: EventWarning, Text: fmt.Sprintf("第%d段主模型异常，已降级到备用模型", i), Degraded: true})
			}
			full.WriteString(seg)
			full.WriteString("\n")
		}
		return full.String(), nil
	}

	emit(ProgressEvent{Type: EventStage, Stage: "创作者 Worker 撰写正文中", Role: string(llm.RoleWorker)})
	text, _, _, wDegraded, err := d.callRoleStream(ctx, llm.RoleWorker, pl, req.ProjectID,
		buildWorkerUserPrompt(req, bundle, outline, 0, 0), emitToken(emit, string(llm.RoleWorker)))
	if wDegraded {
		emit(ProgressEvent{Type: EventWarning, Text: "创作者主模型异常，已降级到备用模型", Degraded: true})
	}
	return text, err
}

// verifyAndRevise 校验官审查 + 微调迭代循环（最大 maxIter 轮）
func (d *Dispatcher) verifyAndRevise(ctx context.Context, req GenerateRequest, bundle ContextBundle, content string, pl PipelineName, maxIter int, emit func(ProgressEvent)) (string, error) {
	current := content
	for iter := 1; iter <= maxIter; iter++ {
		if ctx.Err() != nil {
			return current, ctx.Err()
		}
		emit(ProgressEvent{Type: EventStage, Stage: fmt.Sprintf("校验官 Verifier 第 %d 轮审查", iter), Role: string(llm.RoleVerifier), Iteration: iter})
		review, _, vModel, vDegraded, err := d.callRole(ctx, llm.RoleVerifier, pl, req.ProjectID,
			buildVerifierUserPrompt(req, bundle, truncateForReview(current), pl))
		if err != nil {
			return current, err
		}
		if vDegraded {
			emit(ProgressEvent{Type: EventWarning, Text: "校验官主模型异常，已降级到备用模型", Degraded: true})
		}
		_ = vModel

		if reviewPassed(review) {
			emit(ProgressEvent{Type: EventStage, Stage: "校验通过", Role: string(llm.RoleVerifier), Iteration: iter})
			return current, nil
		}

		// 存在缺陷 → 推送问题清单
		issues := extractIssues(review)
		emit(ProgressEvent{Type: EventWarning, Stage: fmt.Sprintf("发现缺陷，回传创作者微调（第 %d/%d 轮）", iter, maxIter), Role: string(llm.RoleVerifier), Iteration: iter, Issues: issues, Degraded: false})

		// 微调前通知前端清空已渲染文本（重写全文）
		emit(ProgressEvent{Type: EventToken, Text: "", Role: string(llm.RoleWorker), Reset: true})
		emit(ProgressEvent{Type: EventStage, Stage: "创作者 Worker 根据修改意见微调中", Role: string(llm.RoleWorker), Iteration: iter})

		rev, _, _, rDegraded, err := d.callRoleStream(ctx, llm.RoleWorker, pl, req.ProjectID,
			buildReviseUserPrompt(req, bundle, review, current), emitToken(emit, string(llm.RoleWorker)))
		if err != nil {
			return current, err
		}
		if rDegraded {
			emit(ProgressEvent{Type: EventWarning, Text: "微调主模型异常，已降级到备用模型", Degraded: true})
		}
		current = rev
	}
	// 达到最大轮次仍有缺陷：终止循环并标注现存问题
	emit(ProgressEvent{Type: EventWarning, Stage: fmt.Sprintf("已达最大迭代轮次(%d)，交付当前稿件并标注现存问题", maxIter), Issues: []string{"校验未完全通过，可能仍存在次要缺陷，请人工复核"}})
	return current, nil
}

// streamDirect 直接用指定适配器流式生成（手动模式使用，带调用日志）
func (d *Dispatcher) streamDirect(ctx context.Context, ad llm.ModelAdapter, systemPrompt, userPrompt string, emit func(ProgressEvent), role, model, projectID string) (string, llm.Usage, error) {
	ch, err := ad.Stream(ctx, systemPrompt, userPrompt)
	if err != nil {
		d.logCall(ctx, projectID, llm.Role(role), model, llm.Usage{}, 0, "error", err.Error())
		return "", llm.Usage{}, err
	}
	var buf []byte
	var gotUsage *llm.Usage
	var streamErr error
	for chunk := range ch {
		if chunk.Err != nil {
			streamErr = chunk.Err
			break
		}
		if chunk.Text != "" {
			buf = append(buf, chunk.Text...)
			emit(ProgressEvent{Type: EventToken, Text: chunk.Text, Role: role, Model: model})
		}
		if chunk.Usage != nil {
			gotUsage = chunk.Usage
		}
	}
	text := string(buf)
	var u llm.Usage
	if gotUsage != nil {
		u = *gotUsage
	}
	if u.CompletionTokens == 0 {
		u.CompletionTokens = llm.EstimateTokens(text)
	}
	if u.PromptTokens == 0 {
		u.PromptTokens = llm.EstimateTokens(systemPrompt + userPrompt)
	}
	status := "ok"
	if streamErr != nil {
		status = "partial"
	}
	d.logCall(ctx, projectID, llm.Role(role), model, u, 0, status, errStr(streamErr))
	_ = d.store.IncrUsage(ctx, model, 1, u.Total())
	if streamErr != nil && len(buf) == 0 {
		return "", u, streamErr
	}
	return text, u, nil
}

// reviewPassed 判断校验官是否通过
func reviewPassed(review string) bool {
	r := strings.TrimSpace(review)
	if r == "" {
		return true
	}
	return strings.HasPrefix(r, "【校验通过】") || strings.Contains(r, "校验通过")
}

// extractIssues 从校验意见中提取问题清单
func extractIssues(review string) []string {
	lines := strings.Split(strings.TrimSpace(review), "\n")
	out := []string{}
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l == "" || strings.Contains(l, "校验通过") {
			continue
		}
		out = append(out, l)
		if len(out) >= 12 {
			break
		}
	}
	if len(out) == 0 {
		out = append(out, review)
	}
	return out
}

// truncateForReview 超长文本截断供校验官审查（规避上下文溢出）
func truncateForReview(s string) string {
	r := []rune(s)
	const max = 8000
	if len(r) > max {
		return string(r[:max]) + "\n……（后文略，仅审查前文连贯性与设定一致性）"
	}
	return s
}
