package api

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

// ===== 项目管理 =====

func (s *Server) HandleListProjects(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListProjects(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, map[string]interface{}{"items": items})
}

func (s *Server) HandleCreateProject(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		writeError(w, http.StatusBadRequest, "name 不能为空")
		return
	}
	p, err := s.store.CreateProject(r.Context(), strings.TrimSpace(req.Name), strings.TrimSpace(req.Type))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeCreated(w, map[string]interface{}{"item": p})
}

func (s *Server) HandleGetProject(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	p, err := s.store.GetProject(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if p == nil {
		writeError(w, http.StatusNotFound, "项目不存在")
		return
	}
	// 附带最新版本与资源计数
	latest, _ := s.store.LatestVersion(r.Context(), id)
	chars, _ := s.store.ListCharacters(r.Context(), id)
	ws, _ := s.store.ListWorldSettings(r.Context(), id)
	mats, _ := s.store.ListMaterials(r.Context(), id)
	chCount := s.store.ChapterCount(r.Context(), id)
	volCount := s.store.VolumeCount(r.Context(), id)
	writeOK(w, map[string]interface{}{
		"item":             p,
		"latest_version":   latest,
		"character_count":  len(chars),
		"worldsetting_count": len(ws),
		"material_count":   len(mats),
		"chapter_count":    chCount,
		"volume_count":     volCount,
	})
}

func (s *Server) HandleUpdateProject(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		Name *string `json:"name"`
		Type *string `json:"type"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	p, err := s.store.UpdateProject(r.Context(), id, req.Name, req.Type)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if p == nil {
		writeError(w, http.StatusNotFound, "项目不存在")
		return
	}
	writeOK(w, map[string]interface{}{"item": p})
}

func (s *Server) HandleDeleteProject(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := s.store.DeleteProject(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) HandleDuplicateProject(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	src, err := s.store.GetProject(r.Context(), id)
	if err != nil || src == nil {
		writeError(w, http.StatusNotFound, "项目不存在")
		return
	}
	dup, err := s.store.CreateProject(r.Context(), src.Name+" (副本)", src.Type)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "复制失败: "+err.Error())
		return
	}
	// 复制章节和版本
	chs, _ := s.store.ListChapters(r.Context(), id, "")
	for _, c := range chs {
		_, _ = s.store.CreateChapter(r.Context(), dup.ID, c.VolumeID, c.Title, c.Content)
	}
	writeCreated(w, map[string]interface{}{"item": dup})
}

// ===== 稿件版本 =====

func (s *Server) HandleListVersions(w http.ResponseWriter, r *http.Request) {
	pid := chi.URLParam(r, "id")
	items, err := s.store.ListVersions(r.Context(), pid)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, map[string]interface{}{"items": items})
}

func (s *Server) HandleSaveVersion(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProjectID string `json:"project_id"`
		Title     string `json:"title"`
		Content   string `json:"content"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(req.ProjectID) == "" {
		writeError(w, http.StatusBadRequest, "project_id 不能为空")
		return
	}
	v, err := s.store.SaveVersion(r.Context(), req.ProjectID, req.Title, req.Content)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeCreated(w, map[string]interface{}{"item": v})
}

func (s *Server) HandleGetVersion(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	v, err := s.store.GetVersion(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if v == nil {
		writeError(w, http.StatusNotFound, "版本不存在")
		return
	}
	writeOK(w, map[string]interface{}{"item": v})
}
