package api

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/ai-novel/studio/internal/infrastructure/llm"
)

// ============================================================
// candidates.go —— 多候选续写：一次生成 N 条不同剧情走向的续写，供用户挑选
// （借鉴彩云小梦/Sudowrite 的多走向续写；单次调用降低成本与等待时间）
// ============================================================

// HandleGenerateCandidates POST /api/generate/candidates
// 单次 AI 调用生成 count 个剧情走向不同的续写候选，按【候选N】标记切分返回。
// 相比并发多次调用：省限流配额、省等待（一次流式/非流式生成），候选差异靠 prompt 约束。
func (s *Server) HandleGenerateCandidates(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProjectID        string `json:"project_id"`
		ChapterID        string `json:"chapter_id"`
		UserDemand       string `json:"user_demand"`
		SelectedText     string `json:"selected_text"`
		HistoryContent   string `json:"history_content"`
		WorldSetting     string `json:"world_setting"`
		CharacterSetting string `json:"character_setting"`
		MaterialText     string `json:"material_text"`
		TargetWord       int    `json:"target_word"`
		Count            int    `json:"count"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "请求体解析失败: "+err.Error())
		return
	}
	if req.Count <= 0 || req.Count > 5 {
		req.Count = 3
	}
	if strings.TrimSpace(req.UserDemand) == "" && strings.TrimSpace(req.SelectedText) == "" &&
		strings.TrimSpace(req.HistoryContent) == "" {
		writeError(w, http.StatusBadRequest, "请提供续写需求、选中文字或前文（至少一项）")
		return
	}
	// 未显式传前文时，从章节兜底加载
	if strings.TrimSpace(req.HistoryContent) == "" && req.ChapterID != "" {
		if ch, err := s.store.GetChapter(r.Context(), req.ChapterID); err == nil && ch != nil {
			req.HistoryContent = ch.Content
		}
	}

	guard, err := s.limiter.AllowRequest(r.Context(), clientIP(r))
	if err != nil {
		writeError(w, http.StatusTooManyRequests, err.Error())
		return
	}
	defer guard.Release()

	ctx, cancel := context.WithTimeout(r.Context(), reqTimeout)
	defer cancel()

	// 每个候选目标字数：总目标均分，下限 100 字
	perWord := 250
	if req.TargetWord > 0 {
		perWord = req.TargetWord / req.Count
	}
	if perWord < 100 {
		perWord = 100
	}

	var b strings.Builder
	if strings.TrimSpace(req.HistoryContent) != "" {
		b.WriteString("【前文】\n" + truncate(req.HistoryContent, 6000) + "\n\n")
	}
	if strings.TrimSpace(req.CharacterSetting) != "" {
		b.WriteString("【人物卡】\n" + truncate(req.CharacterSetting, 2000) + "\n\n")
	}
	if strings.TrimSpace(req.WorldSetting) != "" {
		b.WriteString("【世界观设定】\n" + truncate(req.WorldSetting, 2000) + "\n\n")
	}
	if strings.TrimSpace(req.SelectedText) != "" {
		b.WriteString("【选中文字（从这之后续写）】\n" + truncate(req.SelectedText, 3000) + "\n\n")
	}
	if strings.TrimSpace(req.UserDemand) != "" {
		b.WriteString("【续写要求】\n" + req.UserDemand + "\n\n")
	}

	prompt := fmt.Sprintf(`【任务：多候选续写】
你是网文续写创作者。请基于给定前文，续写 %d 个剧情走向【各不相同】的候选片段，每个 %d 字左右：
- 候选 1：平稳推进（延续当前节奏与情绪）
- 候选 2：制造冲突或反转（引入意外事件/对话冲突）
- 候选 3：抛出新线索或伏笔（为后续章节埋钩子）
（若需求指定了方向，以需求为准。）
要求：人设、文风与前文一致；只输出正文，不要任何解释。
输出格式（每个候选用单独一行【候选N】标记开头）：
【候选1】
（候选1正文）
【候选2】
（候选2正文）
【候选3】
（候选3正文）

%s`, req.Count, perWord, b.String())

	text, model, err := s.callRoleTool(ctx, llm.RoleWorker, prompt, "", "candidates")
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "多候选生成失败: "+err.Error())
		return
	}
	candidates := splitCandidates(text, req.Count)
	if len(candidates) == 0 {
		// 兜底：未按标记输出时整段作为单候选
		if strings.TrimSpace(text) != "" {
			candidates = []string{strings.TrimSpace(text)}
		}
	}
	if len(candidates) == 0 {
		writeError(w, http.StatusUnprocessableEntity, "生成结果为空，请重试")
		return
	}
	writeOK(w, map[string]interface{}{"candidates": candidates, "count": len(candidates), "model": model})
}

// splitCandidates 按【候选N】标记切分 AI 输出的多候选文本
func splitCandidates(text string, count int) []string {
	re := regexp.MustCompile(`【候选\s*(\d+)】`)
	idx := re.FindAllStringSubmatchIndex(text, -1)
	if len(idx) == 0 {
		return nil
	}
	var out []string
	for i := range idx {
		start := idx[i][1]
		end := len(text)
		if i+1 < len(idx) {
			end = idx[i+1][0]
		}
		seg := strings.TrimSpace(text[start:end])
		if seg != "" {
			out = append(out, seg)
		}
	}
	if len(out) > count {
		out = out[:count]
	}
	return out
}
