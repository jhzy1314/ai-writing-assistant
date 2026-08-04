package api

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

// precheckRequest 第一层预检请求体
type precheckRequest struct {
	UserDemand       string `json:"user_demand"`
	TargetWord       int    `json:"target_word"`
	WorldSetting     string `json:"world_setting"`
	CharacterSetting string `json:"character_setting"`
	HistoryContent   string `json:"history_content"`
	ProjectID        string `json:"project_id"` // 可选：有项目时按项目历史章节字数统计预估（更贴合作者习惯，稳定不随机）
}

// PrecheckResult 需求-字数预检返回体
type PrecheckResult struct {
	Analysis       string `json:"analysis"`
	SceneCount     int    `json:"scene_count"`
	CharacterCount int    `json:"character_count"`
	RecommendedMin int    `json:"recommended_min"`
	RecommendedMax int    `json:"recommended_max"`
	Mismatch       bool   `json:"mismatch"`
	MismatchType   string `json:"mismatch_type"` // "too_low" | "too_high" | ""
	Suggestion     string `json:"suggestion"`
	Model          string `json:"model"`
}

// HandlePrecheck POST /api/precheck
// 第一层预检：调用 Helper 分析需求复杂度与目标字数匹配度
// 请求体：{ "user_demand","target_word","world_setting","character_setting","history_content" }
func (s *Server) HandlePrecheck(w http.ResponseWriter, r *http.Request) {
	var req precheckRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "请求体解析失败: "+err.Error())
		return
	}
	if strings.TrimSpace(req.UserDemand) == "" {
		writeError(w, http.StatusBadRequest, "user_demand 不能为空")
		return
	}

	// 项目历史字数统计预估（优先，不调 LLM）：读本项目已有章节的实际字数习惯，稳定且贴合作者
	if req.ProjectID != "" {
		if chs, err := s.store.ListChapters(r.Context(), req.ProjectID, ""); err == nil && len(chs) >= 3 {
			var sizes []int
			for _, c := range chs {
				if c.WordCount > 0 {
					sizes = append(sizes, c.WordCount)
				}
			}
			if len(sizes) >= 3 {
				recent := sizes
				if len(recent) > 5 {
					recent = sizes[len(sizes)-5:]
				}
				sum := 0
				for _, n := range recent {
					sum += n
				}
				avg := sum / len(recent)
				// 单一推荐值：近5章平均 × 需求系数（稳定、有依据、不甩大区间）
				lo := int(float64(avg)/100) * 100
				if lo < 100 {
					lo = 100
				}
				// 需求/大纲内容量修正：用户目标与需求规模不应被历史平均框死
				demand := req.UserDemand + "\n" + req.HistoryContent
				factor := 1.0
				if strings.Contains(demand, "大战") || strings.Contains(demand, "对决") || strings.Contains(demand, "高潮") ||
					strings.Contains(demand, "大场面") || strings.Contains(demand, "决战") || strings.Contains(demand, "转折") ||
					strings.Contains(demand, "冲突") || strings.Contains(demand, "比赛") || strings.Contains(demand, "冒险") {
					factor = 1.4 // 大场面/多事件章节：推荐值上浮
				} else if strings.Contains(demand, "日常") || strings.Contains(demand, "过渡") || strings.Contains(demand, "平淡") {
					factor = 0.8 // 日常过渡：推荐值下调
				}
				rec := int(float64(avg)*factor/100) * 100
				if rec < 100 {
					rec = 100
				}
				writeOK(w, PrecheckResult{
					Analysis:       fmt.Sprintf("本项目 %d 章平均 %d 字/章（近 %d 章），按本章需求调整后推荐", len(sizes), avg, len(recent)),
					SceneCount:     len(recent),
					RecommendedMin: rec,
					RecommendedMax: rec,
					Mismatch:       false,
					Model:          "project-stats+demand",
				})
				return
			}
		}
	}

	// 轻量校验：输入小于 30 字不触发预检（避免浪费 token）
	if cn := charLen(req.UserDemand); cn < 30 && req.TargetWord <= 2000 && charLen(req.HistoryContent) == 0 {
		writeOK(w, PrecheckResult{
			Analysis:       "需求简略，默认匹配，无需预检",
			RecommendedMin: req.TargetWord,
			RecommendedMax: req.TargetWord,
			Model:          "heuristic",
		})
		return
	}

	guard, err := s.limiter.AllowRequest(r.Context(), clientIP(r))
	if err != nil {
		writeError(w, http.StatusTooManyRequests, err.Error())
		return
	}
	defer guard.Release()

	ctx, cancel := context.WithTimeout(r.Context(), reqTimeout)
	defer cancel()

	prompt := buildPrecheckPrompt(req)
	text, modelName, err := s.callHelperTool(ctx, prompt, "", "")
	if err != nil {
		// Helper 调用失败，回退到纯启发式校验
		result := heuristicPrecheck(req)
		result.Model = "heuristic(fallback)"
		writeOK(w, result)
		return
	}

	result := parsePrecheck(text, req.TargetWord)
	result.Model = modelName
	writeOK(w, result)
}

func buildPrecheckPrompt(req precheckRequest) string {
	var b strings.Builder
	b.WriteString("【需求-字数匹配预检】分析以下创作需求与目标字数的匹配程度。\n\n")
	b.WriteString("┌─ 用户需求 ─────────────────\n")
	b.WriteString(req.UserDemand)
	b.WriteString("\n└─────────────────────────────\n\n")
	if req.WorldSetting != "" {
		b.WriteString("┌─ 世界观 ───────────────────\n")
		b.WriteString(req.WorldSetting)
		b.WriteString("\n└─────────────────────────────\n\n")
	}
	if req.CharacterSetting != "" {
		b.WriteString("┌─ 人物卡 ───────────────────\n")
		b.WriteString(req.CharacterSetting)
		b.WriteString("\n└─────────────────────────────\n\n")
	}
	if req.HistoryContent != "" {
		hist := req.HistoryContent
		if charLen(hist) > 800 {
			hist = string([]rune(hist)[:800])
		}
		b.WriteString("┌─ 前文（截取800字）─────────\n")
		b.WriteString(hist)
		b.WriteString("\n└─────────────────────────────\n\n")
	}
	b.WriteString(fmt.Sprintf("【设定目标字数】%d 字\n\n", req.TargetWord))
	b.WriteString("请按以下格式逐行输出（纯文本，不要多余说明）：\n")
	b.WriteString("场景数量：<数字>\n登场人物：<数字>\n推荐下限字数：<数字>\n推荐上限字数：<数字>\n匹配判断：合理/需求偏高需增加字数/需求偏低可缩减字数\n简短建议：<一句话>\n\n")
	b.WriteString("【字数估算方法（必须按下述规则计算，不要自由发挥，同一需求每次结果必须一致）】\n")
	b.WriteString("1. 先数需求与创作框架中明确列出的场景/事件节点个数 = 场景数量（重复的节点不重复计数）；\n")
	b.WriteString("2. 基准字数：日常过渡场景 200-300 字/个；对话场景 300-400 字/个；冲突/高潮场景 500-700 字/个；\n")
	b.WriteString("3. 推荐下限 = 各场景下限之和；推荐上限 = 各场景上限之和（按上述基准估算）；\n")
	b.WriteString("4. 若需求没有明确场景（只是一句话），则推荐下限 = 目标字数的 60%，推荐上限 = 目标字数的 120%（取整到百位）；\n")
	b.WriteString("5. 登场人物 = 需求与前文中明确提及且会出场的人物个数。\n")
	return b.String()
}


// mismatchTypeOf 根据目标字数与推荐区间判断不匹配类型
func mismatchTypeOf(target, lo, hi int) string {
	if target <= 0 {
		return ""
	}
	if target < lo {
		return "too_low"
	}
	if target > hi {
		return "too_high"
	}
	return ""
}
func parsePrecheck(text string, targetWord int) PrecheckResult {
	r := PrecheckResult{Analysis: text}
	// 解析结构化字段
	if m := regexp.MustCompile(`场景数量[：:]\s*(\d+)`).FindStringSubmatch(text); len(m) >= 2 {
		r.SceneCount, _ = strconv.Atoi(m[1])
	}
	if m := regexp.MustCompile(`登场人物[：:]\s*(\d+)`).FindStringSubmatch(text); len(m) >= 2 {
		r.CharacterCount, _ = strconv.Atoi(m[1])
	}
	if m := regexp.MustCompile(`推荐下限字数[：:]\s*(\d+)`).FindStringSubmatch(text); len(m) >= 2 {
		r.RecommendedMin, _ = strconv.Atoi(m[1])
	}
	if m := regexp.MustCompile(`推荐上限字数[：:]\s*(\d+)`).FindStringSubmatch(text); len(m) >= 2 {
		r.RecommendedMax, _ = strconv.Atoi(m[1])
	}
	if m := regexp.MustCompile(`匹配判断[：:]\s*(.+)`).FindStringSubmatch(text); len(m) >= 2 {
		judge := strings.TrimSpace(m[1])
		if strings.Contains(judge, "偏高") {
			r.Mismatch = true
			r.MismatchType = "too_low"
		} else if strings.Contains(judge, "偏低") {
			r.Mismatch = true
			r.MismatchType = "too_high"
		}
	}
	if m := regexp.MustCompile(`简短建议[：:]\s*(.+)`).FindStringSubmatch(text); len(m) >= 2 {
		r.Suggestion = strings.TrimSpace(m[1])
	}
	// 若 Helper 未给出数值，用启发式补充
	if r.RecommendedMax == 0 {
		hr := heuristicPrecheckRaw(r.SceneCount, r.CharacterCount, targetWord)
		if r.RecommendedMin == 0 {
			r.RecommendedMin = hr.RecommendedMin
		}
		r.RecommendedMax = hr.RecommendedMax
		r.Mismatch = hr.Mismatch
		r.MismatchType = hr.MismatchType
		if r.Suggestion == "" {
			r.Suggestion = hr.Suggestion
		}
		if r.SceneCount == 0 {
			r.SceneCount = hr.SceneCount
		}
		if r.CharacterCount == 0 {
			r.CharacterCount = hr.CharacterCount
		}
	}
	// 只要 target 不在推荐区间内，标记 mismatch
	if targetWord < r.RecommendedMin {
		r.Mismatch = true
		r.MismatchType = "too_low"
		if r.Suggestion == "" {
			r.Suggestion = fmt.Sprintf("预估需 %d-%d 字，设定目标 %d 字偏少，建议增加至约 %d 字",
				r.RecommendedMin, r.RecommendedMax, targetWord, (r.RecommendedMin+r.RecommendedMax)/2)
		}
	} else if targetWord > r.RecommendedMax {
		r.Mismatch = true
		r.MismatchType = "too_high"
		if r.Suggestion == "" {
			r.Suggestion = fmt.Sprintf("预估需 %d-%d 字，设定目标 %d 字偏高，建议缩减至约 %d 字",
				r.RecommendedMin, r.RecommendedMax, targetWord, r.RecommendedMax)
		}
	}
	return r
}

// heuristicPrecheckRaw 纯启发式估算（Helper 调用失败时的兜底）
func heuristicPrecheckRaw(sceneCount, charCount, targetWord int) PrecheckResult {
	r := PrecheckResult{}
	// 场景数×700 作为基础字数；人物数×300 作为附加
	if sceneCount <= 0 {
		sceneCount = 3
	}
	r.RecommendedMin = sceneCount * 600
	r.RecommendedMax = sceneCount * 1200
	if charCount > 0 {
		r.RecommendedMin += charCount * 200
		r.RecommendedMax += charCount * 400
	}
	// 下限不低于 500
	if r.RecommendedMin < 500 {
		r.RecommendedMin = 500
	}
	r.RecommendedMax += 500 // 余量
	r.SceneCount = sceneCount
	r.CharacterCount = charCount
	if targetWord < r.RecommendedMin {
		r.Mismatch = true
		r.MismatchType = "too_low"
	} else if targetWord > r.RecommendedMax {
		r.Mismatch = true
		r.MismatchType = "too_high"
	}
	return r
}

func heuristicPrecheck(req precheckRequest) PrecheckResult {
	// 简单关键词计数估算场景数
	sceneCount := 3
	userDemandLower := strings.ToLower(req.UserDemand)
	if strings.Contains(userDemandLower, "章节") || strings.Contains(userDemandLower, "章") {
		sceneCount = 5
	}
	if strings.Contains(userDemandLower, "片段") || strings.Contains(userDemandLower, "段落") {
		sceneCount = 2
	}
	if strings.Contains(req.UserDemand, "长篇") || strings.Contains(req.UserDemand, "连载") {
		sceneCount = 8
	}
	if strings.Contains(req.UserDemand, "短篇") || strings.Contains(req.UserDemand, "随笔") {
		sceneCount = 1
	}
	charCount := 0
	if req.CharacterSetting != "" {
		charCount = strings.Count(req.CharacterSetting, "【") + 1
		if charCount < 2 {
			charCount = 2
		}
	}
	r := heuristicPrecheckRaw(sceneCount, charCount, req.TargetWord)
	r.Analysis = fmt.Sprintf("（自动估算）场景数≈%d，人物≈%d，建议字数 %d-%d，目标 %d",
		sceneCount, charCount, r.RecommendedMin, r.RecommendedMax, req.TargetWord)
	return r
}

// charLen 字符数（rune count）
func charLen(s string) int { return len([]rune(s)) }

