package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/ai-novel/studio/internal/infrastructure/database"
)

// HandleProjectTools POST /api/project-tools —— 项目管理类"粗活"统一入口（纯数据/规则，无正文生成）。
// 请求体：{ "tool": "material_pack|char_absence|motif_track|hook_check|timeline|stats_calendar", "project_id": "...", "params": {} }
// 响应：  { "result": {...} }
func (s *Server) HandleProjectTools(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, "读取请求失败: "+err.Error())
		return
	}
	var req struct {
		Tool      string                 `json:"tool"`
		ProjectID string                 `json:"project_id"`
		Params    map[string]interface{} `json:"params"`
	}
	if err := json.Unmarshal(body, &req); err != nil || strings.TrimSpace(req.Tool) == "" {
		writeError(w, http.StatusBadRequest, "请求体无效")
		return
	}
	if req.ProjectID == "" {
		writeError(w, http.StatusBadRequest, "缺少 project_id")
		return
	}
	ctx := r.Context()
	switch req.Tool {
	case "material_pack":
		s.projectToolMaterialPack(ctx, w, req.ProjectID, req.Params)
	case "char_absence":
		s.projectToolCharAbsence(ctx, w, req.ProjectID)
	case "motif_track":
		s.projectToolMotifTrack(ctx, w, req.ProjectID, req.Params)
	case "hook_check":
		s.projectToolHookCheck(ctx, w, req.ProjectID)
	case "timeline":
		s.projectToolTimeline(ctx, w, req.ProjectID)
	case "stats_calendar":
		s.projectToolStatsCalendar(ctx, w, req.ProjectID)
	default:
		writeError(w, http.StatusBadRequest, "未知 tool: "+req.Tool)
	}
}

// ---------- 1. 一键取料包：把 人物卡+前情+参考章节+伏笔+梗 拼成一段文本，直接复制给网页 AI ----------
func (s *Server) projectToolMaterialPack(ctx context.Context, w http.ResponseWriter, projectID string, params map[string]interface{}) {
	var b strings.Builder
	// 人物卡（全部）
	if chars, err := s.store.ListCharacters(ctx, projectID); err == nil && len(chars) > 0 {
		b.WriteString("【人物卡】\n")
		for _, c := range chars {
			if strings.TrimSpace(c.Name) == "" {
				continue
			}
			b.WriteString("- " + c.Name + "：" + strings.TrimSpace(c.Description) + "\n")
		}
		b.WriteString("\n")
	}
	// 世界观
	if ws, err := s.store.ListWorldSettings(ctx, projectID); err == nil && len(ws) > 0 {
		b.WriteString("【世界观】\n")
		for _, x := range ws {
			if strings.TrimSpace(x.Content) != "" {
				b.WriteString(strings.TrimSpace(x.Content) + "\n")
			}
		}
		b.WriteString("\n")
	}
	// 最近前情：上一章尾部 + 全部章节摘要
	chs, _ := s.store.ListChapters(ctx, projectID, "")
	if len(chs) > 0 {
		last := chs[len(chs)-1]
		if strings.TrimSpace(last.Content) != "" {
			b.WriteString("【上一章结尾】\n")
			r := []rune(strings.TrimSpace(last.Content))
			if len(r) > 1000 {
				b.WriteString(string(r[len(r)-1000:]) + "\n")
			} else {
				b.WriteString(string(r) + "\n")
			}
			b.WriteString("\n")
		}
		// 章节摘要（synopsis）
		var sum strings.Builder
		for i := range chs {
			if strings.TrimSpace(chs[i].Synopsis) != "" {
				sum.WriteString("第" + itoa(chs[i].SortOrder) + "章：" + strings.TrimSpace(chs[i].Synopsis) + "\n")
			}
		}
		if sum.Len() > 0 {
			b.WriteString("【各章摘要】\n" + sum.String() + "\n")
		}
	}
	// 未回收伏笔
	if fs, err := s.store.ListForeshadows(ctx, projectID); err == nil && len(fs) > 0 {
		b.WriteString("【未回收伏笔】\n")
		for _, f := range fs {
			if f.Status == database.ForeshadowPending && strings.TrimSpace(f.Title) != "" {
				b.WriteString("- " + f.Title + "：" + strings.TrimSpace(f.Description) + "\n")
			}
		}
		b.WriteString("\n")
	}
	// 用户指定文风参考章节
	if raw, ok := params["chapter_ids"]; ok {
		if ids, ok2 := raw.([]interface{}); ok2 && len(ids) > 0 {
			b.WriteString("【文风参考（照着这个语气写）】\n")
			for _, idv := range ids {
				id := fmt.Sprint(idv)
				if ch, err := s.store.GetChapter(ctx, id); err == nil && ch != nil && strings.TrimSpace(ch.Content) != "" {
					seg := truncateRunes2(strings.TrimSpace(ch.Content), 900)
					b.WriteString("（" + ch.Title + "节选）\n" + seg + "\n\n")
				}
			}
		}
	}
	writeOK(w, map[string]interface{}{"result": strings.TrimSpace(b.String())})
}

// ---------- 2. 人物出场统计：谁多少章没出场了 ----------
func (s *Server) projectToolCharAbsence(ctx context.Context, w http.ResponseWriter, projectID string) {
	chars, err := s.store.ListCharacters(ctx, projectID)
	if err != nil || len(chars) == 0 {
		writeOK(w, map[string]interface{}{"result": "还没有人物卡"})
		return
	}
	chs, _ := s.store.ListChapters(ctx, projectID, "")
	total := 0
	appear := make(map[string]int) // 人名 -> 出现章节数
	for i := range chs {
		if strings.TrimSpace(chs[i].Content) == "" {
			continue
		}
		total++
		for _, c := range chars {
			if strings.TrimSpace(c.Name) != "" && strings.Contains(chs[i].Content, c.Name) {
				appear[c.Name]++
			}
		}
	}
	type row struct {
		Name    string `json:"name"`
		Count   int    `json:"count"`
		Total   int    `json:"total"`
		Absent  int    `json:"absent"` // 连续未出场章节数（从最后一章往前数）
	}
	rows := make([]row, 0, len(chars))
	for _, c := range chars {
		if strings.TrimSpace(c.Name) == "" {
			continue
		}
		absent := 0
		for i := len(chs) - 1; i >= 0; i-- {
			if strings.TrimSpace(chs[i].Content) == "" {
				continue
			}
			if strings.Contains(chs[i].Content, c.Name) {
				break
			}
			absent++
		}
		rows = append(rows, row{Name: c.Name, Count: appear[c.Name], Total: total, Absent: absent})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Absent > rows[j].Absent })
	writeOK(w, map[string]interface{}{"result": rows, "total_chapters": total})
}

// ---------- 3. 意象/梗追踪：搜词在哪些章节出现 ----------
func (s *Server) projectToolMotifTrack(ctx context.Context, w http.ResponseWriter, projectID string, params map[string]interface{}) {
	word := strings.TrimSpace(fmt.Sprint(params["word"]))
	if word == "" {
		writeError(w, http.StatusBadRequest, "缺少搜索词 word")
		return
	}
	chs, _ := s.store.ListChapters(ctx, projectID, "")
	type hit struct {
		Chapter int    `json:"chapter"`
		Title   string `json:"title"`
		Count   int    `json:"count"`
		Context string `json:"context"` // 首次出现上下文
	}
	var hits []hit
	total := 0
	for i := range chs {
		c := chs[i].Content
		if strings.TrimSpace(c) == "" {
			continue
		}
		n := strings.Count(c, word)
		if n == 0 {
			continue
		}
		total += n
		ctxTxt := ""
		if idx := strings.Index(c, word); idx >= 0 {
			// idx 是字节索引，转成 rune 索引后切片（中文安全）
			r := []rune(c)
			runeIdx := len([]rune(c[:idx]))
			start := runeIdx - 15
			if start < 0 {
				start = 0
			}
			end := runeIdx + len([]rune(word)) + 15
			if end > len(r) {
				end = len(r)
			}
			ctxTxt = string(r[start:end])
		}
		hits = append(hits, hit{Chapter: chs[i].SortOrder, Title: chs[i].Title, Count: n, Context: ctxTxt})
	}
	writeOK(w, map[string]interface{}{"result": hits, "word": word, "total_occurrences": total})
}

// ---------- 4. 章末钩子检查：每章结尾是否留了悬念 ----------
func (s *Server) projectToolHookCheck(ctx context.Context, w http.ResponseWriter, projectID string) {
	chs, _ := s.store.ListChapters(ctx, projectID, "")
	type hook struct {
		Chapter int    `json:"chapter"`
		Title   string `json:"title"`
		Ending  string `json:"ending"`
		IsHook  bool   `json:"is_hook"`
		Reason  string `json:"reason"`
	}
	var out []hook
	for i := range chs {
		content := strings.TrimSpace(chs[i].Content)
		if content == "" {
			continue
		}
		ending := lastSentences(content, 2)
		out = append(out, hook{
			Chapter: chs[i].SortOrder,
			Title:   chs[i].Title,
			Ending:  ending,
			IsHook:  looksLikeHook(ending),
			Reason:  hookReason(ending),
		})
	}
	writeOK(w, map[string]interface{}{"result": out})
}

// lastSentences 取文本最后 n 个句子
func lastSentences(text string, n int) string {
	sents := splitSentencesBy(text)
	if len(sents) <= n {
		return strings.TrimSpace(text)
	}
	return strings.TrimSpace(strings.Join(sents[len(sents)-n:], ""))
}

// splitSentencesBy 按句号/问号/叹号切句（保留标点）
func splitSentencesBy(text string) []string {
	var out []string
	var b strings.Builder
	for _, r := range text {
		b.WriteRune(r)
		if r == '。' || r == '！' || r == '？' || r == '…' || r == '!' || r == '?' {
			if strings.TrimSpace(b.String()) != "" {
				out = append(out, b.String())
			}
			b.Reset()
		}
	}
	if s := strings.TrimSpace(b.String()); s != "" {
		out = append(out, s)
	}
	return out
}

// looksLikeHook 判断结尾是否像钩子（悬念/转折/新信息/未完成感）
func looksLikeHook(end string) bool {
	return hookReason(end) != ""
}

var hookKeywords = []string{"突然", "却", "可是", "但是", "没想到", "竟然", "居然", "然而", "忽然", "……", "?", "？", "!", "！", "没想到", "是谁", "到底", "怎么回事", "该不会", "难道", "他/她", "下一秒", "那天", "再见", "明天"}

func hookReason(end string) string {
	if strings.Contains(end, "？") || strings.Contains(end, "?") {
		return "以问句结尾，留悬念"
	}
	if strings.Contains(end, "…") || strings.Contains(end, "……") {
		return "以省略号结尾，话未说完"
	}
	for _, k := range []string{"突然", "忽然", "竟然", "居然", "没想到", "却", "可是", "然而"} {
		if strings.Contains(end, k) {
			return "含转折词（" + k + "）"
		}
	}
	if strings.Contains(end, "！") || strings.Contains(end, "!") {
		return "以感叹结尾（情绪冲击）"
	}
	return ""
}

// ---------- 5. 时间线视图：按章节列关键事件（标题/字数/摘要/更新时间） ----------
func (s *Server) projectToolTimeline(ctx context.Context, w http.ResponseWriter, projectID string) {
	chs, _ := s.store.ListChapters(ctx, projectID, "")
	type ev struct {
		Chapter   int    `json:"chapter"`
		Title     string `json:"title"`
		WordCount int    `json:"word_count"`
		Synopsis  string `json:"synopsis"`
		UpdatedAt string `json:"updated_at"`
	}
	var out []ev
	for i := range chs {
		out = append(out, ev{
			Chapter:   chs[i].SortOrder,
			Title:     chs[i].Title,
			WordCount: chs[i].WordCount,
			Synopsis:  truncateRunes2(strings.TrimSpace(chs[i].Synopsis), 80),
			UpdatedAt: chs[i].UpdatedAt,
		})
	}
	writeOK(w, map[string]interface{}{"result": out})
}

// ---------- 6. 写作统计日历：按天统计新增字数 ----------
func (s *Server) projectToolStatsCalendar(ctx context.Context, w http.ResponseWriter, projectID string) {
	chs, _ := s.store.ListChapters(ctx, projectID, "")
	byDay := map[string]int{}
	var days []string
	for i := range chs {
		d := chs[i].UpdatedAt
		if len(d) >= 10 {
			d = d[:10]
		}
		if _, ok := byDay[d]; !ok {
			days = append(days, d)
		}
		byDay[d] += chs[i].WordCount
	}
	sort.Strings(days)
	type day struct {
		Date  string `json:"date"`
		Words int    `json:"words"`
	}
	var out []day
	total := 0
	for _, d := range days {
		out = append(out, day{Date: d, Words: byDay[d]})
		total += byDay[d]
	}
	writeOK(w, map[string]interface{}{"result": out, "total_words": total})
}

// ---------- 小工具 ----------
func truncateRunes2(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

func itoa(n int) string {
	return fmt.Sprintf("%d", n)
}

var _ = time.Now // 保留 time 引用（如后续加日期统计）
