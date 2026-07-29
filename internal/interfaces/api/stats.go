package api

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// HandleListLogs 模型调用日志查询（规格第七章5：调用记录持久化）
func (s *Server) HandleListLogs(w http.ResponseWriter, r *http.Request) {
	pid := r.URL.Query().Get("project_id")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	items, err := s.store.ListLogs(r.Context(), pid, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, map[string]interface{}{"items": items})
}

// HandleUsage 当日用量统计 + 近 N 天各模型明细 + 各角色消耗（规格第七章5/7）
func (s *Server) HandleUsage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	calls, tokens, _ := s.store.DailyTotalUsage(ctx)
	dailyByModel, _ := s.store.DailyUsageByModel(ctx)
	periodByModel, _ := s.store.PeriodUsage(ctx, 7)
	dailyByRole, _ := s.store.DailyUsageByRole(ctx)

	limits := map[string]int{
		"daily_call_limit":        s.store.GetConfigInt(ctx, "daily_call_limit", 500),
		"daily_token_limit":       s.store.GetConfigInt(ctx, "daily_token_limit", 2000000),
		"per_request_token_limit": s.store.GetConfigInt(ctx, "per_request_token_limit", 8000),
		"rate_limit_per_minute":   s.store.GetConfigInt(ctx, "rate_limit_per_minute", 20),
		"max_concurrent":          s.store.GetConfigInt(ctx, "max_concurrent", 5),
		"max_iterations":          s.store.GetConfigInt(ctx, "max_iterations", 3),
	}
	writeOK(w, map[string]interface{}{
		"today":          map[string]int{"calls": calls, "tokens": tokens},
		"today_by_model": dailyByModel,
		"today_by_role":  dailyByRole,
		"week_by_model":  periodByModel,
		"limits":         limits,
	})
}

// HandleListConfigs 列出全部后台配置（限额参数）
func (s *Server) HandleListConfigs(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListConfigs(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, map[string]interface{}{"items": items})
}

// HandleSetConfig 修改单个后台配置（无需改代码即可调整阈值）
func (s *Server) HandleSetConfig(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	var req struct {
		Value       string `json:"value"`
		Description string `json:"description"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.store.SetConfig(r.Context(), key, req.Value, req.Description); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, map[string]string{"status": "ok"})
}
