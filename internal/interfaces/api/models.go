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
	if m == nil {
		writeError(w, http.StatusNotFound, "模型不存在")
		return
	}
	// 检查该模型是否被任何角色绑定，防止静默清空导致AI生成失败
	boundRoles, _ := s.store.GetRolesBoundToModel(r.Context(), m.Name)
	if len(boundRoles) > 0 {
		writeError(w, http.StatusConflict, "该模型已被绑定到角色「"+strings.Join(boundRoles, "、")+"」的流水线中，请先在角色模型分配中移除绑定后再删除")
		return
	}
	if err := s.store.DeleteModel(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.registry.Invalidate(m.Name)
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
	// 允许空数组（清空角色绑定）
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

	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second) // 网页AI首次回复可能较慢（豆包/Kimi SSE 流）
	defer cancel()

	var adapter llm.ModelAdapter
	if m.ModelType == "web" {
		cookie, err := llm.DecryptCookie(m.Cookie)
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, "Cookie 已失效或格式错误，请重新粘贴有效的 Cookie")
			return
		}
		webAdapter, err := llm.NewWebAIAdapter(m.Name, m.Provider, cookie, m.SessionToken, m.RequestURL, m.MaxTokens, m.TimeoutSeconds)
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, "无法连接「"+m.Provider+"」网页AI，请检查 Cookie 是否仍然有效")
			return
		}
		adapter = webAdapter
	} else {
		a, err := llm.NewOpenAICompatible(ctx, m.Vendor, m.APIKey, m.APIEndpoint, m.Name, m.MaxTokens, m.Temperature, m.TopP)
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, "API 密钥或地址配置有误，请在模型管理中检查并重新填写")
			return
		}
		adapter = a
	}

	_, _, err = adapter.Generate(ctx, "你是一个助手。", "回复 OK")
	if err != nil {
		if m.ModelType == "web" {
			errMsg := "连接失败，请确认：1) Cookie 未过期 2) 网络可以访问「" + m.Provider + "」"
			_, _ = s.store.UpdateWebAIModel(r.Context(), id, nil, nil, nil, nil, nil, nil, nil, nil, &errMsg)
			writeError(w, http.StatusServiceUnavailable, errMsg)
		} else {
			writeError(w, http.StatusServiceUnavailable, "连接失败，请检查：1) API Key 是否正确 2) 网络是否正常 3) 账户是否有余额")
		}
		return
	}

	if m.ModelType == "web" {
		statusMsg := ""
		_, _ = s.store.UpdateWebAIModel(r.Context(), id, nil, nil, nil, nil, nil, nil, nil, nil, &statusMsg)
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

// ===== 网页AI模型管理 =====

func (s *Server) HandleListWebAIProviders(w http.ResponseWriter, r *http.Request) {
	providers := llm.ListProviders()
	result := make(map[string]interface{})
	for k, v := range providers {
		result[k] = map[string]interface{}{
			"name":    v.Name,
			"baseURL": v.BaseURL,
		}
	}
	writeOK(w, map[string]interface{}{"providers": result})
}

func (s *Server) HandleCreateWebAIModel(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name           string `json:"name"`
		Provider       string `json:"provider"`
		Cookie         string `json:"cookie"`
		SessionToken   string `json:"session_token"`
		RequestURL     string `json:"request_url"`
		Description    string `json:"description"`
		MaxTokens      int    `json:"max_tokens"`
		TimeoutSeconds int    `json:"timeout_seconds"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "模型名称必填")
		return
	}
	if req.Provider == "" && req.RequestURL == "" {
		writeError(w, http.StatusBadRequest, "请选择提供商或填写自定义请求URL")
		return
	}
	if req.MaxTokens <= 0 {
		req.MaxTokens = 4000
	}
	if req.TimeoutSeconds <= 0 {
		req.TimeoutSeconds = 300
	}

	item, err := s.store.CreateWebAIModel(r.Context(), req.Name, req.Provider, req.Cookie, req.SessionToken, req.RequestURL, req.Description, req.MaxTokens, req.TimeoutSeconds)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	item.Cookie = database.MaskCookie(item.Cookie)
	writeCreated(w, map[string]interface{}{"item": item})
}

func (s *Server) HandleUpdateWebAIModel(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		Name           *string `json:"name"`
		Provider       *string `json:"provider"`
		Cookie         *string `json:"cookie"`
		SessionToken   *string `json:"session_token"`
		RequestURL     *string `json:"request_url"`
		Description    *string `json:"description"`
		MaxTokens      *int    `json:"max_tokens"`
		TimeoutSeconds *int    `json:"timeout_seconds"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	item, err := s.store.UpdateWebAIModel(r.Context(), id, req.Name, req.Provider, req.Cookie, req.SessionToken, req.RequestURL, req.Description, req.MaxTokens, req.TimeoutSeconds, nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if item == nil {
		writeError(w, http.StatusNotFound, "模型不存在")
		return
	}
	s.registry.Invalidate(item.Name)
	item.Cookie = database.MaskCookie(item.Cookie)
	writeOK(w, map[string]interface{}{"item": item})
}

func (s *Server) HandleTestWebAIConnection(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ModelID      string `json:"model_id"`
		Provider     string `json:"provider"`
		Cookie       string `json:"cookie"`
		SessionToken string `json:"session_token"`
		RequestURL   string `json:"request_url"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// 优先按模型 ID 从 DB 取真实凭证（前端拿到的 cookie 是加密密文，不能直接用）
	if req.ModelID != "" {
		item, err := s.store.GetModel(r.Context(), req.ModelID)
		if err == nil && item != nil {
			req.Provider = item.Provider
			req.Cookie = item.Cookie        // 解密后的真实 cookie
			req.SessionToken = item.SessionToken
			if item.RequestURL != "" {
				req.RequestURL = item.RequestURL
			}
		}
	}

	if req.Provider == "" && req.RequestURL == "" {
		writeError(w, http.StatusBadRequest, "请选择提供商或填写自定义请求URL")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	adapter, err := llm.NewWebAIAdapter("test", req.Provider, req.Cookie, req.SessionToken, req.RequestURL, 4000, 300)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "无法连接「"+req.Provider+"」网页AI，请检查 Cookie 是否正确、网络是否通畅")
		return
	}

	_, _, err = adapter.Generate(ctx, "你是一个助手。", "回复 OK")
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "连接失败，可能原因：1) Cookie 已过期 2) 网页AI服务暂时不可用 3) 网络不通。请稍后重试或更换其他免费模型")
		return
	}

	writeOK(w, map[string]interface{}{"status": "ok", "message": "连通性测试通过"})
}
