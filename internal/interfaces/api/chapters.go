package api

import (
	"context"
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

	// AI 智能分章：让 AI 通读全文后标注章节边界，再传给正则精确切分
	if req.SplitBy == "" || req.SplitBy == "auto" {
		aiContent, err := s.aiSmartSplit(r.Context(), req.Content)
		if err == nil && aiContent != "" {
			req.Content = aiContent
			req.SplitBy = "## " // 已由 AI 添加 ## 标题，正则可以精确切分
		}
		// AI 失败时静默回退到纯正则模式（不影响用户操作）
	}

	chapters, err := s.store.SplitChapters(r.Context(), &req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, map[string]interface{}{"items": chapters, "count": len(chapters)})
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

// aiSmartSplit 调用 deepseek-v4-flash 通读全文后智能识别章节边界
func (s *Server) aiSmartSplit(ctx context.Context, content string) (string, error) {
	adapter, err := s.registry.AdapterByName(ctx, "deepseek-v4-flash")
	if err != nil {
		adapters, err2 := s.registry.AdaptersForRole(ctx, llm.RoleThinker)
		if err2 != nil || len(adapters) == 0 {
			return "", fmt.Errorf("no model available for AI split")
		}
		adapter = adapters[0]
	}

	var result strings.Builder
	chunkSize := 12000
	runes := []rune(content)

	for i := 0; i < len(runes); i += chunkSize {
		end := i + chunkSize
		if end > len(runes) {
			end = len(runes)
		}
		chunk := string(runes[i:end])

		subCtx, cancel := context.WithTimeout(ctx, 60*time.Second)

		sysPrompt := `你是一个专业的小说编辑。请仔细阅读以下小说文本，找出其中所有真实的章节分界点。
规则：
1. 只识别真正的章节开始位置，不要把文中叙述中提到的"第一天""第二天""回来"等词语误判为章节标题
2. 真正的章节标题通常包含"第X章"或"第X卷"等明确标记
3. 如果文本开头有卷首页、前言等不属于章节的内容，单独标注为"前言"
4. 用 ## 号标记每个识别出的章节标题
5. 不要在叙述性文字中间插入章节标记
6. 如果文本中没有明确的章节边界，就不要强行分割

请输出处理后的完整文本。格式示例：
前言内容...

## 第一章 标题

章节正文...

## 第二章 标题

章节正文...`

		aiText, _, err := adapter.Generate(subCtx, sysPrompt, chunk)
		cancel()
		if err != nil {
			return "", fmt.Errorf("AI split failed: %v", err)
		}
		result.WriteString(aiText)
	}

	return result.String(), nil
}
