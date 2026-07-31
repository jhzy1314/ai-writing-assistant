package api

import (
	"net/http"

	"github.com/ai-novel/studio/internal/infrastructure/rag"
)

// HandleRAGPreview POST /api/rag/preview —— 预览当前需求的 RAG 检索结果（不生成，只展示命中的相关记忆）
// 请求: {project_id, chapter_id, user_demand}
// 响应: {chunks: [{chapter_no, title, text, score}]}
func (s *Server) HandleRAGPreview(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProjectID   string `json:"project_id"`
		ChapterID   string `json:"chapter_id"`
		UserDemand  string `json:"user_demand"`
		SelectedText string `json:"selected_text"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.ProjectID == "" || s.rag == nil {
		writeOK(w, map[string]interface{}{"chunks": []interface{}{}})
		return
	}
	// 懒建索引
	n, _ := s.store.CountRAGChunks(r.Context(), req.ProjectID)
	if n == 0 {
		_ = s.rag.IndexChapters(r.Context(), req.ProjectID)
	}
	query := req.UserDemand + "\n" + req.SelectedText
	chunks, err := s.rag.Search(r.Context(), req.ProjectID, req.ChapterID, query, 5)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]map[string]interface{}, 0, len(chunks))
	for _, c := range chunks {
		out = append(out, map[string]interface{}{
			"chapter_no": c.ChapterNo,
			"title":      c.Title,
			"text":       c.Text,
			"score":      ragPreviewScore(c.Text, query),
		})
	}
	writeOK(w, map[string]interface{}{"chunks": out})
}

// ragPreviewScore 简易相关度展示值（文本重叠比例，仅用于可视化排序展示）
func ragPreviewScore(text, query string) float64 {
	if text == "" || query == "" {
		return 0
	}
	toks := map[rune]bool{}
	for _, r := range query {
		if r >= 0x4E00 && r <= 0x9FFF {
			toks[r] = true
		}
	}
	if len(toks) == 0 {
		return 0
	}
	hit := 0
	for _, r := range text {
		if toks[r] {
			hit++
		}
	}
	score := float64(hit) / float64(len(toks))
	if score > 1 {
		score = 1
	}
	return score
}

var _ = rag.Cosine // keep import
