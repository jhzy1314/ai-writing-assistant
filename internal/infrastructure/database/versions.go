package database

import (
	"context"
	"database/sql"
)

// Version documents 表一行（稿件版本快照）
type Version struct {
	ID        string `json:"id"`
	ProjectID string `json:"project_id"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	Version   int    `json:"version"`
	CreatedAt string `json:"created_at"`
}

// SaveVersion 保存新版本，version 自动递增
func (s *Store) SaveVersion(ctx context.Context, projectID, title, content string) (*Version, error) {
	var maxVer int
	_ = s.db.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(version),0) FROM documents WHERE project_id=?`, projectID).Scan(&maxVer)
	v := &Version{
		ID:        newID(),
		ProjectID: projectID,
		Title:     title,
		Content:   content,
		Version:   maxVer + 1,
		CreatedAt: now(),
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO documents(id, project_id, title, content, version, created_at) VALUES(?,?,?,?,?,?)`,
		v.ID, v.ProjectID, v.Title, v.Content, v.Version, v.CreatedAt)
	if err != nil {
		return nil, err
	}
	// 更新项目时间戳
	_, _ = s.db.ExecContext(ctx, `UPDATE projects SET updated_at=? WHERE id=?`, now(), projectID)
	return v, nil
}

// ListVersions 列出项目版本（按版本号倒序）
func (s *Store) ListVersions(ctx context.Context, projectID string) ([]Version, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, project_id, title, content, version, created_at FROM documents
		 WHERE project_id=? ORDER BY version DESC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Version{}
	for rows.Next() {
		var v Version
		if err := rows.Scan(&v.ID, &v.ProjectID, &v.Title, &v.Content, &v.Version, &v.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// GetVersion 按 ID 查版本
func (s *Store) GetVersion(ctx context.Context, id string) (*Version, error) {
	var v Version
	err := s.db.QueryRowContext(ctx,
		`SELECT id, project_id, title, content, version, created_at FROM documents WHERE id=?`, id).
		Scan(&v.ID, &v.ProjectID, &v.Title, &v.Content, &v.Version, &v.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// LatestVersion 获取项目最新版本（无则返回 nil）
func (s *Store) LatestVersion(ctx context.Context, projectID string) (*Version, error) {
	var v Version
	err := s.db.QueryRowContext(ctx,
		`SELECT id, project_id, title, content, version, created_at FROM documents
		 WHERE project_id=? ORDER BY version DESC LIMIT 1`, projectID).
		Scan(&v.ID, &v.ProjectID, &v.Title, &v.Content, &v.Version, &v.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &v, nil
}
