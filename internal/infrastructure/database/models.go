package database

import (
	"context"
	"database/sql"
	"fmt"
)

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
}

// MaskAPIKey 脱敏 API Key（仅保留前3后4）
func MaskAPIKey(k string) string {
	if len(k) <= 8 {
		return "****"
	}
	return k[:3] + "****" + k[len(k)-4:]
}

func (s *Store) ListModels(ctx context.Context) ([]ModelConfig, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, vendor, api_endpoint, api_key, status, daily_limit, COALESCE(is_custom,0), COALESCE(context_limit,4096), COALESCE(support_stream,1), COALESCE(is_default,0), COALESCE(description,''), COALESCE(temperature,0.7), COALESCE(top_p,0.9), created_at FROM models ORDER BY created_at`)
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

// ListPublicModels 返回脱敏后的可用模型列表（供前端选择）
func (s *Store) ListPublicModels(ctx context.Context) ([]ModelConfig, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, vendor, api_endpoint, '', status, daily_limit, COALESCE(is_custom,0), COALESCE(context_limit,4096), COALESCE(support_stream,1), COALESCE(is_default,0), COALESCE(description,''), COALESCE(temperature,0.7), COALESCE(top_p,0.9), created_at FROM models WHERE status='active' ORDER BY is_default DESC, created_at`)
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

func (s *Store) GetModel(ctx context.Context, id string) (*ModelConfig, error) {
	var m ModelConfig
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, vendor, api_endpoint, api_key, status, daily_limit, COALESCE(is_custom,0), COALESCE(context_limit,4096), COALESCE(support_stream,1), COALESCE(is_default,0), COALESCE(description,''), COALESCE(temperature,0.7), COALESCE(top_p,0.9), created_at FROM models WHERE id=?`, id).
		Scan(&m.ID, &m.Name, &m.Vendor, &m.APIEndpoint, &m.APIKey, &m.Status, &m.DailyLimit, &m.IsCustom, &m.ContextLimit, &m.SupportStream, &m.IsDefault, &m.Description, &m.Temperature, &m.TopP, &m.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// GetModelByName 按名称查模型（含 api_key，供适配层使用）
func (s *Store) GetModelByName(ctx context.Context, name string) (*ModelConfig, error) {
	var m ModelConfig
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, vendor, api_endpoint, api_key, status, daily_limit, COALESCE(is_custom,0), COALESCE(context_limit,4096), COALESCE(support_stream,1), COALESCE(is_default,0), COALESCE(description,''), COALESCE(temperature,0.7), COALESCE(top_p,0.9), created_at FROM models WHERE name=?`, name).
		Scan(&m.ID, &m.Name, &m.Vendor, &m.APIEndpoint, &m.APIKey, &m.Status, &m.DailyLimit, &m.IsCustom, &m.ContextLimit, &m.SupportStream, &m.IsDefault, &m.Description, &m.Temperature, &m.TopP, &m.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
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
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO models(id, name, vendor, api_endpoint, api_key, status, daily_limit, is_custom, context_limit, support_stream, is_default, description, temperature, top_p, created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		m.ID, m.Name, m.Vendor, m.APIEndpoint, m.APIKey, m.Status, m.DailyLimit, m.IsCustom, m.ContextLimit, m.SupportStream, m.IsDefault, m.Description, m.Temperature, m.TopP, m.CreatedAt)
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
