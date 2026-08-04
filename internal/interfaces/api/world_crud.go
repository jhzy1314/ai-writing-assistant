package api

// 2026-08-05 转型纯作家辅助工具：势力 / 地点 / 时间线事件 CRUD handlers

import (
	"database/sql"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/ai-novel/studio/internal/infrastructure/database"
)

// ===== factions =====

func (s *Server) HandleListFactions(w http.ResponseWriter, r *http.Request) {
	pid := r.URL.Query().Get("project_id")
	if pid == "" {
		writeError(w, http.StatusBadRequest, "缺少 project_id 查询参数")
		return
	}
	items, err := s.store.ListFactions(r.Context(), pid)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, map[string]interface{}{"items": items})
}

func (s *Server) HandleCreateFaction(w http.ResponseWriter, r *http.Request) {
	var req database.Faction
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(req.ProjectID) == "" || strings.TrimSpace(req.Name) == "" {
		writeError(w, http.StatusBadRequest, "project_id 与 name 必填")
		return
	}
	item, err := s.store.CreateFaction(r.Context(), req.ProjectID, req.Name, req.Description, req.Leader, req.Members, req.Relations)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeCreated(w, map[string]interface{}{"item": item})
}

func (s *Server) HandleUpdateFaction(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
		Leader      *string `json:"leader"`
		Members     *string `json:"members"`
		Relations   *string `json:"relations"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var name, desc, leader, members, rel string
	if req.Name != nil {
		name = *req.Name
	}
	if req.Description != nil {
		desc = *req.Description
	}
	if req.Leader != nil {
		leader = *req.Leader
	}
	if req.Members != nil {
		members = *req.Members
	}
	if req.Relations != nil {
		rel = *req.Relations
	}
	if err := s.store.UpdateFaction(r.Context(), id, name, desc, leader, members, rel); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, map[string]interface{}{"id": id})
}

func (s *Server) HandleDeleteFaction(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := s.store.DeleteFaction(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, map[string]interface{}{"deleted": true})
}

// ===== locations =====

func (s *Server) HandleListLocations(w http.ResponseWriter, r *http.Request) {
	pid := r.URL.Query().Get("project_id")
	if pid == "" {
		writeError(w, http.StatusBadRequest, "缺少 project_id 查询参数")
		return
	}
	items, err := s.store.ListLocations(r.Context(), pid)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, map[string]interface{}{"items": items})
}

func (s *Server) HandleCreateLocation(w http.ResponseWriter, r *http.Request) {
	var req database.Location
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(req.ProjectID) == "" || strings.TrimSpace(req.Name) == "" {
		writeError(w, http.StatusBadRequest, "project_id 与 name 必填")
		return
	}
	item, err := s.store.CreateLocation(r.Context(), req.ProjectID, req.Name, req.Description, req.Type, req.Related)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeCreated(w, map[string]interface{}{"item": item})
}

func (s *Server) HandleUpdateLocation(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
		Type        *string `json:"type"`
		Related     *string `json:"related"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var name, desc, typ, related string
	if req.Name != nil {
		name = *req.Name
	}
	if req.Description != nil {
		desc = *req.Description
	}
	if req.Type != nil {
		typ = *req.Type
	}
	if req.Related != nil {
		related = *req.Related
	}
	if err := s.store.UpdateLocation(r.Context(), id, name, desc, typ, related); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, map[string]interface{}{"id": id})
}

func (s *Server) HandleDeleteLocation(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := s.store.DeleteLocation(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, map[string]interface{}{"deleted": true})
}

// ===== timeline（事件时间线） =====

func (s *Server) HandleListTimelineEvents(w http.ResponseWriter, r *http.Request) {
	pid := r.URL.Query().Get("project_id")
	if pid == "" {
		writeError(w, http.StatusBadRequest, "缺少 project_id 查询参数")
		return
	}
	items, err := s.store.ListTimelineEvents(r.Context(), pid)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, map[string]interface{}{"items": items})
}

func (s *Server) HandleCreateTimelineEvent(w http.ResponseWriter, r *http.Request) {
	var req database.TimelineEvent
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(req.ProjectID) == "" || strings.TrimSpace(req.Event) == "" {
		writeError(w, http.StatusBadRequest, "project_id 与 event 必填")
		return
	}
	item, err := s.store.CreateTimelineEvent(r.Context(), req.ProjectID, req.ChapterID, req.Event, req.EventTime, req.Characters)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeCreated(w, map[string]interface{}{"item": item})
}

func (s *Server) HandleUpdateTimelineEvent(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		ChapterID  *string `json:"chapter_id"`
		Event      *string `json:"event"`
		EventTime  *string `json:"event_time"`
		Characters *string `json:"characters"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var chapterID, event, eventTime, characters string
	if req.ChapterID != nil {
		chapterID = *req.ChapterID
	}
	if req.Event != nil {
		event = *req.Event
	}
	if req.EventTime != nil {
		eventTime = *req.EventTime
	}
	if req.Characters != nil {
		characters = *req.Characters
	}
	if err := s.store.UpdateTimelineEvent(r.Context(), id, chapterID, event, eventTime, characters); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, map[string]interface{}{"id": id})
}

func (s *Server) HandleDeleteTimelineEvent(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := s.store.DeleteTimelineEvent(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, map[string]interface{}{"deleted": true})
}

// ===== relations（人物关系） =====

func (s *Server) HandleListRelations(w http.ResponseWriter, r *http.Request) {
	pid := r.URL.Query().Get("project_id")
	if pid == "" {
		writeError(w, http.StatusBadRequest, "缺少 project_id 查询参数")
		return
	}
	items, err := s.store.ListRelations(r.Context(), pid)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, map[string]interface{}{"items": items})
}

func (s *Server) HandleCreateRelation(w http.ResponseWriter, r *http.Request) {
	var req database.Relation
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(req.ProjectID) == "" || strings.TrimSpace(req.CharA) == "" || strings.TrimSpace(req.CharB) == "" {
		writeError(w, http.StatusBadRequest, "project_id 与 char_a、char_b 必填")
		return
	}
	item, err := s.store.CreateRelation(r.Context(), req.ProjectID, req.CharA, req.CharB, req.Relation, req.Note)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeCreated(w, map[string]interface{}{"item": item})
}

func (s *Server) HandleUpdateRelation(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		CharA    *string `json:"char_a"`
		CharB    *string `json:"char_b"`
		Relation *string `json:"relation"`
		Note     *string `json:"note"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var a, b, rel, note string
	if req.CharA != nil {
		a = *req.CharA
	}
	if req.CharB != nil {
		b = *req.CharB
	}
	if req.Relation != nil {
		rel = *req.Relation
	}
	if req.Note != nil {
		note = *req.Note
	}
	if err := s.store.UpdateRelation(r.Context(), id, a, b, rel, note); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, map[string]interface{}{"id": id})
}

func (s *Server) HandleDeleteRelation(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := s.store.DeleteRelation(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, map[string]interface{}{"deleted": true})
}

// ===== annotations（批注/高亮） =====

func (s *Server) HandleListAnnotations(w http.ResponseWriter, r *http.Request) {
	cid := r.URL.Query().Get("chapter_id")
	if cid == "" {
		writeError(w, http.StatusBadRequest, "缺少 chapter_id 查询参数")
		return
	}
	items, err := s.store.ListAnnotations(r.Context(), cid)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, map[string]interface{}{"items": items})
}

func (s *Server) HandleCreateAnnotation(w http.ResponseWriter, r *http.Request) {
	var req database.Annotation
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(req.ProjectID) == "" || strings.TrimSpace(req.ChapterID) == "" || req.End <= req.Start {
		writeError(w, http.StatusBadRequest, "project_id/chapter_id 必填，end 必须大于 start")
		return
	}
	item, err := s.store.CreateAnnotation(r.Context(), &req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeCreated(w, map[string]interface{}{"item": item})
}

func (s *Server) HandleUpdateAnnotation(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		Note  *string `json:"note"`
		Color *string `json:"color"`
		Start *int    `json:"start"`
		End   *int    `json:"end"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	note, color := "", ""
	if req.Note != nil {
		note = *req.Note
	}
	if req.Color != nil {
		color = *req.Color
	}
	if err := s.store.UpdateAnnotation(r.Context(), id, note, color, req.Start, req.End); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, map[string]interface{}{"id": id})
}

func (s *Server) HandleDeleteAnnotation(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := s.store.DeleteAnnotation(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, map[string]interface{}{"deleted": true})
}

// ===== reading_progress（阅读进度） =====

func (s *Server) HandleGetReadingProgress(w http.ResponseWriter, r *http.Request) {
	pid := r.URL.Query().Get("project_id")
	if pid == "" {
		writeError(w, http.StatusBadRequest, "缺少 project_id 查询参数")
		return
	}
	item, err := s.store.GetReadingProgress(r.Context(), pid)
	if err != nil {
		if err == sql.ErrNoRows {
			writeOK(w, map[string]interface{}{"item": nil})
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, map[string]interface{}{"item": item})
}

func (s *Server) HandleSetReadingProgress(w http.ResponseWriter, r *http.Request) {
	var req database.ReadingProgress
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(req.ProjectID) == "" {
		writeError(w, http.StatusBadRequest, "project_id 必填")
		return
	}
	if err := s.store.SetReadingProgress(r.Context(), req.ProjectID, req.ChapterID, req.ScrollPct); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, map[string]interface{}{"saved": true})
}
