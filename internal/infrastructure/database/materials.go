package database

import (
	"context"
)

// ============================================================
// materials.go —— 拆书素材库（句式/动作描写/对话标签等表达资产）
// ============================================================

// WritingMaterial 写作素材（复用 rag.Vector 做向量检索）
type WritingMaterial struct {
	ID        string `json:"id"`
	ProjectID string `json:"project_id"`
	Category  string `json:"category"` // 句式/动作描写/对话标签/环境描写/词汇/其他
	Content   string `json:"content"`
	Source    string `json:"source"` // 来源（章节标题/文件/手动）
	Vector    string `json:"vector"`
	CreatedAt string `json:"created_at"`
}

// CreateWritingMaterial 新增素材
func (s *Store) CreateWritingMaterial(ctx context.Context, m WritingMaterial) (*WritingMaterial, error) {
	m.ID = newID()
	m.CreatedAt = now()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO writing_materials(id, project_id, category, content, source, vector, created_at)
		VALUES(?,?,?,?,?,?,?)`,
		m.ID, m.ProjectID, m.Category, m.Content, m.Source, m.Vector, m.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// ListWritingMaterials 列出项目素材（可选类别过滤）
func (s *Store) ListWritingMaterials(ctx context.Context, projectID, category string) ([]WritingMaterial, error) {
	q := `SELECT id, project_id, category, content, source, vector, created_at FROM writing_materials WHERE project_id=?`
	args := []interface{}{projectID}
	if category != "" {
		q += ` AND category=?`
		args = append(args, category)
	}
	q += ` ORDER BY created_at DESC`
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WritingMaterial
	for rows.Next() {
		var m WritingMaterial
		if err := rows.Scan(&m.ID, &m.ProjectID, &m.Category, &m.Content, &m.Source, &m.Vector, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// ListAllWritingMaterialVectors 取全部素材向量（语义检索用，可全量内存比较）
// 上限 500 条：防止素材失控增长时每次生成请求的全表扫描拖慢上下文组装
func (s *Store) ListAllWritingMaterialVectors(ctx context.Context, projectID string) ([]WritingMaterial, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, project_id, category, content, source, vector, created_at
		FROM writing_materials WHERE project_id=? AND vector != '' ORDER BY created_at DESC LIMIT 500`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WritingMaterial
	for rows.Next() {
		var m WritingMaterial
		if err := rows.Scan(&m.ID, &m.ProjectID, &m.Category, &m.Content, &m.Source, &m.Vector, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// UpdateWritingMaterialWithVector 更新素材内容/类别并同步新向量（单条 UPDATE 原子完成）。
// 原 UpdateWritingMaterial 只更新内容/类别，向量需调用方另行 Exec——失败时向量陈旧且与内容非原子。
func (s *Store) UpdateWritingMaterialWithVector(ctx context.Context, id string, category, content *string, vector string) (*WritingMaterial, error) {
	setParts := []string{}
	args := []interface{}{}
	if category != nil {
		setParts = append(setParts, "category=?")
		args = append(args, *category)
	}
	if content != nil {
		setParts = append(setParts, "content=?")
		args = append(args, *content)
	}
	if vector != "" {
		setParts = append(setParts, "vector=?")
		args = append(args, vector)
	}
	if len(setParts) == 0 {
		return s.GetWritingMaterial(ctx, id)
	}
	args = append(args, id)
	if _, err := s.db.ExecContext(ctx, "UPDATE writing_materials SET "+joinSet(setParts)+" WHERE id=?", args...); err != nil {
		return nil, err
	}
	return s.GetWritingMaterial(ctx, id)
}

// GetWritingMaterial 查询单个素材
func (s *Store) GetWritingMaterial(ctx context.Context, id string) (*WritingMaterial, error) {
	var m WritingMaterial
	err := s.db.QueryRowContext(ctx, `
		SELECT id, project_id, category, content, source, vector, created_at
		FROM writing_materials WHERE id=?`, id).
		Scan(&m.ID, &m.ProjectID, &m.Category, &m.Content, &m.Source, &m.Vector, &m.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// DeleteWritingMaterial 删除素材
func (s *Store) DeleteWritingMaterial(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM writing_materials WHERE id=?`, id)
	return err
}

// ClearWritingMaterials 清空项目素材（重新拆书时用）
func (s *Store) ClearWritingMaterials(ctx context.Context, projectID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM writing_materials WHERE project_id=?`, projectID)
	return err
}
