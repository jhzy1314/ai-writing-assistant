package api

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"github.com/ai-novel/studio/internal/infrastructure/database"
	"github.com/go-chi/chi/v5"
)

// ============================================================
// stylesamples.go —— 文风样本库 API（本地知识库）
// 内容存本地库，供生成时风格参考；列表接口不含正文（省流量）
// ============================================================

// HandleImportStyleSamples POST /api/stylesamples/import —— 批量导入（cmd/sample-import 产物）
func (s *Server) HandleImportStyleSamples(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Samples []database.StyleSample `json:"samples"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(req.Samples) == 0 {
		writeError(w, http.StatusBadRequest, "samples 不能为空")
		return
	}
	n, err := s.store.ImportStyleSamples(r.Context(), req.Samples)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, map[string]interface{}{"imported": n})
}

// HandleListStyleSamples GET /api/stylesamples?category=xxx
func (s *Server) HandleListStyleSamples(w http.ResponseWriter, r *http.Request) {
	cat := r.URL.Query().Get("category")
	withContent := r.URL.Query().Get("full") == "1"
	var (
		items []database.StyleSample
		err   error
	)
	if withContent {
		items, err = s.store.ListStyleSamples(r.Context(), cat)
	} else {
		items, err = s.store.ListStyleSampleMeta(r.Context())
		if err == nil && cat != "" {
			filtered := items[:0]
			for _, m := range items {
				if m.Category == cat {
					filtered = append(filtered, m)
				}
			}
			items = filtered
		}
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, map[string]interface{}{"items": items})
}

// HandleGetStyleSample GET /api/stylesamples/{id}（含正文）
func (s *Server) HandleGetStyleSample(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	m, err := s.store.GetStyleSample(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if m == nil {
		writeError(w, http.StatusNotFound, "样本不存在")
		return
	}
	writeOK(w, map[string]interface{}{"item": m})
}

// HandleCreateStyleSample POST /api/stylesamples
func (s *Server) HandleCreateStyleSample(w http.ResponseWriter, r *http.Request) {
	var m database.StyleSample
	if err := decodeJSON(r, &m); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(m.Content) == "" {
		writeError(w, http.StatusBadRequest, "content 必填")
		return
	}
	item, err := s.store.CreateStyleSample(r.Context(), m)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeCreated(w, map[string]interface{}{"item": item})
}

// HandleUpdateStyleSample PUT /api/stylesamples/{id}
func (s *Server) HandleUpdateStyleSample(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		Title    *string `json:"title"`
		Author   *string `json:"author"`
		Category *string `json:"category"`
		Content  *string `json:"content"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	item, err := s.store.UpdateStyleSample(r.Context(), id, req.Title, req.Author, req.Category, req.Content)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "样本不存在")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if item == nil {
		writeError(w, http.StatusNotFound, "样本不存在")
		return
	}
	writeOK(w, map[string]interface{}{"item": item})
}

// HandleDeleteStyleSample DELETE /api/stylesamples/{id}
func (s *Server) HandleDeleteStyleSample(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := s.store.DeleteStyleSample(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
