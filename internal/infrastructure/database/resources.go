package database

import (
	"context"
	"database/sql"
	"fmt"
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

// ===== factions（势力，2026-08-05 转型纯作家辅助新增） =====

// Faction 势力/组织实体
type Faction struct {
	ID          string `json:"id"`
	ProjectID   string `json:"project_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Leader      string `json:"leader"`
	Members     string `json:"members"`
	Relations   string `json:"relations"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

func (s *Store) ListFactions(ctx context.Context, projectID string) ([]Faction, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, project_id, name, description, leader, members, relations, created_at, updated_at FROM factions WHERE project_id=? ORDER BY created_at`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Faction{}
	for rows.Next() {
		var f Faction
		if err := rows.Scan(&f.ID, &f.ProjectID, &f.Name, &f.Description, &f.Leader, &f.Members, &f.Relations, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (s *Store) CreateFaction(ctx context.Context, projectID, name, description, leader, members, relations string) (*Faction, error) {
	f := &Faction{ID: newID(), ProjectID: projectID, Name: name, Description: description, Leader: leader, Members: members, Relations: relations, CreatedAt: now(), UpdatedAt: now()}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO factions (id, project_id, name, description, leader, members, relations, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?)`,
		f.ID, f.ProjectID, f.Name, f.Description, f.Leader, f.Members, f.Relations, f.CreatedAt, f.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return f, nil
}

func (s *Store) UpdateFaction(ctx context.Context, id, name, description, leader, members, relations string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE factions SET name=?, description=?, leader=?, members=?, relations=?, updated_at=? WHERE id=?`,
		name, description, leader, members, relations, now(), id)
	return err
}

func (s *Store) DeleteFaction(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM factions WHERE id=?`, id)
	return err
}

// ===== locations（地点，2026-08-05 转型纯作家辅助新增） =====

// Location 地点实体
type Location struct {
	ID          string `json:"id"`
	ProjectID   string `json:"project_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        string `json:"type"`
	Related     string `json:"related"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

func (s *Store) ListLocations(ctx context.Context, projectID string) ([]Location, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, project_id, name, description, type, related, created_at, updated_at FROM locations WHERE project_id=? ORDER BY created_at`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Location{}
	for rows.Next() {
		var l Location
		if err := rows.Scan(&l.ID, &l.ProjectID, &l.Name, &l.Description, &l.Type, &l.Related, &l.CreatedAt, &l.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func (s *Store) CreateLocation(ctx context.Context, projectID, name, description, typ, related string) (*Location, error) {
	l := &Location{ID: newID(), ProjectID: projectID, Name: name, Description: description, Type: typ, Related: related, CreatedAt: now(), UpdatedAt: now()}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO locations (id, project_id, name, description, type, related, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?)`,
		l.ID, l.ProjectID, l.Name, l.Description, l.Type, l.Related, l.CreatedAt, l.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return l, nil
}

func (s *Store) UpdateLocation(ctx context.Context, id, name, description, typ, related string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE locations SET name=?, description=?, type=?, related=?, updated_at=? WHERE id=?`,
		name, description, typ, related, now(), id)
	return err
}

func (s *Store) DeleteLocation(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM locations WHERE id=?`, id)
	return err
}

// ===== timeline（时间线事件，2026-08-05 补齐 CRUD） =====

// TimelineEvent 时间线事件
type TimelineEvent struct {
	ID         string `json:"id"`
	ProjectID  string `json:"project_id"`
	ChapterID  string `json:"chapter_id"`
	Event      string `json:"event"`
	EventTime  string `json:"event_time"`
	Characters string `json:"characters"`
	CreatedAt  string `json:"created_at"`
}

func (s *Store) ListTimelineEvents(ctx context.Context, projectID string) ([]TimelineEvent, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, project_id, chapter_id, event, event_time, characters, created_at FROM timeline WHERE project_id=? ORDER BY event_time, created_at`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []TimelineEvent{}
	for rows.Next() {
		var t TimelineEvent
		if err := rows.Scan(&t.ID, &t.ProjectID, &t.ChapterID, &t.Event, &t.EventTime, &t.Characters, &t.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) CreateTimelineEvent(ctx context.Context, projectID, chapterID, event, eventTime, characters string) (*TimelineEvent, error) {
	t := &TimelineEvent{ID: newID(), ProjectID: projectID, ChapterID: chapterID, Event: event, EventTime: eventTime, Characters: characters, CreatedAt: now()}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO timeline (id, project_id, chapter_id, event, event_time, characters, created_at) VALUES (?,?,?,?,?,?,?)`,
		t.ID, t.ProjectID, t.ChapterID, t.Event, t.EventTime, t.Characters, t.CreatedAt)
	if err != nil {
		return nil, err
	}
	return t, nil
}

func (s *Store) UpdateTimelineEvent(ctx context.Context, id, chapterID, event, eventTime, characters string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE timeline SET chapter_id=?, event=?, event_time=?, characters=? WHERE id=?`,
		chapterID, event, eventTime, characters, id)
	return err
}

func (s *Store) DeleteTimelineEvent(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM timeline WHERE id=?`, id)
	return err
}

// ===== relations（人物关系，2026-08-05 转型纯作家辅助新增） =====

// Relation 人物关系（char_a ↔ char_b，存角色名）
type Relation struct {
	ID        string `json:"id"`
	ProjectID string `json:"project_id"`
	CharA     string `json:"char_a"`
	CharB     string `json:"char_b"`
	Relation  string `json:"relation"`
	Note      string `json:"note"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func (s *Store) ListRelations(ctx context.Context, projectID string) ([]Relation, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, project_id, char_a, char_b, relation, note, created_at, updated_at FROM relations WHERE project_id=? ORDER BY created_at`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Relation{}
	for rows.Next() {
		var r Relation
		if err := rows.Scan(&r.ID, &r.ProjectID, &r.CharA, &r.CharB, &r.Relation, &r.Note, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) CreateRelation(ctx context.Context, projectID, charA, charB, relation, note string) (*Relation, error) {
	r := &Relation{ID: newID(), ProjectID: projectID, CharA: charA, CharB: charB, Relation: relation, Note: note, CreatedAt: now(), UpdatedAt: now()}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO relations (id, project_id, char_a, char_b, relation, note, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?)`,
		r.ID, r.ProjectID, r.CharA, r.CharB, r.Relation, r.Note, r.CreatedAt, r.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (s *Store) UpdateRelation(ctx context.Context, id, charA, charB, relation, note string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE relations SET char_a=?, char_b=?, relation=?, note=?, updated_at=? WHERE id=?`,
		charA, charB, relation, note, now(), id)
	return err
}

func (s *Store) DeleteRelation(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM relations WHERE id=?`, id)
	return err
}

// ===== annotations（批注/高亮，2026-08-05 阅读工具新增，纯文本偏移锚定） =====

// Annotation 批注/高亮：锚定章节纯文本偏移 [start, end)
type Annotation struct {
	ID           string `json:"id"`
	ProjectID    string `json:"project_id"`
	ChapterID    string `json:"chapter_id"`
	Start        int    `json:"start"`
	End          int    `json:"end"`
	SelectedText string `json:"selected_text"`
	Type         string `json:"type"` // highlight | comment
	Color        string `json:"color"`
	Note         string `json:"note"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

func (s *Store) ListAnnotations(ctx context.Context, chapterID string) ([]Annotation, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, project_id, chapter_id, start, end, selected_text, type, color, note, created_at, updated_at FROM annotations WHERE chapter_id=? ORDER BY start`, chapterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Annotation{}
	for rows.Next() {
		var a Annotation
		if err := rows.Scan(&a.ID, &a.ProjectID, &a.ChapterID, &a.Start, &a.End, &a.SelectedText, &a.Type, &a.Color, &a.Note, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) CreateAnnotation(ctx context.Context, a *Annotation) (*Annotation, error) {
	a.ID = newID()
	a.CreatedAt = now()
	a.UpdatedAt = now()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO annotations (id, project_id, chapter_id, start, end, selected_text, type, color, note, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		a.ID, a.ProjectID, a.ChapterID, a.Start, a.End, a.SelectedText, a.Type, a.Color, a.Note, a.CreatedAt, a.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return a, nil
}

func (s *Store) UpdateAnnotation(ctx context.Context, id, note, color string, start, end *int) error {
	// 快照对齐：start/end 为 nil 时不改偏移（普通改 note/color 场景）
	if start == nil && end == nil {
		_, err := s.db.ExecContext(ctx, `UPDATE annotations SET note=?, color=?, updated_at=? WHERE id=?`, note, color, now(), id)
		return err
	}
	cur := &Annotation{}
	err := s.db.QueryRowContext(ctx,
		`SELECT start, end FROM annotations WHERE id=?`, id).Scan(&cur.Start, &cur.End)
	if err != nil {
		return err
	}
	ns, ne := cur.Start, cur.End
	if start != nil {
		ns = *start
	}
	if end != nil {
		ne = *end
	}
	if ns < 0 || ne < ns {
		return fmt.Errorf("invalid annotation offset: start=%d end=%d", ns, ne)
	}
	_, err = s.db.ExecContext(ctx,
		`UPDATE annotations SET note=?, color=?, start=?, end=?, updated_at=? WHERE id=?`, note, color, ns, ne, now(), id)
	return err
}

func (s *Store) DeleteAnnotation(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM annotations WHERE id=?`, id)
	return err
}

// ===== reading_progress（阅读进度，2026-08-05 阅读工具新增） =====

// ReadingProgress 阅读进度：每项目一条，记录当前读到的章节
type ReadingProgress struct {
	ProjectID string `json:"project_id"`
	ChapterID string `json:"chapter_id"`
	ScrollPct int    `json:"scroll_pct"`
	UpdatedAt string `json:"updated_at"`
}

func (s *Store) GetReadingProgress(ctx context.Context, projectID string) (*ReadingProgress, error) {
	var rp ReadingProgress
	err := s.db.QueryRowContext(ctx,
		`SELECT project_id, chapter_id, scroll_pct, updated_at FROM reading_progress WHERE project_id=?`, projectID).
		Scan(&rp.ProjectID, &rp.ChapterID, &rp.ScrollPct, &rp.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &rp, nil
}

func (s *Store) SetReadingProgress(ctx context.Context, projectID, chapterID string, scrollPct int) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO reading_progress (project_id, chapter_id, scroll_pct, updated_at) VALUES (?,?,?,?)
         ON CONFLICT(project_id) DO UPDATE SET chapter_id=excluded.chapter_id, scroll_pct=excluded.scroll_pct, updated_at=excluded.updated_at`,
		projectID, chapterID, scrollPct, now())
	return err
}
