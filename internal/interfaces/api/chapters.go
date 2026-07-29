package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ai-novel/studio/internal/infrastructure/database"
	"github.com/ai-novel/studio/internal/infrastructure/llm"
	"github.com/go-chi/chi/v5"
)

// ===== 卷管理 =====

func (s *Server) HandleListVolumes(w http.ResponseWriter, r *http.Request) {
	pid := chi.URLParam(r, "id")
	items, err := s.store.ListVolumes(r.Context(), pid)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, map[string]interface{}{"items": items})
}

func (s *Server) HandleCreateVolume(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProjectID string `json:"project_id"`
		Title     string `json:"title"`
		SortOrder int    `json:"sort_order"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(req.ProjectID) == "" || strings.TrimSpace(req.Title) == "" {
		writeError(w, http.StatusBadRequest, "project_id 与 title 必填")
		return
	}
	v, err := s.store.CreateVolume(r.Context(), req.ProjectID, strings.TrimSpace(req.Title), req.SortOrder)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeCreated(w, map[string]interface{}{"item": v})
}

func (s *Server) HandleUpdateVolume(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		Title     *string `json:"title"`
		SortOrder *int    `json:"sort_order"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	v, err := s.store.UpdateVolume(r.Context(), id, req.Title, req.SortOrder)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if v == nil {
		writeError(w, http.StatusNotFound, "卷不存在")
		return
	}
	writeOK(w, map[string]interface{}{"item": v})
}

func (s *Server) HandleDeleteVolume(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := s.store.DeleteVolume(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) HandleReorderVolumes(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Items []struct {
			ID        string `json:"id"`
			SortOrder int    `json:"sort_order"`
		} `json:"items"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(req.Items) == 0 {
		writeError(w, http.StatusBadRequest, "items 不能为空")
		return
	}
	if err := s.store.ReorderVolumes(r.Context(), req.Items); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, map[string]string{"status": "ok"})
}

// ===== 章节管理 =====

func (s *Server) HandleListChapters(w http.ResponseWriter, r *http.Request) {
	pid := r.URL.Query().Get("project_id")
	vid := r.URL.Query().Get("volume_id")
	if pid == "" {
		writeError(w, http.StatusBadRequest, "缺少 project_id 查询参数")
		return
	}
	items, err := s.store.ListChapters(r.Context(), pid, vid)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, map[string]interface{}{"items": items})
}

func (s *Server) HandleSearchChapters(w http.ResponseWriter, r *http.Request) {
	pid := chi.URLParam(r, "id")
	q := r.URL.Query().Get("q")
	if pid == "" || q == "" { writeError(w, http.StatusBadRequest, "缺少 project_id 或 q 参数"); return }
	results, err := s.store.SearchChapters(r.Context(), pid, q)
	if err != nil { writeError(w, http.StatusInternalServerError, err.Error()); return }
	writeOK(w, map[string]interface{}{"items": results})
}

func (s *Server) HandleCreateChapter(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProjectID string `json:"project_id"`
		VolumeID  string `json:"volume_id"`
		Title     string `json:"title"`
		Content   string `json:"content"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(req.ProjectID) == "" {
		writeError(w, http.StatusBadRequest, "project_id 必填")
		return
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = "新章节"
	}
	ch, err := s.store.CreateChapter(r.Context(), req.ProjectID, req.VolumeID, title, req.Content)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeCreated(w, map[string]interface{}{"item": ch})
}

func (s *Server) HandleGetChapter(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ch, err := s.store.GetChapter(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if ch == nil {
		writeError(w, http.StatusNotFound, "章节不存在")
		return
	}
	writeOK(w, map[string]interface{}{"item": ch})
}

func (s *Server) HandleUpdateChapter(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		Title    *string `json:"title"`
		Content  *string `json:"content"`
		VolumeID *string `json:"volume_id"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	ch, err := s.store.UpdateChapter(r.Context(), id, req.Title, req.Content, req.VolumeID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if ch == nil {
		writeError(w, http.StatusNotFound, "章节不存在")
		return
	}
	writeOK(w, map[string]interface{}{"item": ch})
}

func (s *Server) HandleDeleteChapter(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := s.store.DeleteChapter(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) HandleCopyChapter(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ch, err := s.store.CopyChapter(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if ch == nil {
		writeError(w, http.StatusNotFound, "章节不存在")
		return
	}
	writeCreated(w, map[string]interface{}{"item": ch})
}

func (s *Server) HandleReorderChapters(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Items []struct {
			ID        string `json:"id"`
			SortOrder int    `json:"sort_order"`
		} `json:"items"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(req.Items) == 0 {
		writeError(w, http.StatusBadRequest, "items 不能为空")
		return
	}
	if err := s.store.ReorderChapters(r.Context(), req.Items); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, map[string]string{"status": "ok"})
}

// ===== 章节版本管理 =====

func (s *Server) HandleListChapterVersions(w http.ResponseWriter, r *http.Request) {
	cid := chi.URLParam(r, "id")
	items, err := s.store.ListChapterVersions(r.Context(), cid)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, map[string]interface{}{"items": items})
}

func (s *Server) HandleSaveChapterVersion(w http.ResponseWriter, r *http.Request) {
	cid := chi.URLParam(r, "id")
	var req struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	cv, err := s.store.SaveChapterVersion(r.Context(), cid, req.Title, req.Content)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeCreated(w, map[string]interface{}{"item": cv})
}

func (s *Server) HandleGetChapterVersion(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	cv, err := s.store.GetChapterVersion(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if cv == nil {
		writeError(w, http.StatusNotFound, "章节版本不存在")
		return
	}
	writeOK(w, map[string]interface{}{"item": cv})
}

// ===== 导入导出 / 分割 =====

func (s *Server) HandleExportChapters(w http.ResponseWriter, r *http.Request) {
	pid := r.URL.Query().Get("project_id")
	if pid == "" {
		writeError(w, http.StatusBadRequest, "缺少 project_id 查询参数")
		return
	}
	data, err := s.store.ExportChapters(r.Context(), pid)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, map[string]interface{}{"item": data})
}

func (s *Server) HandleImportChapters(w http.ResponseWriter, r *http.Request) {
	var req database.ExportData
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "请求体解析失败: "+err.Error())
		return
	}
	if strings.TrimSpace(req.ProjectID) == "" {
		writeError(w, http.StatusBadRequest, "project_id 必填")
		return
	}
	count, err := s.store.ImportChapters(r.Context(), req.ProjectID, &req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, map[string]interface{}{"imported": count, "status": "ok"})
}

func (s *Server) HandleSplitChapters(w http.ResponseWriter, r *http.Request) {
	var req database.SplitChaptersRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "请求体解析失败: "+err.Error())
		return
	}
	if strings.TrimSpace(req.ProjectID) == "" || strings.TrimSpace(req.Content) == "" {
		writeError(w, http.StatusBadRequest, "project_id 与 content 必填")
		return
	}

	// AI 分章：先正则快速切分，失败则调专用 Agent 识别标题后精确切割原文
	if req.SplitBy == "" || req.SplitBy == "auto" {
		if s.store.CountSegments(req.Content, "auto") < 2 {
			titles, err := s.splitTitlesFromAI(r.Context(), req.Content)
			if err == nil && len(titles) >= 2 {
				chapters, splitErr := s.store.SplitByTitles(r.Context(), &req, titles)
				if splitErr == nil {
					writeOK(w, map[string]interface{}{"items": chapters, "count": len(chapters)})
					return
				}
			}
		}
	}

	chapters, err := s.store.SplitChapters(r.Context(), &req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, map[string]interface{}{"items": chapters, "count": len(chapters)})
}

// splitTitlesFromAI 专用 Agent：只识别章目标题列表，不重写正文（省 Token、保原文）
func (s *Server) splitTitlesFromAI(ctx context.Context, content string) ([]string, error) {
	adapters, err := s.registry.AdaptersForRole(ctx, llm.RoleHelper)
	if err != nil || len(adapters) == 0 {
		return nil, fmt.Errorf("no model available")
	}
	adapter := adapters[0]

	// 取文本首尾各 3000 字做采样，覆盖开头+结尾章节标题
	runes := []rune(content)
	sample := string(runes[:min(3000, len(runes))])
	if len(runes) > 6000 {
		sample += "\n\n…[中间略]…\n\n" + string(runes[len(runes)-min(2000, len(runes)-3000):])
	}

	sysPrompt := `你是小说章节分割专家。你的唯一任务：分析文本，列出所有真实的章节标题。

规则：
1. 真正的章节标题模式："第X章"、"Chapter X"、"第X节"、单独成行的短标题（<25字）
2. 排除口语化时间状语："第一天"、"第二天"、"几小时后"等不是标题
3. 排除书中提及但不在章节开头的标题
4. 按原文出现顺序输出

输出格式：纯 JSON 字符串数组。不输出任何其他内容。
示例：["第一章 雨夜","第二章 初遇","第三章 离别"]`

	subCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	aiText, _, err := adapter.Generate(subCtx, sysPrompt, sample)
	if err != nil {
		return nil, err
	}

	// 提取 JSON 数组
	start := strings.Index(aiText, "[")
	end := strings.LastIndex(aiText, "]")
	if start < 0 || end <= start {
		return nil, fmt.Errorf("AI 返回格式无效")
	}
	jsonStr := aiText[start : end+1]

	var titles []string
	if err := json.Unmarshal([]byte(jsonStr), &titles); err != nil {
		return nil, fmt.Errorf("JSON 解析失败: %w", err)
	}
	return titles, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ===== 统计 =====

func (s *Server) HandleProjectStats(w http.ResponseWriter, r *http.Request) {
	pid := chi.URLParam(r, "id")
	if pid == "" {
		writeError(w, http.StatusBadRequest, "缺少 project_id")
		return
	}
	stats, err := s.store.GetProjectStats(r.Context(), pid)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, map[string]interface{}{"item": stats})
}

// ===== 合并/拆分 =====

func (s *Server) HandleMergeChapters(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ChapterIDs []string `json:"chapter_ids"`
		Title      string   `json:"title"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(req.ChapterIDs) < 2 {
		writeError(w, http.StatusBadRequest, "至少选择2个章节进行合并")
		return
	}
	if strings.TrimSpace(req.Title) == "" {
		req.Title = "合并章节"
	}
	ch, err := s.store.MergeChapters(r.Context(), req.ChapterIDs, strings.TrimSpace(req.Title))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeCreated(w, map[string]interface{}{"item": ch})
}

func (s *Server) HandleSplitChapter(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		CursorPos int `json:"cursor_pos"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.CursorPos <= 0 {
		writeError(w, http.StatusBadRequest, "cursor_pos 必须大于0")
		return
	}
	chapters, err := s.store.SplitChapter(r.Context(), id, req.CursorPos)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeCreated(w, map[string]interface{}{"items": chapters})
}
