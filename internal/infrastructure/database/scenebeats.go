package database

import (
	"context"
)

// ============================================================
// scenebeats.go —— 场景节拍/细纲（章节下挂场景卡：标题+摘要+节拍）
// ============================================================

// SceneBeat 场景节拍
type SceneBeat struct {
	ID        string `json:"id"`
	ProjectID string `json:"project_id"`
	ChapterID string `json:"chapter_id"`
	Title     string `json:"title"`
	Summary   string `json:"summary"`
	SortOrder int    `json:"sort_order"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// CreateSceneBeat 新建场景卡
// sort_order 缺省时用单条 INSERT + 子查询 MAX+1 计算：
// 单语句原子执行，避免「读 MAX 再插入」两段式在并发下拿到相同 sort_order
func (s *Store) CreateSceneBeat(ctx context.Context, b SceneBeat) (*SceneBeat, error) {
	b.ID = newID()
	b.CreatedAt = now()
	b.UpdatedAt = now()
	if b.SortOrder == 0 {
		_, err := s.db.ExecContext(ctx, `
			INSERT INTO scene_beats(id, project_id, chapter_id, title, summary, sort_order, created_at, updated_at)
			VALUES(?,?,?,?,?, COALESCE((SELECT MAX(sort_order) FROM scene_beats WHERE chapter_id=?),0)+1, ?, ?)`,
			b.ID, b.ProjectID, b.ChapterID, b.Title, b.Summary, b.ChapterID, b.CreatedAt, b.UpdatedAt)
		if err != nil {
			return nil, err
		}
		// 回填子查询计算出的 sort_order（SQL 内计算，返回对象需同步）
		_ = s.db.QueryRowContext(ctx, `SELECT sort_order FROM scene_beats WHERE id=?`, b.ID).Scan(&b.SortOrder)
		return &b, nil
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO scene_beats(id, project_id, chapter_id, title, summary, sort_order, created_at, updated_at)
		VALUES(?,?,?,?,?,?,?,?)`,
		b.ID, b.ProjectID, b.ChapterID, b.Title, b.Summary, b.SortOrder, b.CreatedAt, b.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &b, nil
}

// ListSceneBeats 列出章节场景卡（按 sort_order）
func (s *Store) ListSceneBeats(ctx context.Context, chapterID string) ([]SceneBeat, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, project_id, chapter_id, title, summary, sort_order, created_at, updated_at
		FROM scene_beats WHERE chapter_id=? ORDER BY sort_order`, chapterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SceneBeat
	for rows.Next() {
		var b SceneBeat
		if err := rows.Scan(&b.ID, &b.ProjectID, &b.ChapterID, &b.Title, &b.Summary, &b.SortOrder, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// ListProjectSceneBeats 列出项目全部场景卡
func (s *Store) ListProjectSceneBeats(ctx context.Context, projectID string) ([]SceneBeat, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, project_id, chapter_id, title, summary, sort_order, created_at, updated_at
		FROM scene_beats WHERE project_id=? ORDER BY sort_order`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SceneBeat
	for rows.Next() {
		var b SceneBeat
		if err := rows.Scan(&b.ID, &b.ProjectID, &b.ChapterID, &b.Title, &b.Summary, &b.SortOrder, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// UpdateSceneBeat 更新场景卡
func (s *Store) UpdateSceneBeat(ctx context.Context, id string, title, summary *string) (*SceneBeat, error) {
	setParts := []string{"updated_at=?"}
	args := []interface{}{now()}
	if title != nil {
		setParts = append(setParts, "title=?")
		args = append(args, *title)
	}
	if summary != nil {
		setParts = append(setParts, "summary=?")
		args = append(args, *summary)
	}
	args = append(args, id)
	if _, err := s.db.ExecContext(ctx, "UPDATE scene_beats SET "+joinSet(setParts)+" WHERE id=?", args...); err != nil {
		return nil, err
	}
	var b SceneBeat
	err := s.db.QueryRowContext(ctx, `
		SELECT id, project_id, chapter_id, title, summary, sort_order, created_at, updated_at
		FROM scene_beats WHERE id=?`, id).
		Scan(&b.ID, &b.ProjectID, &b.ChapterID, &b.Title, &b.Summary, &b.SortOrder, &b.CreatedAt, &b.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &b, nil
}

// DeleteSceneBeat 删除场景卡
func (s *Store) DeleteSceneBeat(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM scene_beats WHERE id=?`, id)
	return err
}
