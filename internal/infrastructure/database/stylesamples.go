package database

import (
	"context"
	"database/sql"
	"strings"
)

// ============================================================
// stylesamples.go —— 文风样本库（用户自购书籍的本地文风参考片段）
// 注意：内容仅存用户本地数据库，不随项目分发；供生成时风格参考与评测参照。
// ============================================================

// StyleSample 文风样本（作品片段）
type StyleSample struct {
	ID         string `json:"id"`
	Title      string `json:"title"`      // 作品名
	Author     string `json:"author"`     // 作者
	Category   string `json:"category"`   // 风格标签（如：热血燃向/克苏鲁悬疑/无限流惊悚）
	SourceFile string `json:"source_file"` // 来源文件（仅本地记录）
	Kind       string `json:"kind"`       // 拆书素材类型：fragment(关键片段)/character(人物卡)/world(世界观)/foreshadow(伏笔)，空=旧样本片段
	Content    string `json:"content"`    // 片段正文
	CreatedAt  string `json:"created_at"`
}

// StyleSampleKind 拆书素材类型常量
const (
	KindFragment   = "fragment"   // 关键片段（文风参考）
	KindCharacter  = "character"  // 人物卡
	KindWorld      = "world"      // 世界观
	KindForeshadow = "foreshadow" // 伏笔设计
)

// CreateStyleSample 新增样本
func (s *Store) CreateStyleSample(ctx context.Context, m StyleSample) (*StyleSample, error) {
	m.ID = newID()
	m.CreatedAt = now()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO style_samples(id, title, author, category, source_file, kind, content, created_at)
		VALUES(?,?,?,?,?,?,?,?)`,
		m.ID, m.Title, m.Author, m.Category, m.SourceFile, m.Kind, m.Content, m.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// ListStyleSamples 列出样本（可选风格过滤；全量返回供本地库使用）
func (s *Store) ListStyleSamples(ctx context.Context, category string) ([]StyleSample, error) {
	q := `SELECT id, title, author, category, source_file, COALESCE(kind,'fragment'), content, created_at FROM style_samples`
	args := []interface{}{}
	if category != "" {
		q += ` WHERE category=?`
		args = append(args, category)
	}
	q += ` ORDER BY title, created_at DESC LIMIT 2000`
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []StyleSample
	for rows.Next() {
		var m StyleSample
		if err := rows.Scan(&m.ID, &m.Title, &m.Author, &m.Category, &m.SourceFile, &m.Kind, &m.Content, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// ListStyleSampleMeta 列出样本元信息（不含 content，供前端列表/选择用）
func (s *Store) ListStyleSampleMeta(ctx context.Context) ([]StyleSample, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, title, author, category, source_file, COALESCE(kind,'fragment'), '', created_at FROM style_samples ORDER BY title, created_at DESC LIMIT 2000`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []StyleSample
	for rows.Next() {
		var m StyleSample
		if err := rows.Scan(&m.ID, &m.Title, &m.Author, &m.Category, &m.SourceFile, &m.Kind, &m.Content, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// GetStyleSample 查询单个样本
func (s *Store) GetStyleSample(ctx context.Context, id string) (*StyleSample, error) {
	var m StyleSample
	err := s.db.QueryRowContext(ctx, `
		SELECT id, title, author, category, source_file, COALESCE(kind,'fragment'), content, created_at FROM style_samples WHERE id=?`, id).
		Scan(&m.ID, &m.Title, &m.Author, &m.Category, &m.SourceFile, &m.Kind, &m.Content, &m.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// UpdateStyleSample 更新样本字段（nil 表示不更新）
func (s *Store) UpdateStyleSample(ctx context.Context, id string, title, author, category, content *string) (*StyleSample, error) {
	setParts := []string{}
	args := []interface{}{}
	if title != nil {
		setParts = append(setParts, "title=?")
		args = append(args, *title)
	}
	if author != nil {
		setParts = append(setParts, "author=?")
		args = append(args, *author)
	}
	if category != nil {
		setParts = append(setParts, "category=?")
		args = append(args, *category)
	}
	if content != nil {
		setParts = append(setParts, "content=?")
		args = append(args, *content)
	}
	if len(setParts) == 0 {
		return s.GetStyleSample(ctx, id)
	}
	args = append(args, id)
	if _, err := s.db.ExecContext(ctx, "UPDATE style_samples SET "+joinSet(setParts)+" WHERE id=?", args...); err != nil {
		return nil, err
	}
	return s.GetStyleSample(ctx, id)
}

// DeleteStyleSample 删除样本
func (s *Store) DeleteStyleSample(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM style_samples WHERE id=?`, id)
	return err
}

// DeleteStyleSamplesBySource 按来源文件删除整本书的素材（拆书参考书删除，返回删除条数）
func (s *Store) DeleteStyleSamplesBySource(ctx context.Context, sourceFile string) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM style_samples WHERE source_file=?`, sourceFile)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// GetStyleSamplesByIDs 按 ID 批量查询样本（生成时风格注入用，保持传入顺序）
func (s *Store) GetStyleSamplesByIDs(ctx context.Context, ids []string) ([]StyleSample, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	q := `SELECT id, title, author, category, source_file, COALESCE(kind,'fragment'), content, created_at FROM style_samples WHERE id IN (`
	args := make([]interface{}, 0, len(ids))
	for i, id := range ids {
		if i > 0 {
			q += ","
		}
		q += "?"
		args = append(args, id)
	}
	q += `)`
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	byID := map[string]StyleSample{}
	var out []StyleSample
	for rows.Next() {
		var m StyleSample
		if err := rows.Scan(&m.ID, &m.Title, &m.Author, &m.Category, &m.SourceFile, &m.Kind, &m.Content, &m.CreatedAt); err != nil {
			return nil, err
		}
		byID[m.ID] = m
	}
	// 按传入顺序返回（样本去重）
	seen := map[string]bool{}
	for _, id := range ids {
		if m, ok := byID[id]; ok && !seen[id] {
			out = append(out, m)
			seen[id] = true
		}
	}
	return out, rows.Err()
}

// CountStyleSamples 样本总数（前端展示）
func (s *Store) CountStyleSamples(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM style_samples`).Scan(&n)
	return n, err
}

// ReplaceStyleSamples 重建指定来源的样本：事务内先删除 source_file 匹配行再批量插入。
// 原子执行——中途失败不丢旧数据（-rebuild 场景），且保留用户手工添加的样本。
func (s *Store) ReplaceStyleSamples(ctx context.Context, samples []StyleSample) (int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	files := map[string]bool{}
	for _, m := range samples {
		if m.SourceFile != "" {
			files[m.SourceFile] = true
		}
	}
	for f := range files {
		if _, err := tx.ExecContext(ctx, `DELETE FROM style_samples WHERE source_file=?`, f); err != nil {
			return 0, err
		}
	}
	count := 0
	for _, m := range samples {
		if strings.TrimSpace(m.Content) == "" {
			continue
		}
		m.ID = newID()
		m.CreatedAt = now()
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO style_samples(id, title, author, category, source_file, kind, content, created_at)
			VALUES(?,?,?,?,?,?,?,?)`,
			m.ID, m.Title, m.Author, m.Category, m.SourceFile, m.Kind, m.Content, m.CreatedAt); err != nil {
			return count, err
		}
		count++
	}
	if err := tx.Commit(); err != nil {
		return count, err
	}
	return count, nil
}

// ImportStyleSamples 批量导入（导入工具产物；事务内逐条插入，跳过空内容与重复 source_file+title）
func (s *Store) ImportStyleSamples(ctx context.Context, samples []StyleSample) (int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	count := 0
	for _, m := range samples {
		if strings.TrimSpace(m.Content) == "" {
			continue
		}
		// 去重：同 source_file + title 已存在则跳过（幂等，重复导入不产生副本）
		var exists int
		if err := tx.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM style_samples WHERE source_file=? AND title=?`,
			m.SourceFile, m.Title).Scan(&exists); err != nil {
			return count, err
		}
		if exists > 0 {
			continue
		}
		m.ID = newID()
		m.CreatedAt = now()
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO style_samples(id, title, author, category, source_file, kind, content, created_at)
			VALUES(?,?,?,?,?,?,?,?)`,
			m.ID, m.Title, m.Author, m.Category, m.SourceFile, m.Kind, m.Content, m.CreatedAt); err != nil {
			return count, err
		}
		count++
	}
	if err := tx.Commit(); err != nil {
		return count, err
	}
	return count, nil
}
