package api

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/ai-novel/studio/internal/infrastructure/database"
	"github.com/ai-novel/studio/internal/infrastructure/llm"
	"github.com/go-chi/chi/v5"
)

// ===== 模型配置（后台管理） =====

func (s *Server) HandleListModels(w http.ResponseWriter, r *http.Request) {
	// 脱敏 api_key
	items, err := s.store.ListModels(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	for i := range items {
		if items[i].APIKey != "" {
			items[i].APIKey = database.MaskAPIKey(items[i].APIKey)
		}
	}
	writeOK(w, map[string]interface{}{"items": items})
}

func (s *Server) HandleCreateModel(w http.ResponseWriter, r *http.Request) {
	var m database.ModelConfig
	if err := decodeJSON(r, &m); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(m.Name) == "" || strings.TrimSpace(m.APIKey) == "" {
		writeError(w, http.StatusBadRequest, "name 与 api_key 必填")
		return
	}
	item, err := s.store.CreateModelFull(r.Context(), m.Name, m.Vendor, m.APIEndpoint, m.APIKey, m.Status, m.DailyLimit, m.IsCustom, m.ContextLimit, m.SupportStream, m.IsDefault, m.Description, m.Temperature, m.TopP)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	item.APIKey = database.MaskAPIKey(item.APIKey)
	writeCreated(w, map[string]interface{}{"item": item})
}

func (s *Server) HandleUpdateModel(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		Name          *string  `json:"name"`
		Vendor        *string  `json:"vendor"`
		APIEndpoint   *string  `json:"api_endpoint"`
		APIKey        *string  `json:"api_key"`
		Status        *string  `json:"status"`
		DailyLimit    *int     `json:"daily_limit"`
		ContextLimit  *int     `json:"context_limit"`
		SupportStream *int     `json:"support_stream"`
		IsDefault     *int     `json:"is_default"`
		IsCustom      *int     `json:"is_custom"`
		Description   *string  `json:"description"`
		Temperature   *float64 `json:"temperature"`
		TopP          *float64 `json:"top_p"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	item, err := s.store.UpdateModel(r.Context(), id, req.Name, req.Vendor, req.APIEndpoint, req.APIKey, req.Status, req.DailyLimit, req.ContextLimit, req.SupportStream, req.IsDefault, req.IsCustom, req.Description, req.Temperature, req.TopP)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if item == nil {
		writeError(w, http.StatusNotFound, "模型不存在")
		return
	}
	// 配置变更后失效适配器缓存
	s.registry.Invalidate(item.Name)
	item.APIKey = database.MaskAPIKey(item.APIKey)
	writeOK(w, map[string]interface{}{"item": item})
}

func (s *Server) HandleDeleteModel(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	// 删除前取名称以失效缓存
	m, _ := s.store.GetModel(r.Context(), id)
	if err := s.store.DeleteModel(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if m != nil {
		s.registry.Invalidate(m.Name)
	}
	w.WriteHeader(http.StatusNoContent)
}

// ===== 角色绑定模型优先级 =====

func (s *Server) HandleGetRoleModels(w http.ResponseWriter, r *http.Request) {
	role := chi.URLParam(r, "role")
	if !llm.ValidRole(role) {
		writeError(w, http.StatusBadRequest, "非法角色，可选: thinker/worker/verifier/helper")
		return
	}
	res, err := s.store.GetRoleModels(r.Context(), role)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, map[string]interface{}{"item": res})
}

func (s *Server) HandleSetRoleModels(w http.ResponseWriter, r *http.Request) {
	role := chi.URLParam(r, "role")
	if !llm.ValidRole(role) {
		writeError(w, http.StatusBadRequest, "非法角色，可选: thinker/worker/verifier/helper")
		return
	}
	var req struct {
		ModelIDs []string `json:"model_ids"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(req.ModelIDs) == 0 {
		writeError(w, http.StatusBadRequest, "model_ids 不能为空")
		return
	}
	res, err := s.store.SetRoleModels(r.Context(), role, req.ModelIDs)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	// 角色绑定变更后失效该角色相关适配器缓存
	s.registry.InvalidateAll()
	writeOK(w, map[string]interface{}{"item": res})
}

// ===== 模型连通性测试与默认设置 =====

func (s *Server) HandleTestModelConnection(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	m, err := s.store.GetModel(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if m == nil {
		writeError(w, http.StatusNotFound, "模型不存在")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	adapter, err := llm.NewOpenAICompatible(ctx, m.Vendor, m.APIKey, m.APIEndpoint, m.Name)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "模型适配失败: "+err.Error())
		return
	}

	// 发送简短测试消息
	_, _, err = adapter.Generate(ctx, "你是一个助手。", "回复 OK")
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "连通性测试失败: "+err.Error())
		return
	}

	writeOK(w, map[string]interface{}{"status": "ok", "message": "连通性测试通过"})
}

func (s *Server) HandleSetDefaultModel(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	item, err := s.store.SetDefaultModel(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if item == nil {
		writeError(w, http.StatusNotFound, "模型不存在")
		return
	}
	item.APIKey = database.MaskAPIKey(item.APIKey)
	writeOK(w, map[string]interface{}{"item": item})
}
