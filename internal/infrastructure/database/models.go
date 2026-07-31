package database

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"fmt"
	"io"
	"sync"
)

var (
	cryptoKey     []byte
	cryptoOnce    sync.Once
)

func getCryptoKey() []byte {
	cryptoOnce.Do(func() {
		hash := sha256.Sum256([]byte("ai-novel-cookie-secret-2024"))
		cryptoKey = hash[:]
	})
	return cryptoKey
}

func encryptCookie(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	block, err := aes.NewCipher(getCryptoKey())
	if err != nil {
		return "", fmt.Errorf("创建加密器失败: %w", err)
	}
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("创建GCM失败: %w", err)
	}
	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("生成nonce失败: %w", err)
	}
	ciphertext := aesGCM.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func decryptCookie(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("解码失败: %w", err)
	}
	block, err := aes.NewCipher(getCryptoKey())
	if err != nil {
		return "", fmt.Errorf("创建解密器失败: %w", err)
	}
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("创建GCM失败: %w", err)
	}
	nonceSize := aesGCM.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("数据格式错误")
	}
	nonce, ciphertextBytes := data[:nonceSize], data[nonceSize:]
	plaintext, err := aesGCM.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return "", fmt.Errorf("解密失败: %w", err)
	}
	return string(plaintext), nil
}

// ModelConfig models 表一行（模型配置）
type ModelConfig struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Vendor        string  `json:"vendor"`
	APIEndpoint   string  `json:"api_endpoint"`
	APIKey        string  `json:"api_key,omitempty"` // 列表接口会脱敏
	Status        string  `json:"status"`
	DailyLimit    int     `json:"daily_limit"`
	IsCustom      int     `json:"is_custom"`
	ContextLimit  int     `json:"context_limit"`
	SupportStream int     `json:"support_stream"`
	IsDefault     int     `json:"is_default"`
	Description   string  `json:"description"`
	Temperature   float64 `json:"temperature"`
	TopP          float64 `json:"top_p"`
	CreatedAt     string  `json:"created_at"`
	// 网页免费AI相关字段
	ModelType     string `json:"model_type"`      // 模型类型：api（标准API）或 web（网页AI）
	Provider      string `json:"provider"`        // 网页AI提供商：kimi-free/doubao-free/qwen-free/deepseek-free/zhipu-free/custom
	Cookie        string `json:"cookie,omitempty"` // 网页Cookie（加密存储，列表脱敏）
	SessionToken  string `json:"session_token,omitempty"` // 会话Token
	RequestURL    string `json:"request_url"`     // 自定义请求URL
	MaxTokens     int    `json:"max_tokens"`      // 单次最大输出字数
	TimeoutSeconds int   `json:"timeout_seconds"` // 超时时间（秒）
	StatusMessage string `json:"status_message"`  // 状态消息（如Cookie过期提示）
}

// MaskAPIKey 脱敏 API Key（仅保留前3后4）
func MaskAPIKey(k string) string {
	if len(k) <= 8 {
		return "****"
	}
	return k[:3] + "****" + k[len(k)-4:]
}

// MaskCookie 脱敏Cookie（仅保留前6后4）
func MaskCookie(c string) string {
	if len(c) <= 10 {
		return "****"
	}
	return c[:6] + "****" + c[len(c)-4:]
}

func (s *Store) ListModels(ctx context.Context) ([]ModelConfig, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, vendor, api_endpoint, api_key, status, daily_limit, COALESCE(is_custom,0), COALESCE(context_limit,4096), COALESCE(support_stream,1), COALESCE(is_default,0), COALESCE(description,''), COALESCE(temperature,0.7), COALESCE(top_p,0.9), created_at,
		COALESCE(model_type,'api'), COALESCE(provider,''), COALESCE(cookie,''), COALESCE(session_token,''), COALESCE(request_url,''), COALESCE(max_tokens,4000), COALESCE(timeout_seconds,300), COALESCE(status_message,'')
		FROM models ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ModelConfig{}
	for rows.Next() {
		var m ModelConfig
		if err := rows.Scan(&m.ID, &m.Name, &m.Vendor, &m.APIEndpoint, &m.APIKey, &m.Status, &m.DailyLimit, &m.IsCustom, &m.ContextLimit, &m.SupportStream, &m.IsDefault, &m.Description, &m.Temperature, &m.TopP, &m.CreatedAt,
			&m.ModelType, &m.Provider, &m.Cookie, &m.SessionToken, &m.RequestURL, &m.MaxTokens, &m.TimeoutSeconds, &m.StatusMessage); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// ListPublicModels 返回脱敏后的可用模型列表（供前端选择）
func (s *Store) ListPublicModels(ctx context.Context) ([]ModelConfig, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, vendor, api_endpoint, '', status, daily_limit, COALESCE(is_custom,0), COALESCE(context_limit,4096), COALESCE(support_stream,1), COALESCE(is_default,0), COALESCE(description,''), COALESCE(temperature,0.7), COALESCE(top_p,0.9), created_at,
		COALESCE(model_type,'api'), COALESCE(provider,''), '', COALESCE(session_token,''), COALESCE(request_url,''), COALESCE(max_tokens,4000), COALESCE(timeout_seconds,300), COALESCE(status_message,'')
		FROM models WHERE status='active' ORDER BY is_default DESC, created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ModelConfig{}
	for rows.Next() {
		var m ModelConfig
		if err := rows.Scan(&m.ID, &m.Name, &m.Vendor, &m.APIEndpoint, &m.APIKey, &m.Status, &m.DailyLimit, &m.IsCustom, &m.ContextLimit, &m.SupportStream, &m.IsDefault, &m.Description, &m.Temperature, &m.TopP, &m.CreatedAt,
			&m.ModelType, &m.Provider, &m.Cookie, &m.SessionToken, &m.RequestURL, &m.MaxTokens, &m.TimeoutSeconds, &m.StatusMessage); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) GetModel(ctx context.Context, id string) (*ModelConfig, error) {
	var m ModelConfig
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, vendor, api_endpoint, api_key, status, daily_limit, COALESCE(is_custom,0), COALESCE(context_limit,4096), COALESCE(support_stream,1), COALESCE(is_default,0), COALESCE(description,''), COALESCE(temperature,0.7), COALESCE(top_p,0.9), created_at,
		COALESCE(model_type,'api'), COALESCE(provider,''), COALESCE(cookie,''), COALESCE(session_token,''), COALESCE(request_url,''), COALESCE(max_tokens,4000), COALESCE(timeout_seconds,300), COALESCE(status_message,'')
		FROM models WHERE id=?`, id).
		Scan(&m.ID, &m.Name, &m.Vendor, &m.APIEndpoint, &m.APIKey, &m.Status, &m.DailyLimit, &m.IsCustom, &m.ContextLimit, &m.SupportStream, &m.IsDefault, &m.Description, &m.Temperature, &m.TopP, &m.CreatedAt,
		&m.ModelType, &m.Provider, &m.Cookie, &m.SessionToken, &m.RequestURL, &m.MaxTokens, &m.TimeoutSeconds, &m.StatusMessage)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	// 解密Cookie
	if m.Cookie != "" {
		decryptedCookie, err := decryptCookie(m.Cookie)
		if err == nil {
			m.Cookie = decryptedCookie
		}
	}
	return &m, nil
}

// GetModelByName 按名称查模型（含 api_key 和 web AI 字段，供适配层使用）
func (s *Store) GetModelByName(ctx context.Context, name string) (*ModelConfig, error) {
	var m ModelConfig
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, vendor, api_endpoint, api_key, status, daily_limit, COALESCE(is_custom,0), COALESCE(context_limit,4096), COALESCE(support_stream,1), COALESCE(is_default,0), COALESCE(description,''), COALESCE(temperature,0.7), COALESCE(top_p,0.9), created_at,
		COALESCE(model_type,'api'), COALESCE(provider,''), COALESCE(cookie,''), COALESCE(session_token,''), COALESCE(request_url,''), COALESCE(max_tokens,4000), COALESCE(timeout_seconds,300), COALESCE(status_message,'')
		FROM models WHERE name=?`, name).
		Scan(&m.ID, &m.Name, &m.Vendor, &m.APIEndpoint, &m.APIKey, &m.Status, &m.DailyLimit, &m.IsCustom, &m.ContextLimit, &m.SupportStream, &m.IsDefault, &m.Description, &m.Temperature, &m.TopP, &m.CreatedAt,
		&m.ModelType, &m.Provider, &m.Cookie, &m.SessionToken, &m.RequestURL, &m.MaxTokens, &m.TimeoutSeconds, &m.StatusMessage)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	// 解密Cookie
	if m.Cookie != "" {
		decryptedCookie, err := decryptCookie(m.Cookie)
		if err == nil {
			m.Cookie = decryptedCookie
		}
	}
	return &m, nil
}

func (s *Store) CreateModel(ctx context.Context, name, vendor, endpoint, apiKey, status string, dailyLimit int) (*ModelConfig, error) {
	return s.CreateModelFull(ctx, name, vendor, endpoint, apiKey, status, dailyLimit, 1, 4096, 1, 0, "", 0.7, 0.9)
}

// CreateModelFull 完整创建模型（含自定义字段）
func (s *Store) CreateModelFull(ctx context.Context, name, vendor, endpoint, apiKey, status string, dailyLimit, isCustom, contextLimit, supportStream, isDefault int, description string, temperature, topP float64) (*ModelConfig, error) {
	if status == "" {
		status = "active"
	}
	m := &ModelConfig{
		ID: newID(), Name: name, Vendor: vendor, APIEndpoint: endpoint, APIKey: apiKey,
		Status: status, DailyLimit: dailyLimit, IsCustom: isCustom, ContextLimit: contextLimit,
		SupportStream: supportStream, IsDefault: isDefault, Description: description, Temperature: temperature, TopP: topP, CreatedAt: now(),
		ModelType: "api", MaxTokens: 4000, TimeoutSeconds: 300,
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO models(id, name, vendor, api_endpoint, api_key, status, daily_limit, is_custom, context_limit, support_stream, is_default, description, temperature, top_p, created_at, model_type, provider, cookie, session_token, request_url, max_tokens, timeout_seconds, status_message) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		m.ID, m.Name, m.Vendor, m.APIEndpoint, m.APIKey, m.Status, m.DailyLimit, m.IsCustom, m.ContextLimit, m.SupportStream, m.IsDefault, m.Description, m.Temperature, m.TopP, m.CreatedAt, m.ModelType, m.Provider, m.Cookie, m.SessionToken, m.RequestURL, m.MaxTokens, m.TimeoutSeconds, m.StatusMessage)
	if err != nil {
		return nil, err
	}
	// 如果设为默认，取消其他模型默认标记
	if isDefault == 1 {
		_, _ = s.db.ExecContext(ctx, `UPDATE models SET is_default=0 WHERE id!=?`, m.ID)
	}
	return m, nil
}

func (s *Store) UpdateModel(ctx context.Context, id string, name, vendor, endpoint, apiKey, status *string, dailyLimit, contextLimit, supportStream, isDefault, isCustom *int, description *string, temperature, topP *float64) (*ModelConfig, error) {
	args := []interface{}{}
	setParts := []string{}
	if name != nil {
		setParts = append(setParts, "name=?")
		args = append(args, *name)
	}
	if vendor != nil {
		setParts = append(setParts, "vendor=?")
		args = append(args, *vendor)
	}
	if endpoint != nil {
		setParts = append(setParts, "api_endpoint=?")
		args = append(args, *endpoint)
	}
	if apiKey != nil {
		setParts = append(setParts, "api_key=?")
		args = append(args, *apiKey)
	}
	if status != nil {
		setParts = append(setParts, "status=?")
		args = append(args, *status)
	}
	if dailyLimit != nil {
		setParts = append(setParts, "daily_limit=?")
		args = append(args, *dailyLimit)
	}
	if contextLimit != nil {
		setParts = append(setParts, "context_limit=?")
		args = append(args, *contextLimit)
	}
	if supportStream != nil {
		setParts = append(setParts, "support_stream=?")
		args = append(args, *supportStream)
	}
	if isDefault != nil {
		setParts = append(setParts, "is_default=?")
		args = append(args, *isDefault)
	}
	if isCustom != nil {
		setParts = append(setParts, "is_custom=?")
		args = append(args, *isCustom)
	}
	if description != nil {
		setParts = append(setParts, "description=?")
		args = append(args, *description)
	}
	if temperature != nil {
		setParts = append(setParts, "temperature=?")
		args = append(args, *temperature)
	}
	if topP != nil {
		setParts = append(setParts, "top_p=?")
		args = append(args, *topP)
	}
	if len(setParts) == 0 {
		return s.GetModel(ctx, id)
	}
	args = append(args, id)
	q := "UPDATE models SET " + joinStrings(setParts, ", ") + " WHERE id=?"
	if _, err := s.db.ExecContext(ctx, q, args...); err != nil {
		return nil, err
	}
	// 如果设为默认，取消其他默认标记
	if isDefault != nil && *isDefault == 1 {
		_, _ = s.db.ExecContext(ctx, `UPDATE models SET is_default=0 WHERE id!=?`, id)
	}
	return s.GetModel(ctx, id)
}

func (s *Store) DeleteModel(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM models WHERE id=?`, id)
	return err
}

// CreateWebAIModel 创建网页AI模型
func (s *Store) CreateWebAIModel(ctx context.Context, name, provider, cookie, sessionToken, requestURL, description string, maxTokens, timeoutSeconds int) (*ModelConfig, error) {
	encryptedCookie, err := encryptCookie(cookie)
	if err != nil {
		return nil, fmt.Errorf("加密Cookie失败: %w", err)
	}

	m := &ModelConfig{
		ID: newID(), Name: name, Vendor: provider, APIEndpoint: requestURL,
		Status: "active", DailyLimit: 0, IsCustom: 1, ContextLimit: 4096,
		SupportStream: 1, IsDefault: 0, Description: description, Temperature: 0.7, TopP: 0.9, CreatedAt: now(),
		ModelType: "web", Provider: provider, Cookie: encryptedCookie, SessionToken: sessionToken,
		RequestURL: requestURL, MaxTokens: maxTokens, TimeoutSeconds: timeoutSeconds, StatusMessage: "",
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO models(id, name, vendor, api_endpoint, api_key, status, daily_limit, is_custom, context_limit, support_stream, is_default, description, temperature, top_p, created_at, model_type, provider, cookie, session_token, request_url, max_tokens, timeout_seconds, status_message) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		m.ID, m.Name, m.Vendor, m.APIEndpoint, m.APIKey, m.Status, m.DailyLimit, m.IsCustom, m.ContextLimit, m.SupportStream, m.IsDefault, m.Description, m.Temperature, m.TopP, m.CreatedAt, m.ModelType, m.Provider, m.Cookie, m.SessionToken, m.RequestURL, m.MaxTokens, m.TimeoutSeconds, m.StatusMessage)
	if err != nil {
		return nil, err
	}
	return m, nil
}

// UpdateWebAIModel 更新网页AI模型配置
func (s *Store) UpdateWebAIModel(ctx context.Context, id string, name, provider, cookie, sessionToken, requestURL, description *string, maxTokens, timeoutSeconds *int, statusMessage *string) (*ModelConfig, error) {
	args := []interface{}{}
	setParts := []string{}
	if name != nil {
		setParts = append(setParts, "name=?")
		args = append(args, *name)
	}
	if provider != nil {
		setParts = append(setParts, "provider=?")
		args = append(args, *provider)
		setParts = append(setParts, "vendor=?")
		args = append(args, *provider)
	}
	if cookie != nil {
		encryptedCookie, err := encryptCookie(*cookie)
		if err != nil {
			return nil, fmt.Errorf("加密Cookie失败: %w", err)
		}
		setParts = append(setParts, "cookie=?")
		args = append(args, encryptedCookie)
	}
	if sessionToken != nil {
		setParts = append(setParts, "session_token=?")
		args = append(args, *sessionToken)
	}
	if requestURL != nil {
		setParts = append(setParts, "request_url=?")
		args = append(args, *requestURL)
		setParts = append(setParts, "api_endpoint=?")
		args = append(args, *requestURL)
	}
	if description != nil {
		setParts = append(setParts, "description=?")
		args = append(args, *description)
	}
	if maxTokens != nil {
		setParts = append(setParts, "max_tokens=?")
		args = append(args, *maxTokens)
	}
	if timeoutSeconds != nil {
		setParts = append(setParts, "timeout_seconds=?")
		args = append(args, *timeoutSeconds)
	}
	if statusMessage != nil {
		setParts = append(setParts, "status_message=?")
		args = append(args, *statusMessage)
	}
	if len(setParts) == 0 {
		return s.GetModel(ctx, id)
	}
	args = append(args, id)
	q := "UPDATE models SET " + joinStrings(setParts, ", ") + " WHERE id=?"
	if _, err := s.db.ExecContext(ctx, q, args...); err != nil {
		return nil, err
	}
	return s.GetModel(ctx, id)
}

// ===== role_models =====

// RoleModelBinding role_models 表一行（角色绑定模型优先级）
type RoleModelBinding struct {
	ID       string `json:"id"`
	Role     string `json:"role"`
	ModelID  string `json:"model_id"`
	Priority int    `json:"priority"`
}

// RoleModelsResult 角色绑定详情（含模型名）
type RoleModelsResult struct {
	Role    string             `json:"role"`
	Models  []RoleModelSummary `json:"models"`
}

type RoleModelSummary struct {
	ModelID  string `json:"model_id"`
	Name     string `json:"name"`
	Vendor   string `json:"vendor"`
	Priority int    `json:"priority"`
}

// GetRoleModels 获取某角色绑定的模型列表（按优先级）
func (s *Store) GetRoleModels(ctx context.Context, role string) (*RoleModelsResult, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT rm.id, rm.role, rm.model_id, rm.priority, m.name, m.vendor
		 FROM role_models rm JOIN models m ON m.id=rm.model_id
		 WHERE rm.role=? ORDER BY rm.priority ASC`, role)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	res := &RoleModelsResult{Role: role, Models: []RoleModelSummary{}}
	for rows.Next() {
		var b RoleModelBinding
		var sum RoleModelSummary
		if err := rows.Scan(&b.ID, &b.Role, &b.ModelID, &b.Priority, &sum.Name, &sum.Vendor); err != nil {
			return nil, err
		}
		sum.ModelID = b.ModelID
		sum.Priority = b.Priority
		res.Models = append(res.Models, sum)
	}
	return res, rows.Err()
}

// SetRoleModels 覆盖式设置某角色的模型优先级列表（事务内清空再重建）
func (s *Store) SetRoleModels(ctx context.Context, role string, modelIDs []string) (*RoleModelsResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM role_models WHERE role=?`, role); err != nil {
		return nil, fmt.Errorf("清空角色绑定失败: %w", err)
	}
	for priority, mid := range modelIDs {
		var exists string
		if err := tx.QueryRowContext(ctx, `SELECT id FROM models WHERE id=?`, mid).Scan(&exists); err != nil {
			if err == sql.ErrNoRows {
				return nil, fmt.Errorf("模型 %s 不存在", mid)
			}
			return nil, err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO role_models(id, role, model_id, priority) VALUES(?,?,?,?)`,
			role+"-"+mid, role, mid, priority); err != nil {
			return nil, fmt.Errorf("写入角色绑定失败: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetRoleModels(ctx, role)
}

// RoleModelNames 返回某角色绑定的模型名称列表（按优先级，供调度层直接使用）
func (s *Store) RoleModelNames(ctx context.Context, role string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT m.name FROM role_models rm JOIN models m ON m.id=rm.model_id
		 WHERE rm.role=? AND m.status='active' ORDER BY rm.priority ASC`, role)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	names := []string{}
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		names = append(names, n)
	}
	return names, rows.Err()
}

// GetRolesBoundToModel 返回绑定到指定模型的所有角色（用于删除模型前的安全检查）
func (s *Store) GetRolesBoundToModel(ctx context.Context, modelName string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT rm.role FROM role_models rm JOIN models m ON m.id=rm.model_id WHERE m.name=?`, modelName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	roles := []string{}
	for rows.Next() {
		var r string
		if err := rows.Scan(&r); err != nil {
			return nil, err
		}
		roles = append(roles, r)
	}
	return roles, rows.Err()
}

// SetDefaultModel 将指定模型设为默认（取消其他默认标记）
func (s *Store) SetDefaultModel(ctx context.Context, id string) (*ModelConfig, error) {
	m, err := s.GetModel(ctx, id)
	if err != nil || m == nil {
		return nil, err
	}
	if m == nil {
		return nil, nil
	}
	one := 1
	return s.UpdateModel(ctx, id, nil, nil, nil, nil, nil, nil, nil, nil, &one, nil, nil, nil, nil)
}

// ListCustomModels 列出用户自定义模型（is_custom=1）
func (s *Store) ListCustomModels(ctx context.Context) ([]ModelConfig, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, vendor, api_endpoint, '', status, daily_limit, COALESCE(is_custom,0), COALESCE(context_limit,4096), COALESCE(support_stream,1), COALESCE(is_default,0), COALESCE(description,''), COALESCE(temperature,0.7), COALESCE(top_p,0.9), created_at FROM models WHERE is_custom=1 ORDER BY is_default DESC, created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ModelConfig{}
	for rows.Next() {
		var m ModelConfig
		if err := rows.Scan(&m.ID, &m.Name, &m.Vendor, &m.APIEndpoint, &m.APIKey, &m.Status, &m.DailyLimit, &m.IsCustom, &m.ContextLimit, &m.SupportStream, &m.IsDefault, &m.Description, &m.Temperature, &m.TopP, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
