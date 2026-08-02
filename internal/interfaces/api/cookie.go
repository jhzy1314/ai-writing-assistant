package api

import (
	"net/http"
	"time"

	"github.com/ai-novel/studio/internal/infrastructure/llm"
	"github.com/go-chi/chi/v5"
)

func (s *Server) HandleAutoCookieStart(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Provider string `json:"provider"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Provider == "" {
		writeError(w, http.StatusBadRequest, "provider 必填")
		return
	}

	session, err := llm.StartCookieCapture(req.Provider)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "启动浏览器失败: "+err.Error())
		return
	}

	writeOK(w, map[string]interface{}{
		"session_id": session.ID,
		"status":     session.Status,
	})
}

func (s *Server) HandleAutoCookiePoll(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "session_id")
	if sessionID == "" {
		writeError(w, http.StatusBadRequest, "session_id 必填")
		return
	}

	session := llm.GetCookieSession(sessionID)
	if session == nil {
		writeError(w, http.StatusNotFound, "会话不存在或已过期")
		return
	}

	resp := map[string]interface{}{
		"status": session.Status,
	}
	if session.Status == "pending" || session.Status == "running" {
		resp["detected_cookies"] = session.DetectedCookies
		resp["detected_len"] = session.DetectedLen
		resp["elapsed_seconds"] = int(time.Since(session.StartedAt).Seconds())
	}
	if session.Status == "completed" {
		resp["cookie"] = session.Cookies
		resp["session_id"] = session.ID
		if session.Token != "" {
			resp["token"] = session.Token
		}
	}
	if session.Status == "failed" {
		resp["error"] = session.Error
	}

	writeOK(w, resp)
}

func (s *Server) HandleAutoCookieCancel(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "session_id")
	if sessionID == "" {
		writeError(w, http.StatusBadRequest, "session_id 必填")
		return
	}

	llm.CancelCookieSession(sessionID)
	w.WriteHeader(http.StatusNoContent)
}
