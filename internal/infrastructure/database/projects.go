package database

import (
	"context"
	"database/sql"
	"fmt"
)

// Project projects 表一行
type Project struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	Outline   string `json:"outline"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// CreateProject 新建项目
func (s *Store) CreateProject(ctx context.Context, name, typ string) (*Project, error) {
	exists, err := s.ProjectNameExists(ctx, name)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, fmt.Errorf("已存在同名项目「%s」，请更换名称", name)
	}
	p := &Project{ID: newID(), Name: name, Type: typ, Outline: "", CreatedAt: now(), UpdatedAt: now()}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO projects(id, name, type, outline, created_at, updated_at) VALUES(?,?,?,?,?,?)`,
		p.ID, p.Name, p.Type, p.Outline, p.CreatedAt, p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return p, nil
}

// ProjectNameExists 检查项目名称是否已存在
func (s *Store) ProjectNameExists(ctx context.Context, name string) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM projects WHERE name=?`, name).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// ListProjects 列出全部项目（按更新时间倒序）
func (s *Store) ListProjects(ctx context.Context) ([]Project, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, type, outline, created_at, updated_at FROM projects ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Project{}
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.Name, &p.Type, &p.Outline, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// GetProject 按 ID 查项目
func (s *Store) GetProject(ctx context.Context, id string) (*Project, error) {
	var p Project
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, type, outline, created_at, updated_at FROM projects WHERE id=?`, id).
		Scan(&p.ID, &p.Name, &p.Type, &p.Outline, &p.CreatedAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// UpdateProject 更新项目（nil 字段表示不更新）
func (s *Store) UpdateProject(ctx context.Context, id string, name *string, typ *string, outline *string) (*Project, error) {
	if name == nil && typ == nil && outline == nil {
		return s.GetProject(ctx, id)
	}
	setParts := []string{}
	args := []interface{}{}
	if name != nil {
		setParts = append(setParts, "name=?")
		args = append(args, *name)
	}
	if typ != nil {
		setParts = append(setParts, "type=?")
		args = append(args, *typ)
	}
	if outline != nil {
		setParts = append(setParts, "outline=?")
		args = append(args, *outline)
	}
	setParts = append(setParts, "updated_at=?")
	args = append(args, now())
	args = append(args, id)
	q := fmt.Sprintf("UPDATE projects SET %s WHERE id=?", joinStrings(setParts, ", "))
	res, err := s.db.ExecContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, nil
	}
	return s.GetProject(ctx, id)
}

// DeleteProject 删除项目（级联删除子表）
func (s *Store) DeleteProject(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM projects WHERE id=?`, id)
	return err
}

func joinStrings(ss []string, sep string) string {
	out := ""
	for i, s := range ss {
		if i > 0 {
			out += sep
		}
		out += s
	}
	return out
}
