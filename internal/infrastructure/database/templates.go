package database

import (
	"context"
	"database/sql"
)

// Template templates 表一行
type Template struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Category  string `json:"category"`
	Content   string `json:"content"`
	IsSystem  bool   `json:"is_system"`
	CreatedAt string `json:"created_at"`
}

func (s *Store) ListTemplates(ctx context.Context, category string) ([]Template, error) {
	q := `SELECT id, name, category, content, is_system, created_at FROM templates`
	args := []interface{}{}
	if category != "" {
		q += ` WHERE category=?`
		args = append(args, category)
	}
	q += ` ORDER BY is_system DESC, created_at DESC`
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Template{}
	for rows.Next() {
		var t Template
		var isSys int
		if err := rows.Scan(&t.ID, &t.Name, &t.Category, &t.Content, &isSys, &t.CreatedAt); err != nil {
			return nil, err
		}
		t.IsSystem = isSys == 1
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) GetTemplate(ctx context.Context, id string) (*Template, error) {
	var t Template
	var isSys int
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, category, content, is_system, created_at FROM templates WHERE id=?`, id).
		Scan(&t.ID, &t.Name, &t.Category, &t.Content, &isSys, &t.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	t.IsSystem = isSys == 1
	return &t, nil
}

func (s *Store) CreateTemplate(ctx context.Context, name, category, content string, isSystem bool) (*Template, error) {
	sys := 0
	if isSystem {
		sys = 1
	}
	t := &Template{ID: newID(), Name: name, Category: category, Content: content, IsSystem: isSystem, CreatedAt: now()}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO templates(id, name, category, content, is_system, created_at) VALUES(?,?,?,?,?,?)`,
		t.ID, t.Name, t.Category, t.Content, sys, t.CreatedAt)
	if err != nil {
		return nil, err
	}
	return t, nil
}

func (s *Store) UpdateTemplate(ctx context.Context, id string, name, category, content *string) (*Template, error) {
	if name == nil && category == nil && content == nil {
		return s.GetTemplate(ctx, id)
	}
	args := []interface{}{}
	setParts := []string{}
	if name != nil {
		setParts = append(setParts, "name=?")
		args = append(args, *name)
	}
	if category != nil {
		setParts = append(setParts, "category=?")
		args = append(args, *category)
	}
	if content != nil {
		setParts = append(setParts, "content=?")
		args = append(args, *content)
	}
	args = append(args, id)
	q := "UPDATE templates SET " + joinStrings(setParts, ", ") + " WHERE id=? AND is_system=0"
	res, err := s.db.ExecContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, nil // 不存在或系统内置不可改
	}
	return s.GetTemplate(ctx, id)
}

func (s *Store) DeleteTemplate(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM templates WHERE id=? AND is_system=0`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return errSystemTemplate
	}
	return nil
}

// SeedSystemTemplates 写入内置系统模板（幂等：按 name 去重）
func (s *Store) SeedSystemTemplates(ctx context.Context, templates []Template) error {
	for _, t := range templates {
		var existing string
		err := s.db.QueryRowContext(ctx, `SELECT id FROM templates WHERE name=? AND is_system=1`, t.Name).Scan(&existing)
		if err == nil {
			// 已存在则更新内容
			_, _ = s.db.ExecContext(ctx, `UPDATE templates SET content=?, category=? WHERE id=?`, t.Content, t.Category, existing)
			continue
		}
		if err != sql.ErrNoRows {
			return err
		}
		sys := 1
		if _, err := s.db.ExecContext(ctx,
			`INSERT INTO templates(id, name, category, content, is_system, created_at) VALUES(?,?,?,?,?,?)`,
			newID(), t.Name, t.Category, t.Content, sys, now()); err != nil {
			return err
		}
	}
	return nil
}
