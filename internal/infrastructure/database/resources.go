package database

import (
	"context"
	"database/sql"
)

// Character characters 表一行（人物卡）
type Character struct {
	ID          string `json:"id"`
	ProjectID   string `json:"project_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	AvatarURL   string `json:"avatar_url"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

func (s *Store) ListCharacters(ctx context.Context, projectID string) ([]Character, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, project_id, name, description, avatar_url, created_at, updated_at FROM characters WHERE project_id=? ORDER BY created_at`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Character{}
	for rows.Next() {
		var c Character
		if err := rows.Scan(&c.ID, &c.ProjectID, &c.Name, &c.Description, &c.AvatarURL, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) CreateCharacter(ctx context.Context, projectID, name, description, avatarURL string) (*Character, error) {
	c := &Character{ID: newID(), ProjectID: projectID, Name: name, Description: description, AvatarURL: avatarURL, CreatedAt: now(), UpdatedAt: now()}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO characters(id, project_id, name, description, avatar_url, created_at, updated_at) VALUES(?,?,?,?,?,?,?)`,
		c.ID, c.ProjectID, c.Name, c.Description, c.AvatarURL, c.CreatedAt, c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (s *Store) UpdateCharacter(ctx context.Context, id string, name, description, avatarURL *string) (*Character, error) {
	if name == nil && description == nil && avatarURL == nil {
		return s.getCharacter(ctx, id)
	}
	args := []interface{}{}
	setParts := []string{}
	if name != nil {
		setParts = append(setParts, "name=?")
		args = append(args, *name)
	}
	if description != nil {
		setParts = append(setParts, "description=?")
		args = append(args, *description)
	}
	if avatarURL != nil {
		setParts = append(setParts, "avatar_url=?")
		args = append(args, *avatarURL)
	}
	setParts = append(setParts, "updated_at=?")
	args = append(args, now())
	args = append(args, id)
	q := "UPDATE characters SET " + joinStrings(setParts, ", ") + " WHERE id=?"
	if _, err := s.db.ExecContext(ctx, q, args...); err != nil {
		return nil, err
	}
	return s.getCharacter(ctx, id)
}

func (s *Store) getCharacter(ctx context.Context, id string) (*Character, error) {
	var c Character
	err := s.db.QueryRowContext(ctx,
		`SELECT id, project_id, name, description, avatar_url, created_at, updated_at FROM characters WHERE id=?`, id).
		Scan(&c.ID, &c.ProjectID, &c.Name, &c.Description, &c.AvatarURL, &c.CreatedAt, &c.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// GetCharacter 按 ID 查询角色（自由写作模式手动勾选出场人物用）
func (s *Store) GetCharacter(ctx context.Context, id string) (*Character, error) {
	return s.getCharacter(ctx, id)
}

func (s *Store) DeleteCharacter(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM characters WHERE id=?`, id)
	return err
}

// CharactersText 拼接项目人物卡为可注入上下文文本
func (s *Store) CharactersText(ctx context.Context, projectID string) (string, error) {
	chars, err := s.ListCharacters(ctx, projectID)
	if err != nil {
		return "", err
	}
	if len(chars) == 0 {
		return "", nil
	}
	var b []byte
	b = append(b, "【人物卡】\n"...)
	for _, c := range chars {
		b = append(b, "· "...)
		b = append(b, c.Name...)
		if c.Description != "" {
			b = append(b, "："...)
			b = append(b, c.Description...)
		}
		b = append(b, '\n')
	}
	return string(b), nil
}

// ===== world_settings =====

// WorldSetting world_settings 表一行
type WorldSetting struct {
	ID        string `json:"id"`
	ProjectID string `json:"project_id"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func (s *Store) ListWorldSettings(ctx context.Context, projectID string) ([]WorldSetting, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, project_id, title, content, created_at, updated_at FROM world_settings WHERE project_id=? ORDER BY created_at`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []WorldSetting{}
	for rows.Next() {
		var w WorldSetting
		if err := rows.Scan(&w.ID, &w.ProjectID, &w.Title, &w.Content, &w.CreatedAt, &w.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

func (s *Store) CreateWorldSetting(ctx context.Context, projectID, title, content string) (*WorldSetting, error) {
	w := &WorldSetting{ID: newID(), ProjectID: projectID, Title: title, Content: content, CreatedAt: now(), UpdatedAt: now()}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO world_settings(id, project_id, title, content, created_at, updated_at) VALUES(?,?,?,?,?,?)`,
		w.ID, w.ProjectID, w.Title, w.Content, w.CreatedAt, w.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return w, nil
}

func (s *Store) UpdateWorldSetting(ctx context.Context, id string, title, content *string) (*WorldSetting, error) {
	if title == nil && content == nil {
		return s.getWorldSetting(ctx, id)
	}
	args := []interface{}{}
	setParts := []string{}
	if title != nil {
		setParts = append(setParts, "title=?")
		args = append(args, *title)
	}
	if content != nil {
		setParts = append(setParts, "content=?")
		args = append(args, *content)
	}
	setParts = append(setParts, "updated_at=?")
	args = append(args, now())
	args = append(args, id)
	q := "UPDATE world_settings SET " + joinStrings(setParts, ", ") + " WHERE id=?"
	if _, err := s.db.ExecContext(ctx, q, args...); err != nil {
		return nil, err
	}
	return s.getWorldSetting(ctx, id)
}

func (s *Store) getWorldSetting(ctx context.Context, id string) (*WorldSetting, error) {
	var w WorldSetting
	err := s.db.QueryRowContext(ctx,
		`SELECT id, project_id, title, content, created_at, updated_at FROM world_settings WHERE id=?`, id).
		Scan(&w.ID, &w.ProjectID, &w.Title, &w.Content, &w.CreatedAt, &w.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &w, nil
}

func (s *Store) DeleteWorldSetting(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM world_settings WHERE id=?`, id)
	return err
}

// WorldSettingsText 拼接项目世界观为可注入上下文文本
func (s *Store) WorldSettingsText(ctx context.Context, projectID string) (string, error) {
	ws, err := s.ListWorldSettings(ctx, projectID)
	if err != nil {
		return "", err
	}
	if len(ws) == 0 {
		return "", nil
	}
	var b []byte
	b = append(b, "【世界观设定】\n"...)
	for _, w := range ws {
		b = append(b, "· "...)
		b = append(b, w.Title...)
		if w.Content != "" {
			b = append(b, "："...)
			b = append(b, w.Content...)
		}
		b = append(b, '\n')
	}
	return string(b), nil
}

// ===== materials =====

// Material materials 表一行
type Material struct {
	ID        string `json:"id"`
	ProjectID string `json:"project_id"`
	Name      string `json:"name"`
	Content   string `json:"content"`
	FileType  string `json:"file_type"`
	CreatedAt string `json:"created_at"`
}

func (s *Store) ListMaterials(ctx context.Context, projectID string) ([]Material, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, project_id, name, content, file_type, created_at FROM materials WHERE project_id=? ORDER BY created_at`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Material{}
	for rows.Next() {
		var m Material
		if err := rows.Scan(&m.ID, &m.ProjectID, &m.Name, &m.Content, &m.FileType, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) CreateMaterial(ctx context.Context, projectID, name, content, fileType string) (*Material, error) {
	m := &Material{ID: newID(), ProjectID: projectID, Name: name, Content: content, FileType: fileType, CreatedAt: now()}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO materials(id, project_id, name, content, file_type, created_at) VALUES(?,?,?,?,?,?)`,
		m.ID, m.ProjectID, m.Name, m.Content, m.FileType, m.CreatedAt)
	if err != nil {
		return nil, err
	}
	return m, nil
}

func (s *Store) DeleteMaterial(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM materials WHERE id=?`, id)
	return err
}
