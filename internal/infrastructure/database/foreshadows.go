package database

import (
	"context"
)

// ============================================================
// foreshadows.go —— 伏笔追踪（埋设/回收/闭环校验）
// ============================================================

// ForeshadowStatus 伏笔状态
const (
	ForeshadowPending    = "pending"    // 已埋设，待回收
	ForeshadowRecollected = "recollected" // 已回收
	ForeshadowDropped    = "dropped"    // 已放弃（作者标记）
)

// Foreshadow 伏笔
type Foreshadow struct {
	ID              string `json:"id"`
	ProjectID       string `json:"project_id"`
	Title           string `json:"title"`
	Description     string `json:"description"`
	SetupChapterID  string `json:"setup_chapter_id"`  // 埋设章节
	SetupChapterNo  int    `json:"setup_chapter_no"`
	PayoffChapterID string `json:"payoff_chapter_id"` // 回收章节
	PayoffChapterNo int    `json:"payoff_chapter_no"`
	Status          string `json:"status"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
}

// CreateForeshadow 新建伏笔
func (s *Store) CreateForeshadow(ctx context.Context, f Foreshadow) (*Foreshadow, error) {
	f.ID = newID()
	f.CreatedAt = now()
	f.UpdatedAt = now()
	if f.Status == "" {
		f.Status = ForeshadowPending
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO foreshadows(id, project_id, title, description, setup_chapter_id, payoff_chapter_id, status, created_at, updated_at)
		VALUES(?,?,?,?,?,?,?,?,?)`,
		f.ID, f.ProjectID, f.Title, f.Description, f.SetupChapterID, f.PayoffChapterID, f.Status, f.CreatedAt, f.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &f, nil
}

// ListForeshadows 列出项目全部伏笔
func (s *Store) ListForeshadows(ctx context.Context, projectID string) ([]Foreshadow, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, project_id, title, description, setup_chapter_id, payoff_chapter_id, status, created_at, updated_at
		FROM foreshadows WHERE project_id=? ORDER BY created_at`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Foreshadow
	for rows.Next() {
		var f Foreshadow
		if err := rows.Scan(&f.ID, &f.ProjectID, &f.Title, &f.Description, &f.SetupChapterID, &f.PayoffChapterID, &f.Status, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// UpdateForeshadow 更新伏笔（标题/描述/回收章节/状态）
func (s *Store) UpdateForeshadow(ctx context.Context, id string, title, description, payoffChapterID *string, status *string) (*Foreshadow, error) {
	setParts := []string{"updated_at=?"}
	args := []interface{}{now()}
	if title != nil {
		setParts = append(setParts, "title=?")
		args = append(args, *title)
	}
	if description != nil {
		setParts = append(setParts, "description=?")
		args = append(args, *description)
	}
	if payoffChapterID != nil {
		setParts = append(setParts, "payoff_chapter_id=?")
		args = append(args, *payoffChapterID)
	}
	if status != nil {
		setParts = append(setParts, "status=?")
		args = append(args, *status)
	}
	args = append(args, id)
	if _, err := s.db.ExecContext(ctx, "UPDATE foreshadows SET "+joinSet(setParts)+" WHERE id=?", args...); err != nil {
		return nil, err
	}
	return s.GetForeshadow(ctx, id)
}

// GetForeshadow 查询单个伏笔
func (s *Store) GetForeshadow(ctx context.Context, id string) (*Foreshadow, error) {
	var f Foreshadow
	err := s.db.QueryRowContext(ctx, `
		SELECT id, project_id, title, description, setup_chapter_id, payoff_chapter_id, status, created_at, updated_at
		FROM foreshadows WHERE id=?`, id).
		Scan(&f.ID, &f.ProjectID, &f.Title, &f.Description, &f.SetupChapterID, &f.PayoffChapterID, &f.Status, &f.CreatedAt, &f.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &f, nil
}

// DeleteForeshadow 删除伏笔
func (s *Store) DeleteForeshadow(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM foreshadows WHERE id=?`, id)
	return err
}

// ForeshadowScan 扫描结果（AI 提取的候选伏笔，先不入库，由前端确认）
type ForeshadowScan struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	ChapterID   string `json:"chapter_id"`
}

func joinSet(parts []string) string {
	s := ""
	for i, p := range parts {
		if i > 0 {
			s += ", "
		}
		s += p
	}
	return s
}
