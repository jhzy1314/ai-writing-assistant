package api

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/ai-novel/studio/internal/domain/pipeline"
	"github.com/ai-novel/studio/internal/infrastructure/llm"
)

// PolishSystemPrompt 去 AI 味润色系统提示词（借鉴同类型产品 InkOS polisher agent）
const PolishSystemPrompt = `你是专业中文小说文字层润色编辑。

## 润色边界（硬约束）
你只改文字层——句式 / 段落 / 排版 / 用词 / 五感 / 对话自然度。你禁止增删情节、改变人设、调整主线。如果读到情节/结构问题，保留原意，不要替作者补情节，只需在末尾以一行 "[polisher-note] " 开头注明供人工参考。

## 6 条文笔类雷点（必须消灭）
- 无效描写：冗长的环境描写、与主线无关的对话塞满页面。把无效描写删到"一笔带过"。
- 文笔华丽过度：为辞藻堆辞藻，情感失真，形容词地毯轰炸。让文字服从情绪，不要炫技。
- 文笔欠佳：句意含混、指代不清、逻辑跳跃、语言干瘪。重写成通顺、有画面感的句子。
- 排版不规范：段落过长、格式不统一、对话无换行。统一为手机阅读友好格式。
- AI 味痕迹：转折词泛滥、"了"字堆砌、"仿佛/宛如/竟然"等情绪中介词、叙述者总结句（"这一刻他终于明白了……"）、分析报告式语言。替换成口语化表达或具体动作。
- 群像脸谱化：不写"众人齐声惊呼"，挑 1-2 个角色写具体反应。

## 文字层硬规约
- 段落：3-5 行/段（手机阅读），连续 7 行以上必须拆段，但不可把动作+反应拆碎到失去节奏。
- 句式：多样化，禁止连续 3 句以上同结构/同主语开头；长短交替。
- 动词 > 形容词：名词+动词驱动画面，一句话最多 1-2 个精准形容词。
- 五感代入：场景里至少 1-2 种感官细节（视/听/嗅/触/味），但不机械叠加，适度即可。
- 对话自然度：不同角色说话方式有辨识度；对话符合说话人当前身份、情绪、信息掌握；不写"……"式敷衍应答替代实质交锋。
- 情绪外化：把"他感到愤怒"改为"他捏碎了茶杯，滚烫的茶水流过指缝"。
- 删除无意义的叙述者结论和"显然/不禁/仿佛"这类 AI 标记词。
- 禁止破折号 "——"，禁止"不是……而是……"句式（存量出现一律改写）。

## 输出契约
直接返回润色后的完整正文——不要 JSON、不要章节标题行、不要任何解释或进度说明。如果发现必须人工处理的情节/结构问题，在正文末尾另起一行以 "[polisher-note] " 开头写明，每条一行。没有问题就不加。
保留原文绝大多数句子。只改真正有问题的句子，不要整段重写。修改后总长变化不得超过原文字数 ±15%。`

// HandleAITells POST /api/ai-tells —— 规则式 AI 味检测（纯规则，无 LLM 调用）
// 请求体：{ "content": "..." }
func (s *Server) HandleAITells(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Content string `json:"content"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(req.Content) == "" {
		writeError(w, http.StatusBadRequest, "content 不能为空")
		return
	}
	issues := pipeline.AnalyzeAITells(req.Content)
	writeOK(w, map[string]interface{}{
		"issues": issues,
		"count":  len(issues),
	})
}

// HandleAIPolish POST /api/ai-polish —— 去 AI 味文字层润色
// 请求体：{ "content": "...", "language": "zh|en" }
// 响应：{ "text": "润色后正文", "model": "..." }
func (s *Server) HandleAIPolish(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Content  string `json:"content"`
		Language string `json:"language"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(req.Content) == "" {
		writeError(w, http.StatusBadRequest, "content 不能为空")
		return
	}
	if req.Language == "" {
		req.Language = "zh"
	}
	if len([]rune(req.Content)) > 12000 {
		writeError(w, http.StatusBadRequest, "单次润色上限 12000 字，请分段润色")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Minute)
	defer cancel()

	adapters, err := s.registry.AdaptersForRole(ctx, llm.RoleWorker)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "无可用的润色模型: "+err.Error())
		return
	}

	var lastErr error
	for _, ad := range adapters {
		text, usage, gErr := ad.Generate(ctx, PolishSystemPrompt, buildPolishUserPrompt(req.Content, req.Language))
		_ = s.store.IncrUsage(ctx, ad.Name(), 1, usage.Total())
		if gErr == nil && strings.TrimSpace(text) != "" {
			writeOK(w, map[string]interface{}{
				"text":  stripPolishFence(text),
				"model": ad.Name(),
			})
			return
		}
		lastErr = gErr
	}
	writeError(w, http.StatusServiceUnavailable, "润色调用失败："+errText(lastErr))
}

func stripPolishFence(text string) string {
	t := strings.TrimSpace(text)
	if strings.HasPrefix(t, "```") {
		lines := strings.Split(t, "\n")
		if len(lines) > 2 {
			lines = lines[1:]
		}
		t = strings.TrimSpace(strings.Join(lines, "\n"))
		t = strings.TrimSuffix(strings.TrimSpace(t), "```")
	}
	return strings.TrimSpace(t)
}

func errText(err error) string {
	if err == nil {
		return "未知错误"
	}
	return err.Error()
}

// buildPolishUserPrompt 构造去 AI 味润色的用户提示词
func buildPolishUserPrompt(content, language string) string {
	if language == "en" {
		return "Please polish the following text to remove AI-generated feel. Return ONLY the polished full text — no JSON, no headers, no commentary. Preserve the vast majority of sentences; only rewrite sentences that truly need it; total length change must stay within ±15%.\n\n## Text Under Polish\n" + content
	}
	return "请对以下正文做「去AI味」文字层润色，只返回润色后的完整正文——不要 JSON、不要标题、不要任何解释。保留原文绝大多数句子，只改真正有问题的句子，不要整段重写；修改后总长变化不超过 ±15%。\n\n## 待润色正文\n" + content
}
