package api

import (
	"net/http"
	"strings"

	"github.com/ai-novel/studio/internal/infrastructure/database"
	"github.com/go-chi/chi/v5"
)

func (s *Server) HandleListTemplates(w http.ResponseWriter, r *http.Request) {
	category := r.URL.Query().Get("category")
	items, err := s.store.ListTemplates(r.Context(), category)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, map[string]interface{}{"items": items})
}

func (s *Server) HandleCreateTemplate(w http.ResponseWriter, r *http.Request) {
	var t database.Template
	if err := decodeJSON(r, &t); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(t.Name) == "" || strings.TrimSpace(t.Content) == "" {
		writeError(w, http.StatusBadRequest, "name 与 content 必填")
		return
	}
	item, err := s.store.CreateTemplate(r.Context(), t.Name, t.Category, t.Content, false)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeCreated(w, map[string]interface{}{"item": item})
}

func (s *Server) HandleUpdateTemplate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		Name     *string `json:"name"`
		Category *string `json:"category"`
		Content  *string `json:"content"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	item, err := s.store.UpdateTemplate(r.Context(), id, req.Name, req.Category, req.Content)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if item == nil {
		writeError(w, http.StatusNotFound, "模板不存在或为系统内置不可修改")
		return
	}
	writeOK(w, map[string]interface{}{"item": item})
}

func (s *Server) HandleDeleteTemplate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := s.store.DeleteTemplate(r.Context(), id); err != nil {
		if database.IsSystemTemplateError(err) {
			writeError(w, http.StatusForbidden, "系统内置模板不可删除")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
