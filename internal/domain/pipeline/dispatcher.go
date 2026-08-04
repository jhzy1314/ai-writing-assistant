package pipeline

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"
	"unicode"

	"github.com/ai-novel/studio/internal/domain/roles"
	"github.com/ai-novel/studio/internal/infrastructure/database"
	"github.com/ai-novel/studio/internal/infrastructure/llm"
	"github.com/ai-novel/studio/internal/infrastructure/quality"
	"github.com/ai-novel/studio/internal/infrastructure/quota"
	"github.com/ai-novel/studio/internal/infrastructure/rag"
	"github.com/ai-novel/studio/internal/infrastructure/search"
)

// Dispatcher 调度中枢 Agent：
// 接收用户请求 → 识别任务类型 → 匹配流水线 → 拆分任务 → 有序调用子 Agent
// → 管控迭代循环 → 处理异常降级 → 汇总最终成品。
// 调度中枢禁止直接生成正文内容，只负责调度与流程控制。
type Dispatcher struct {
	registry *llm.Registry
	store    *database.Store
	limiter  *quota.Limiter
	rag      *rag.Service
}

// NewDispatcher 构造调度中枢
func NewDispatcher(registry *llm.Registry, store *database.Store, limiter *quota.Limiter) *Dispatcher {
	return &Dispatcher{registry: registry, store: store, limiter: limiter, rag: rag.NewService(store)}
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

		// 1. token 预估（仅展示，不做截断——用户明确要求上下文无上限、读全文只求质量）
		ctxText := req.UserDemand + req.WorldSetting + req.CharacterSetting + req.HistoryContent + req.MaterialText + req.SelectedText
		estTokens := quota.EstimateRequestTokens(ctxText, req.TargetWord)
		emit(ProgressEvent{Type: EventEstimate, Tokens: estTokens})

		// 2. 组装共享上下文
		bundle := d.buildContext(ctx, req)

		// 2.5 联网搜索（用户开启时）：检索一次，注入到所有 Agent 提示词
		if req.WebSearch {
			emit(ProgressEvent{Type: EventStage, Stage: "🔎 联网搜索资料中…", Role: "web", Text: "正在检索相关背景资料"})
			req.WebInfo = d.webSearch(ctx, req)
			if req.WebInfo == "" {
				emit(ProgressEvent{Type: EventStage, Stage: "联网搜索未返回结果，继续创作", Role: "web", Text: "未检索到可用资料，按常规创作"})
			}
		}

		// 3. 解析流水线
		lightLimit := d.store.GetConfigInt(ctx, "light_input_char_limit", 500)
		pl := resolvePipeline(req, lightLimit)
		maxIter := d.store.GetConfigInt(ctx, "max_iterations", 3)
		if pl == PipelineStrict {
			maxIter = maxIter + 2 // 严谨模式多迭代 2 轮
		}

		// 4. 输出执行计划
		emit(ProgressEvent{Type: EventPlan, Pipeline: string(pl), Stage: pipelineTitle(pl)})

		// 5. 分派执行
		var finalText string
		var execErr error
		switch pl {
		case PipelineOrchestrated:
			finalText, execErr = d.runOrchestrated(ctx, req, bundle, maxIter, emit)
		case PipelineCollab:
			finalText, execErr = d.runCollab(ctx, req, bundle, maxIter, emit)
		case PipelineDraft:
			finalText, execErr = d.runDraft(ctx, req, bundle, emit)
		case PipelineManual:
			finalText, execErr = d.runManual(ctx, req, bundle, emit)
		case PipelineLight:
			finalText, execErr = d.runLight(ctx, req, bundle, lightLimit, emit)
		case PipelineFree:
			finalText, execErr = d.runFree(ctx, req, bundle, emit)
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

		// 6.3 章节摘要自动提炼（六段式 + 一次性事件标注，写入 synopsis，失败静默）
		// 摘要后续自动注入下一章生成（collectSynopses），形成滚动前情提要
		if finalText != "" {
			d.summarizeAndStore(ctx, req, finalText, emit)
		}

		// 6.4 Scrub 清洗：删除章节元信息残留行（"（第X章）""本章完""以下是修订后的正文"等）
		// 保证交付终稿为"印刷版"（对标 show-me-the-story stripChapterMetaProse）
		if finalText != "" {
			finalText = scrubChapterMeta(finalText)
		}

		// 6.4b AI 味闭环（2026-08-05 共识第 5/6 条）：规则检测"解释性旁白"→定位到段/句→
		// 问题句 ≥3 时交 Worker 局部修改（只改问题句，不整段重写——整篇自动润色会磨掉梗/吐槽，
		// 精准闭环更安全）。单段问题句 ≥5 时额外注明该段可整体重写。白名单：
		// 引号内对话、第一人称转述体叙述不检测。
		// 自由写作模式（free）跳过：无审校无重写，保留毛边感。
		if finalText != "" && pl != PipelineFree {
			sentIssues := DetectSentenceIssues(finalText)
			if len(sentIssues) >= 3 {
				emit(ProgressEvent{Type: EventStage, Stage: fmt.Sprintf("检测到 %d 处解释性旁白，交创作者局部修改", len(sentIssues)), Role: string(llm.RoleWorker)})
				var sb strings.Builder
				perPara := map[int]int{}
				for _, si := range sentIssues {
					sb.WriteString(fmt.Sprintf("· 第 %d 段：\"%s\"（%s，%s）\n", si.ParaIndex, si.Sentence, si.Category, si.FixHint))
					perPara[si.ParaIndex]++
				}
				for pi, n := range perPara {
					if n >= 5 {
						sb.WriteString(fmt.Sprintf("· 第 %d 段问题句达 %d 个，可整体重写该段；其余段落只改问题句。\n", pi, n))
					}
				}
				polished, _, _, _, _, pErr := d.callRole(ctx, llm.RoleWorker, PipelineStandard, req.ProjectID, buildReviseUserPrompt(req, bundle, sb.String(), finalText), req.RoleThinking)
				if pErr == nil && strings.TrimSpace(polished) != "" && len([]rune(polished)) > 100 {
					finalText = polished
					emit(ProgressEvent{Type: EventStage, Stage: "解释性旁白局部修改完成", Role: string(llm.RoleWorker)})
				}
			}
		}

		// 6.5 零成本 AI 味检测（确定性规则，不调模型；命中仅提示，不阻塞输出）
		// 把 Worker 去AI味规约/Verifier AI味清单代码化，生成完成即检查
		if a := quality.Analyze(finalText); len(a.Issues) > 0 {
			parts := make([]string, 0, len(a.Issues))
			for _, is := range a.Issues {
				parts = append(parts, is.Details)
			}
			emit(ProgressEvent{Type: EventWarning, Text: "AI味检测：" + strings.Join(parts, "；") + "（可在生成后手动润色）"})
		}

		// 6. 汇总终稿
		emit(ProgressEvent{Type: EventDone, Pipeline: string(pl), FinalText: finalText, WordCount: countWords(finalText)})
	}()
	return out
}

// webSearch 执行联网搜索并格式化为参考信息；失败/无结果返回空串（不阻塞创作）
func (d *Dispatcher) webSearch(ctx context.Context, req GenerateRequest) string {
	query := buildSearchQuery(req)
	if query == "" {
		return ""
	}
	sctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	results, err := search.WebSearch(sctx, query, 5)
	if err != nil || len(results) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("检索词：%s\n", query))
	for i, r := range results {
		b.WriteString(fmt.Sprintf("%d. %s\n%s\n%s\n", i+1, r.Title, r.URL, r.Snippet))
	}
	return b.String()
}

// buildSearchQuery 从创作需求中提炼搜索词（取前 90 字，去换行）
func buildSearchQuery(req GenerateRequest) string {
	q := strings.TrimSpace(req.UserDemand)
	if q == "" {
		q = strings.TrimSpace(req.SelectedText)
	}
	q = strings.NewReplacer("\n", " ", "\r", " ").Replace(q)
	r := []rune(q)
	if len(r) > 90 {
		q = string(r[:90])
	}
	return strings.TrimSpace(q)
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
	case PipelineCollab:
		return "多Agent协同闭环创作"
	case PipelineOrchestrated:
		return "手动指派Agent模型·完整流水线"
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

// thinkingEnabled 判断某角色是否开启深度思考：roleThinking 中显式指定则用指定值；
// 未指定时走推荐配置：仅规划师动脑（thinker 开），写作/审稿/轻活不动脑（实测审稿思考开销极大，不划算）
func thinkingEnabled(role llm.Role, roleThinking map[string]bool) bool {
	if roleThinking != nil {
		if v, ok := roleThinking[string(role)]; ok {
			return v
		}
	}
	return role == llm.RoleThinker
}

// callRole 非流式调用某角色（含备用模型降级 + 调用日志 + 用量记录 + 超时保护）
// 返回 (文本, 用量, 模型名, 是否降级, 耗时ms, 错误)
func (d *Dispatcher) callRole(ctx context.Context, role llm.Role, pl PipelineName, projectID, userPrompt string, roleThinking map[string]bool) (string, llm.Usage, string, bool, int64, error) {
	adapters, err := d.registry.AdaptersForRole(ctx, role)
	if err != nil {
		return "", llm.Usage{}, "", false, 0, fmt.Errorf("「%s」无可用模型：%w", roleLabel(role), err)
	}
	agent := roles.NewRoleAgent(role, string(pl))
	var lastErr error
	degraded := false
	// 每次调用独立超时（防止 API 挂死）
	timeout := 5 * time.Minute
	// 审稿开思考时推理链很长，放宽到 10 分钟避免误杀；其余保持 5 分钟
	if role == llm.RoleVerifier && thinkingEnabled(role, roleThinking) {
		timeout = 10 * time.Minute
	}
	totalStart := time.Now()
	for _, ad := range adapters {
		if ctx.Err() != nil {
			return "", llm.Usage{}, "", degraded, time.Since(totalStart).Milliseconds(), ctx.Err()
		}
		ad.SetThinking(thinkingEnabled(role, roleThinking))
		roleCtx, cancel := context.WithTimeout(ctx, timeout)
		start := time.Now()
		text, usage, gErr := agent.Generate(roleCtx, ad, userPrompt)
		cancel()
		dur := time.Since(start)
		if gErr == nil {
			d.logCall(ctx, projectID, role, ad.Name(), usage, dur.Milliseconds(), "ok", "")
			_ = d.store.IncrUsage(ctx, ad.Name(), 1, usage.Total())
			return text, usage, ad.Name(), degraded, time.Since(totalStart).Milliseconds(), nil
		}
		d.logCall(ctx, projectID, role, ad.Name(), usage, dur.Milliseconds(), "error", gErr.Error())
		lastErr = gErr
		degraded = true
		if d.isRateLimit(gErr) { d.registry.Mark429(ad.Name()) }
	}
	return "", llm.Usage{}, "", degraded, time.Since(totalStart).Milliseconds(), friendlyErr(role, lastErr)
}

func (d *Dispatcher) isRateLimit(err error) bool {
	s := err.Error()
	return strings.Contains(s, "429") || strings.Contains(s, "rate limit") || strings.Contains(s, "RateLimit")
}

// callRoleStream 流式调用某角色（备用模型降级仅在流启动前生效 + 超时保护）
// textCB 接收每个增量分片（用于实时推送给前端与累积终稿）
// 返回 (文本, 用量, 模型名, 是否降级, 耗时ms, 错误)
func (d *Dispatcher) callRoleStream(ctx context.Context, role llm.Role, pl PipelineName, projectID, userPrompt string, textCB func(string), roleThinking map[string]bool) (string, llm.Usage, string, bool, int64, error) {
	adapters, err := d.registry.AdaptersForRole(ctx, role)
	if err != nil {
		return "", llm.Usage{}, "", false, 0, fmt.Errorf("「%s」无可用模型：%w", roleLabel(role), err)
	}
	agent := roles.NewRoleAgent(role, string(pl))
	var lastErr error
	degraded := false
	// 流式超时较长（Worker 可能输出长篇内容）
	timeout := 15 * time.Minute
	totalStart := time.Now()
	for _, ad := range adapters {
		if ctx.Err() != nil {
			return "", llm.Usage{}, "", degraded, time.Since(totalStart).Milliseconds(), ctx.Err()
		}
		ad.SetThinking(thinkingEnabled(role, roleThinking))
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
		// 修复：流正常结束但没有任何正文输出（推理模型思考占满预算/偶发空响应）→ 视为失败并降级到备用模型
		if streamErr == nil && text == "" {
			cancel()
			d.logCall(ctx, projectID, role, ad.Name(), llm.Usage{}, dur.Milliseconds(), "error", ad.Name()+" 返回空响应")
			lastErr = fmt.Errorf("%s 返回空响应", ad.Name())
			degraded = true
			continue
		}
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
		return text, u, ad.Name(), degraded, time.Since(totalStart).Milliseconds(), nil
	}
	return "", llm.Usage{}, "", degraded, time.Since(totalStart).Milliseconds(), friendlyErr(role, lastErr)
}

// logCall 记录一次模型调用日志
func (d *Dispatcher) logCall(ctx context.Context, projectID string, role llm.Role, modelName string, usage llm.Usage, durMs int64, status, errMsg string) {
	_ = d.store.InsertLog(ctx, database.GenerationLog{
		ProjectID:        projectID,
		Role:             string(role),
		ModelName:        modelName,
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
		CacheHitTokens:   usage.CacheHitTokens,
		DurationMs:       int(durMs),
		Status:           status,
		ErrorMsg:         errMsg,
	})
	// 缓存命中观测（DeepSeek 前缀缓存：命中 token 越多成本越低）
	// 只在高命中率异常偏低时记录，避免每次成功调用刷日志
	if status == "ok" && usage.PromptTokens > 0 {
		hit := usage.CacheHitTokens
		if hit > 0 {
			rate := float64(hit) / float64(usage.PromptTokens) * 100
			if rate < 90 {
				log.Printf("[cache] 命中率偏低 role=%s model=%s hit=%d/%d (%.0f%%)", roleLabel(role), modelName, hit, usage.PromptTokens, rate)
			}
		}
	}
}

func errStr(e error) string {
	if e == nil {
		return ""
	}
	return e.Error()
}

// callRoleWithModel 使用指定模型名调用某角色（orchestrated 模式用，不走 registry 多级备用）
// 返回 (文本, 用量, 模型名, 是否降级, 耗时ms, 错误)
func (d *Dispatcher) callRoleWithModel(ctx context.Context, role llm.Role, pl PipelineName, projectID, modelName, userPrompt string, roleThinking map[string]bool) (string, llm.Usage, string, bool, int64, error) {
	ad, err := d.registry.AdapterByName(ctx, modelName)
	if err != nil {
		return "", llm.Usage{}, "", false, 0, fmt.Errorf("「%s」模型 %s 不可用：%w", roleLabel(role), modelName, err)
	}
	ad.SetThinking(thinkingEnabled(role, roleThinking))
	agent := roles.NewRoleAgent(role, string(pl))
	timeout := 5 * time.Minute
	if role == llm.RoleVerifier || role == llm.RoleHelper { timeout = 5 * time.Minute }
	roleCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	start := time.Now()
	text, usage, gErr := agent.Generate(roleCtx, ad, userPrompt)
	dur := time.Since(start)
	if gErr == nil {
		d.logCall(ctx, projectID, role, modelName, usage, dur.Milliseconds(), "ok", "")
		_ = d.store.IncrUsage(ctx, modelName, 1, usage.Total())
		return text, usage, modelName, false, dur.Milliseconds(), nil
	}
	d.logCall(ctx, projectID, role, modelName, usage, dur.Milliseconds(), "error", gErr.Error())
	return "", llm.Usage{}, modelName, true, dur.Milliseconds(), friendlyErr(role, gErr)
}

// callRoleStreamWithModel 使用指定模型名流式调用某角色
// 返回 (文本, 用量, 模型名, 是否降级, 耗时ms, 错误)
func (d *Dispatcher) callRoleStreamWithModel(ctx context.Context, role llm.Role, pl PipelineName, projectID, modelName, userPrompt string, textCB func(string), roleThinking map[string]bool) (string, llm.Usage, string, bool, int64, error) {
	ad, err := d.registry.AdapterByName(ctx, modelName)
	if err != nil {
		return "", llm.Usage{}, "", false, 0, fmt.Errorf("「%s」模型 %s 不可用：%w", roleLabel(role), modelName, err)
	}
	ad.SetThinking(thinkingEnabled(role, roleThinking))
	agent := roles.NewRoleAgent(role, string(pl))
	roleCtx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	start := time.Now()
	ch, sErr := agent.Stream(roleCtx, ad, userPrompt)
	if sErr != nil {
		d.logCall(ctx, projectID, role, modelName, llm.Usage{}, 0, "error", sErr.Error())
		return "", llm.Usage{}, modelName, true, 0, fmt.Errorf("流式启动失败: %w", sErr)
	}
	var buf []byte
	var gotUsage *llm.Usage
	for chunk := range ch {
		if chunk.Err != nil { break }
		if chunk.Text != "" {
			buf = append(buf, chunk.Text...)
			if textCB != nil { textCB(chunk.Text) }
		}
		if chunk.Usage != nil { gotUsage = chunk.Usage }
	}
	dur := time.Since(start)
	text := string(buf)
	var u llm.Usage
	if gotUsage != nil { u = *gotUsage }
	if u.CompletionTokens == 0 { u.CompletionTokens = llm.EstimateTokens(text) }
	if u.PromptTokens == 0 { u.PromptTokens = llm.EstimateTokens(agent.SystemPrompt + userPrompt) }
	d.logCall(ctx, projectID, role, modelName, u, dur.Milliseconds(), "ok", "")
	_ = d.store.IncrUsage(ctx, modelName, 1, u.Total())
	return text, u, modelName, false, dur.Milliseconds(), nil
}

func countWords(s string) int {
	count := 0
	inWord := false
	for _, r := range s {
		if unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r) {
			count++
			inWord = false
		} else if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if !inWord {
				count++
				inWord = true
			}
		} else {
			inWord = false
		}
	}
	return count
}
