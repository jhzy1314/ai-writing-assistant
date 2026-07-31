package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"unicode"

	"github.com/google/uuid"
)

// Volume volumes 表一行
type Volume struct {
	ID        string `json:"id"`
	ProjectID string `json:"project_id"`
	Title     string `json:"title"`
	SortOrder int    `json:"sort_order"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// Chapter chapters 表一行
type Chapter struct {
	ID        string `json:"id"`
	ProjectID string `json:"project_id"`
	VolumeID  string `json:"volume_id"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	WordCount int    `json:"word_count"`
	SortOrder int    `json:"sort_order"`
	Tags      string `json:"tags"`
	Synopsis  string `json:"synopsis"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	IsDeleted int    `json:"is_deleted"`
	DeletedAt string `json:"deleted_at,omitempty"`
}

// ChapterWithVolume 章节附带卷信息（树形展示用）
type ChapterWithVolume struct {
	Chapter
	VolumeTitle string `json:"volume_title,omitempty"`
}

// ChapterVersion chapter_versions 表一行
type ChapterVersion struct {
	ID        string `json:"id"`
	ChapterID string `json:"chapter_id"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	Version   int    `json:"version"`
	CreatedAt string `json:"created_at"`
}

// SplitChaptersRequest 分割导入请求
type SplitChaptersRequest struct {
	ProjectID string `json:"project_id"`
	Content   string `json:"content"`
	SplitBy   string `json:"split_by"` // "## " / "第.*章" / "auto"
}

// ===== Volume CRUD =====

func (s *Store) ListVolumes(ctx context.Context, projectID string) ([]Volume, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, project_id, title, sort_order, created_at, updated_at FROM volumes WHERE project_id=? ORDER BY sort_order`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Volume{}
	for rows.Next() {
		var v Volume
		if err := rows.Scan(&v.ID, &v.ProjectID, &v.Title, &v.SortOrder, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (s *Store) CreateVolume(ctx context.Context, projectID, title string, sortOrder int) (*Volume, error) {
	if sortOrder <= 0 {
		var maxOrd int
		_ = s.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(sort_order),0) FROM volumes WHERE project_id=?`, projectID).Scan(&maxOrd)
		sortOrder = maxOrd + 1
	}
	v := &Volume{ID: newID(), ProjectID: projectID, Title: title, SortOrder: sortOrder, CreatedAt: now(), UpdatedAt: now()}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO volumes(id, project_id, title, sort_order, created_at, updated_at) VALUES(?,?,?,?,?,?)`,
		v.ID, v.ProjectID, v.Title, v.SortOrder, v.CreatedAt, v.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return v, nil
}

func (s *Store) UpdateVolume(ctx context.Context, id string, title *string, sortOrder *int) (*Volume, error) {
	if title == nil && sortOrder == nil {
		return s.getVolume(ctx, id)
	}
	args := []interface{}{}
	setParts := []string{}
	if title != nil {
		setParts = append(setParts, "title=?")
		args = append(args, *title)
	}
	if sortOrder != nil {
		setParts = append(setParts, "sort_order=?")
		args = append(args, *sortOrder)
	}
	setParts = append(setParts, "updated_at=?")
	args = append(args, now())
	args = append(args, id)
	q := "UPDATE volumes SET " + joinStrings(setParts, ", ") + " WHERE id=?"
	if _, err := s.db.ExecContext(ctx, q, args...); err != nil {
		return nil, err
	}
	return s.getVolume(ctx, id)
}

func (s *Store) getVolume(ctx context.Context, id string) (*Volume, error) {
	var v Volume
	err := s.db.QueryRowContext(ctx, `SELECT id, project_id, title, sort_order, created_at, updated_at FROM volumes WHERE id=?`, id).
		Scan(&v.ID, &v.ProjectID, &v.Title, &v.SortOrder, &v.CreatedAt, &v.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func (s *Store) DeleteVolume(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM volumes WHERE id=?`, id)
	return err
}

func (s *Store) ReorderVolumes(ctx context.Context, items []struct {
	ID        string `json:"id"`
	SortOrder int    `json:"sort_order"`
}) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, it := range items {
		if _, err := tx.ExecContext(ctx, `UPDATE volumes SET sort_order=?, updated_at=? WHERE id=?`, it.SortOrder, now(), it.ID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ===== Chapter CRUD =====

func (s *Store) ListChapters(ctx context.Context, projectID, volumeID string) ([]ChapterWithVolume, error) {
	var q string
	var args []interface{}
	if volumeID != "" {
		q = `SELECT c.id, c.project_id, c.volume_id, c.title, c.content, c.word_count, c.sort_order, c.created_at, c.updated_at,
			COALESCE(v.title,'') FROM chapters c LEFT JOIN volumes v ON v.id=c.volume_id
			WHERE c.project_id=? AND c.volume_id=? AND c.is_deleted=0 ORDER BY c.sort_order`
		args = []interface{}{projectID, volumeID}
	} else {
		q = `SELECT c.id, c.project_id, c.volume_id, c.title, c.content, c.word_count, c.sort_order, c.created_at, c.updated_at,
			COALESCE(v.title,'') FROM chapters c LEFT JOIN volumes v ON v.id=c.volume_id
			WHERE c.project_id=? AND c.is_deleted=0 ORDER BY c.volume_id, c.sort_order`
		args = []interface{}{projectID}
	}
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ChapterWithVolume{}
	for rows.Next() {
		var c ChapterWithVolume
		if err := rows.Scan(&c.ID, &c.ProjectID, &c.VolumeID, &c.Title, &c.Content, &c.WordCount, &c.SortOrder, &c.CreatedAt, &c.UpdatedAt, &c.VolumeTitle); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) CreateChapter(ctx context.Context, projectID, volumeID, title, content string) (*Chapter, error) {
	var maxOrd int
	_ = s.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(sort_order),0) FROM chapters WHERE project_id=?`, projectID).Scan(&maxOrd)
	wc := len([]rune(content))
	c := &Chapter{
		ID: newID(), ProjectID: projectID, VolumeID: volumeID, Title: title, Content: content,
		WordCount: wc, SortOrder: maxOrd + 1, CreatedAt: now(), UpdatedAt: now(),
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO chapters(id, project_id, volume_id, title, content, word_count, sort_order, tags, synopsis, created_at, updated_at, is_deleted, deleted_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,0,'')`,
		c.ID, c.ProjectID, c.VolumeID, c.Title, c.Content, c.WordCount, c.SortOrder, c.Tags, c.Synopsis, c.CreatedAt, c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	_, _ = s.db.ExecContext(ctx, `UPDATE projects SET updated_at=? WHERE id=?`, now(), projectID)
	return c, nil
}

func (s *Store) GetChapter(ctx context.Context, id string) (*Chapter, error) {
	var c Chapter
	err := s.db.QueryRowContext(ctx,
		`SELECT id, project_id, volume_id, title, content, word_count, sort_order, COALESCE(tags,''), COALESCE(synopsis,''), created_at, updated_at, is_deleted, COALESCE(deleted_at,'') FROM chapters WHERE id=?`, id).
		Scan(&c.ID, &c.ProjectID, &c.VolumeID, &c.Title, &c.Content, &c.WordCount, &c.SortOrder, &c.Tags, &c.Synopsis, &c.CreatedAt, &c.UpdatedAt, &c.IsDeleted, &c.DeletedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *Store) UpdateChapter(ctx context.Context, id string, title, content, volumeID, ifUpdatedAt *string) (*Chapter, error) {
	if title == nil && content == nil && volumeID == nil {
		return s.GetChapter(ctx, id)
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
		wc := len([]rune(*content))
		setParts = append(setParts, "word_count=?")
		args = append(args, wc)
	}
	if volumeID != nil {
		setParts = append(setParts, "volume_id=?")
		args = append(args, *volumeID)
	}
	setParts = append(setParts, "updated_at=?")
	args = append(args, now())
	args = append(args, id)
	q := "UPDATE chapters SET " + joinStrings(setParts, ", ") + " WHERE id=?"
	// 乐观锁：若客户端提供了上次看到的 updated_at，则仅在该时间戳匹配时更新
	if ifUpdatedAt != nil && *ifUpdatedAt != "" {
		q += " AND updated_at=?"
		args = append(args, *ifUpdatedAt)
	}
	result, err := s.db.ExecContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	if ifUpdatedAt != nil && *ifUpdatedAt != "" {
		n, _ := result.RowsAffected()
		if n == 0 {
			return nil, fmt.Errorf("内容已被其他窗口修改，请刷新后重试")
		}
	}
	return s.GetChapter(ctx, id)
}

func (s *Store) DeleteChapter(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE chapters SET is_deleted=1, deleted_at=? WHERE id=?`, now(), id)
	return err
}

// ListTrashChapters 列出已软删除的章节（回收站）
func (s *Store) ListTrashChapters(ctx context.Context, projectID string) ([]Chapter, error) {
	var q string
	var args []interface{}
	if projectID != "" {
		q = `SELECT id, project_id, volume_id, title, content, word_count, sort_order, COALESCE(tags,''), COALESCE(synopsis,''), created_at, updated_at, is_deleted, COALESCE(deleted_at,'') FROM chapters WHERE is_deleted=1 AND project_id=? ORDER BY deleted_at DESC`
		args = []interface{}{projectID}
	} else {
		q = `SELECT id, project_id, volume_id, title, content, word_count, sort_order, COALESCE(tags,''), COALESCE(synopsis,''), created_at, updated_at, is_deleted, COALESCE(deleted_at,'') FROM chapters WHERE is_deleted=1 ORDER BY deleted_at DESC`
	}
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Chapter{}
	for rows.Next() {
		var c Chapter
		if err := rows.Scan(&c.ID, &c.ProjectID, &c.VolumeID, &c.Title, &c.Content, &c.WordCount, &c.SortOrder, &c.Tags, &c.Synopsis, &c.CreatedAt, &c.UpdatedAt, &c.IsDeleted, &c.DeletedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// RestoreChapter 恢复软删除的章节
func (s *Store) RestoreChapter(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, `UPDATE chapters SET is_deleted=0, deleted_at='' WHERE id=? AND is_deleted=1`, id)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("章节不存在或未被删除")
	}
	return nil
}

// PermanentDeleteChapter 永久删除章节（二次确认后调用）
func (s *Store) PermanentDeleteChapter(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM chapters WHERE id=? AND is_deleted=1`, id)
	return err
}

// PurgeOldTrash 清理超过7天的回收站章节
func (s *Store) PurgeOldTrash(ctx context.Context) (int, error) {
	result, err := s.db.ExecContext(ctx,
		`DELETE FROM chapters WHERE is_deleted=1 AND deleted_at != '' AND datetime(deleted_at) < datetime('now', '-7 days')`)
	if err != nil {
		return 0, err
	}
	n, _ := result.RowsAffected()
	return int(n), nil
}

func (s *Store) CopyChapter(ctx context.Context, id string) (*Chapter, error) {
	src, err := s.GetChapter(ctx, id)
	if err != nil || src == nil {
		return nil, err
	}
	if src == nil {
		return nil, fmt.Errorf("章节不存在")
	}
	return s.CreateChapter(ctx, src.ProjectID, src.VolumeID, src.Title+" (副本)", src.Content)
}

func (s *Store) ReorderChapters(ctx context.Context, items []struct {
	ID        string `json:"id"`
	SortOrder int    `json:"sort_order"`
}) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, it := range items {
		if _, err := tx.ExecContext(ctx, `UPDATE chapters SET sort_order=?, updated_at=? WHERE id=?`, it.SortOrder, now(), it.ID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ===== Chapter Version CRUD =====

func (s *Store) SaveChapterVersion(ctx context.Context, chapterID, title, content string) (*ChapterVersion, error) {
	var maxVer int
	_ = s.db.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(version),0) FROM chapter_versions WHERE chapter_id=?`, chapterID).Scan(&maxVer)
	cv := &ChapterVersion{
		ID:        newID(),
		ChapterID: chapterID,
		Title:     title,
		Content:   content,
		Version:   maxVer + 1,
		CreatedAt: now(),
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO chapter_versions(id, chapter_id, title, content, version, created_at) VALUES(?,?,?,?,?,?)`,
		cv.ID, cv.ChapterID, cv.Title, cv.Content, cv.Version, cv.CreatedAt)
	if err != nil {
		return nil, err
	}
	return cv, nil
}

func (s *Store) ListChapterVersions(ctx context.Context, chapterID string) ([]ChapterVersion, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, chapter_id, title, content, version, created_at FROM chapter_versions WHERE chapter_id=? ORDER BY version DESC`, chapterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ChapterVersion{}
	for rows.Next() {
		var cv ChapterVersion
		if err := rows.Scan(&cv.ID, &cv.ChapterID, &cv.Title, &cv.Content, &cv.Version, &cv.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, cv)
	}
	return out, rows.Err()
}

func (s *Store) GetChapterVersion(ctx context.Context, id string) (*ChapterVersion, error) {
	var cv ChapterVersion
	err := s.db.QueryRowContext(ctx,
		`SELECT id, chapter_id, title, content, version, created_at FROM chapter_versions WHERE id=?`, id).
		Scan(&cv.ID, &cv.ChapterID, &cv.Title, &cv.Content, &cv.Version, &cv.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &cv, nil
}

// ===== Import / Export / Split =====

// ExportChapters 导出项目全部章节为 JSON
type ExportChapter struct {
	Title     string `json:"title"`
	VolumeID  string `json:"volume_id,omitempty"`
	Content   string `json:"content"`
	SortOrder int    `json:"sort_order"`
}

type ExportData struct {
	ProjectID string          `json:"project_id"`
	Volumes   []Volume        `json:"volumes,omitempty"`
	Chapters  []ExportChapter `json:"chapters"`
}

func (s *Store) ExportChapters(ctx context.Context, projectID string) (*ExportData, error) {
	volumes, _ := s.ListVolumes(ctx, projectID)
	chapters, err := s.ListChapters(ctx, projectID, "")
	if err != nil {
		return nil, err
	}
	ecs := make([]ExportChapter, len(chapters))
	for i, c := range chapters {
		ecs[i] = ExportChapter{
			Title: c.Title, VolumeID: c.VolumeID,
			Content: c.Content, SortOrder: c.SortOrder,
		}
	}
	return &ExportData{ProjectID: projectID, Volumes: volumes, Chapters: ecs}, nil
}

// ImportChapters 从 JSON 导入章节（清空后重建）
func (s *Store) ImportChapters(ctx context.Context, projectID string, data *ExportData) (int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	// 软删除旧章节
	if _, err := tx.ExecContext(ctx, `UPDATE chapters SET is_deleted=1, deleted_at=? WHERE project_id=?`, now(), projectID); err != nil {
		return 0, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM volumes WHERE project_id=?`, projectID); err != nil {
		return 0, err
	}

	// 重建卷
	volMap := map[string]string{} // old id -> new id
	for _, vol := range data.Volumes {
		newID := newID()
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO volumes(id, project_id, title, sort_order, created_at, updated_at) VALUES(?,?,?,?,?,?)`,
			newID, projectID, vol.Title, vol.SortOrder, now(), now()); err != nil {
			return 0, err
		}
		volMap[vol.ID] = newID
	}

	// 重建章节
	count := 0
	for _, c := range data.Chapters {
		vid := c.VolumeID
		if mapped, ok := volMap[c.VolumeID]; ok {
			vid = mapped
		}
		wc := wordCount(c.Content)
		newID := newID()
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO chapters(id, project_id, volume_id, title, content, word_count, sort_order, tags, synopsis, created_at, updated_at, is_deleted, deleted_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,0,'')`,
			newID, projectID, vid, c.Title, c.Content, wc, count, "", "", now(), now()); err != nil {
			return 0, err
		}
		count++
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	_, _ = s.db.ExecContext(ctx, `UPDATE projects SET updated_at=? WHERE id=?`, now(), projectID)
	return count, nil
}

// SplitChapters 将正文按标题/标记分割为章节并写入
// 导入前清空该项目所有现有章节（事务保护），确保 sort_order 从1开始递增
func (s *Store) SplitChapters(ctx context.Context, req *SplitChaptersRequest) ([]Chapter, error) {
	if strings.TrimSpace(req.Content) == "" {
		return nil, fmt.Errorf("content 不能为空")
	}
	segments := splitContent(req.Content, req.SplitBy)
	// 过滤空章节：内容少于100字的段落合并到相邻章节
	segments = mergeSmallSegments(segments, 100)
	if len(segments) == 0 {
		return nil, fmt.Errorf("error:未能识别出章节，请检查正文是否包含标题标记")
	}
	// 仅识别到1段：不覆盖现有章节，追加
	if len(segments) == 1 {
		var result []Chapter
		ch, err := s.CreateChapterWithVersion(ctx, req.ProjectID, "", segments[0].title, segments[0].content)
		if err != nil {
			return nil, err
		}
		result = append(result, *ch)
		return result, nil
	}

	// 多章分割：在事务中先软删除现有章节再写入，避免编号冲突
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	// 软删除旧章节
	if _, err := tx.ExecContext(ctx, `UPDATE chapters SET is_deleted=1, deleted_at=? WHERE project_id=?`, now(), req.ProjectID); err != nil {
		return nil, err
	}

	var result []Chapter
	for i, seg := range segments {
		title := seg.title
		if title == "" {
			title = fmt.Sprintf("第%d章", i+1)
		}
		wc := wordCount(seg.content)
		ord := i + 1
		newID := newID()
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO chapters(id, project_id, volume_id, title, content, word_count, sort_order, tags, synopsis, created_at, updated_at, is_deleted, deleted_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,0,'')`,
			newID, req.ProjectID, "", title, seg.content, wc, ord, "", "", now(), now()); err != nil {
			return result, fmt.Errorf("写入第%d章失败: %w", ord, err)
		}
		// 创建初始版本快照
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO chapter_versions(id, chapter_id, title, content, version, created_at) VALUES(?,?,?,?,1,?)`,
			newID, newID, title+" (初始)", seg.content, now()); err != nil {
			return result, fmt.Errorf("保存第%d章版本失败: %w", ord, err)
		}
		ch := &Chapter{
			ID: newID, ProjectID: req.ProjectID, VolumeID: "", Title: title,
			Content: seg.content, WordCount: wc, SortOrder: ord,
			CreatedAt: now(), UpdatedAt: now(),
		}
		result = append(result, *ch)
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	_, _ = s.db.ExecContext(ctx, `UPDATE projects SET updated_at=? WHERE id=?`, now(), req.ProjectID)
	return result, nil
}

// SplitByTitles 使用 AI 识别的标题列表，在原始文本中精确定位并分割章节
// 相比 SplitChapters 仅依赖正则，该方法先用 AI 拿到标题，再在原文中 match 位置切分
func (s *Store) SplitByTitles(ctx context.Context, req *SplitChaptersRequest, titles []string) ([]Chapter, error) {
	if len(titles) < 2 || strings.TrimSpace(req.Content) == "" {
		return nil, fmt.Errorf("至少需要2个标题")
	}

	content := req.Content
	var segments []chapterSeg
	prevEnd := 0

	for _, title := range titles {
		// 在剩余文本中搜索标题
		idx := strings.Index(content[prevEnd:], title)
		if idx < 0 {
			// 尝试模糊匹配：去掉前导空白
			trimmed := strings.TrimSpace(title)
			idx = strings.Index(content[prevEnd:], trimmed)
		}
		if idx < 0 {
			// 标题未在原文中找到，尝试第一个非标点字符开始的子串
			for i, r := range []rune(title) {
				if r > 32 && r != '#' && r != ' ' {
					sub := string([]rune(title)[i:])
					idx = strings.Index(content[prevEnd:], sub)
					break
				}
			}
		}
		if idx < 0 {
			continue // 找不到就跳过
		}

		start := prevEnd + idx
		if segments != nil {
			// 上一段从 prevEnd 到 start（不含新标题）
			segContent := strings.TrimSpace(content[prevEnd:start])
			if segContent != "" {
				// 分离标题行和正文
				bodyStart := strings.Index(segContent, "\n")
				if bodyStart < 0 {
					bodyStart = len(segContent)
				}
				segTitle := strings.TrimSpace(segContent[:bodyStart])
				segBody := strings.TrimSpace(segContent[bodyStart:])
				if segBody == "" && bodyStart < len(segContent) {
					segBody = strings.TrimSpace(segContent[bodyStart:])
				}
				segments = append(segments, chapterSeg{title: segTitle, content: segBody})
			}
		}

		// 跳到标题之后继续
		prevEnd = start
		if len(segments) == 0 {
			// 第一个标题：正文从标题行之后开始
			bodyStart := strings.Index(content[prevEnd:], "\n")
			if bodyStart >= 0 {
				prevEnd = start + bodyStart + 1
			}
		}
		segments = append(segments) // placeholder, will be filled below
		segments = segments[:len(segments)-1]
	}

	// 最后一段：从 prevEnd 到文末
	if prevEnd < len(content) {
		lastContent := strings.TrimSpace(content[prevEnd:])
		bodyStart := strings.Index(lastContent, "\n")
		segTitle := "后续章节"
		segBody := lastContent
		if bodyStart >= 0 && bodyStart < 40 {
			segTitle = strings.TrimSpace(lastContent[:bodyStart])
			segBody = strings.TrimSpace(lastContent[bodyStart:])
		}
		if segBody != "" {
			segments = append(segments, chapterSeg{title: segTitle, content: segBody})
		}
	}

	// 重新构建：为每个标题匹配其后的正文段落
	segments = nil
	prevEnd = 0
	for i, title := range titles {
		idx := strings.Index(content[prevEnd:], title)
		if idx < 0 {
			idx = strings.Index(content[prevEnd:], strings.TrimSpace(title))
		}
		if idx < 0 {
			continue
		}
		start := prevEnd + idx
		titleEnd := start + len(title)

		if i > 0 {
			segContent := strings.TrimSpace(content[prevEnd:start])
			if len(segments) > 0 && segContent != "" {
				segments[len(segments)-1].content = segContent
			}
		}

		// 下一段的内容边界
		end := len(content)
		if i < len(titles)-1 {
			nextTitle := titles[i+1]
			rest := content[titleEnd:]
			nextIdx := strings.Index(rest, nextTitle)
			if nextIdx < 0 {
				nextIdx = strings.Index(rest, strings.TrimSpace(nextTitle))
			}
			if nextIdx >= 0 {
				end = titleEnd + nextIdx
			}
		}

		segments = append(segments, chapterSeg{
			title:   title,
			content: strings.TrimSpace(content[titleEnd:end]),
		})
		prevEnd = titleEnd
	}

	if len(segments) < 2 {
		return nil, fmt.Errorf("未能从标题列表中分割出足够章节")
	}

	return s.commitSegments(ctx, req.ProjectID, segments)
}

// commitSegments 将分割后的段落在事务中写入数据库
func (s *Store) commitSegments(ctx context.Context, projectID string, segments []chapterSeg) ([]Chapter, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `UPDATE chapters SET is_deleted=1, deleted_at=? WHERE project_id=?`, now(), projectID); err != nil {
		return nil, err
	}

	var result []Chapter
	for i, seg := range segments {
		title := seg.title
		if title == "" {
			title = fmt.Sprintf("第%d章", i+1)
		}
		wc := wordCount(seg.content)
		cid := newID()
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO chapters(id, project_id, volume_id, title, content, word_count, sort_order, tags, synopsis, created_at, updated_at, is_deleted, deleted_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,0,'')`,
			cid, projectID, "", title, seg.content, wc, i+1, "", "", now(), now()); err != nil {
			return result, fmt.Errorf("写入第%d章失败: %w", i+1, err)
		}
		vid := uuid.NewString()
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO chapter_versions(id, chapter_id, title, content, version, created_at) VALUES(?,?,?,?,1,?)`,
			vid, cid, title, seg.content, now()); err != nil {
			return result, nil
		}
		ch := Chapter{ID: cid, ProjectID: projectID, Title: title, Content: seg.content, WordCount: wc, SortOrder: i + 1}
		result = append(result, ch)
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	_, _ = s.db.ExecContext(ctx, `UPDATE projects SET updated_at=? WHERE id=?`, now(), projectID)
	return result, nil
}

// CountSegments 预检：给定内容和分隔方式，返回能切出几段（只计数不写库）
func (s *Store) CountSegments(content, splitBy string) int {
	return len(splitContent(content, splitBy))
}

type chapterSeg struct {
	title   string
	content string
}

func splitContent(content, splitBy string) []chapterSeg {
	if splitBy == "" {
		splitBy = "auto"
	}
	var parts []chapterSeg
	lines := strings.Split(content, "\n")
	if splitBy == "auto" {
		// 先按 ## / ### / 第X章 模式识别
		parts = splitByPattern(lines)
		// 如果只识别到1段，尝试用 --- 分割（常见于 Markdown 导出的章节分隔）
		if len(parts) <= 1 && hasHRSeparator(lines) {
			parts = splitBySeparator(content, "---")
		}
	} else if splitBy == "## " || splitBy == "### " {
		parts = splitByHeading(lines, splitBy)
	} else {
		parts = splitBySeparator(content, splitBy)
	}
	return parts
}

// mergeSmallSegments 合并内容过少的段落到相邻章节
func mergeSmallSegments(segs []chapterSeg, minChars int) []chapterSeg {
	if len(segs) <= 1 {
		return segs
	}
	var result []chapterSeg
	for _, seg := range segs {
		wc := len([]rune(strings.TrimSpace(seg.content)))
		if wc < minChars && len(result) > 0 {
			// 合并到上一个章节
			last := &result[len(result)-1]
			if seg.title != "" && last.title == "" {
				last.title = seg.title
			}
			last.content += "\n\n" + seg.content
		} else {
			result = append(result, seg)
		}
	}
	return result
}

// hasHRSeparator 检测内容是否包含至少两条 --- 或 *** 水平分隔线
func hasHRSeparator(lines []string) bool {
	count := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "---" || trimmed == "***" || trimmed == "* * *" || trimmed == "- - -" {
			count++
		} else if strings.HasPrefix(trimmed, "---") && matchChapterRegex(strings.TrimSpace(trimmed[3:])) {
			count++
		} else if strings.HasPrefix(trimmed, "***") && matchChapterRegex(strings.TrimSpace(trimmed[3:])) {
			count++
		}
	}
	return count >= 2
}

func splitByPattern(lines []string) []chapterSeg {
	var parts []chapterSeg
	buf := &strings.Builder{}
	curTitle := ""
	prevWasHeading := false // 上下文校验：连续疑似标题行判定为伪标题
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		isMDHeading := strings.HasPrefix(trimmed, "## ") || strings.HasPrefix(trimmed, "### ") || strings.HasPrefix(trimmed, "# ")
		isChHeading := !isMDHeading && matchChapterRegex(trimmed)

		// 二次校验：上一行已是标题且≥15字(疑似正文误判)，当前行也是标题 → 上一行是伪标题，回退为正文
		if prevWasHeading && (isChHeading || isMDHeading) && len([]rune(trimmed)) <= 25 {
			prevTitleLen := len([]rune(curTitle))
			if prevTitleLen >= 15 {
				if buf.Len() > 0 {
					buf.WriteString("\n")
				}
				buf.WriteString(curTitle)
				curTitle = ""
				prevWasHeading = false
			}
		}

		if isMDHeading || isChHeading {
			// 卷名识别：匹配「第X卷」但不创建章节，记录卷信息跳过
			if strings.HasPrefix(trimmed, "第") && strings.Contains(trimmed, "卷") && !strings.Contains(trimmed, "章") && !strings.Contains(trimmed, "节") && !strings.Contains(trimmed, "回") {
				// 卷名不创建章节条目，仅保存前一段落
				if curTitle != "" || buf.Len() > 0 {
					parts = append(parts, chapterSeg{title: curTitle, content: strings.TrimSpace(buf.String())})
					buf.Reset()
				}
				curTitle = "" // 卷名丢弃，不作为章节标题
				prevWasHeading = false
				continue
			}
			if curTitle != "" || buf.Len() > 0 {
				parts = append(parts, chapterSeg{title: curTitle, content: strings.TrimSpace(buf.String())})
				buf.Reset()
			}
			curTitle = strings.TrimLeft(trimmed, "# ")
			curTitle = strings.TrimPrefix(curTitle, "**")
			curTitle = strings.TrimSuffix(curTitle, "**")
			prevWasHeading = true
			continue
		}
		prevWasHeading = false
		if buf.Len() > 0 {
			buf.WriteString("\n")
		}
		buf.WriteString(line)
	}
	if curTitle != "" || buf.Len() > 0 {
		parts = append(parts, chapterSeg{title: curTitle, content: strings.TrimSpace(buf.String())})
	}
	return parts
}

func splitByHeading(lines []string, prefix string) []chapterSeg {
	var parts []chapterSeg
	buf := &strings.Builder{}
	curTitle := ""
	for _, line := range lines {
		if strings.HasPrefix(line, prefix) {
			if curTitle != "" || buf.Len() > 0 {
				parts = append(parts, chapterSeg{title: curTitle, content: strings.TrimSpace(buf.String())})
				buf.Reset()
			}
			curTitle = strings.TrimPrefix(line, prefix)
		} else {
			if buf.Len() > 0 {
				buf.WriteString("\n")
			}
			buf.WriteString(line)
		}
	}
	if curTitle != "" || buf.Len() > 0 {
		parts = append(parts, chapterSeg{title: curTitle, content: strings.TrimSpace(buf.String())})
	}
	return parts
}

func splitBySeparator(content, sep string) []chapterSeg {
	chunks := strings.Split(content, sep)
	var parts []chapterSeg
	for i, chunk := range chunks {
		chunk = strings.TrimSpace(chunk)
		if chunk == "" {
			continue
		}
		lines := strings.Split(chunk, "\n")
		// 跳过开头的空行
		firstNonEmpty := 0
		for firstNonEmpty < len(lines) && strings.TrimSpace(lines[firstNonEmpty]) == "" {
			firstNonEmpty++
		}
		title := fmt.Sprintf("分段%d", i+1)
		bodyStart := firstNonEmpty + 1
		if firstNonEmpty < len(lines) {
			title = strings.TrimSpace(lines[firstNonEmpty])
			// 去掉 Markdown 标题标记
			title = strings.TrimLeft(title, "# ")
		}
		// 拼接正文（跳过标题行）
		if bodyStart < len(lines) {
			body := strings.Join(lines[bodyStart:], "\n")
			parts = append(parts, chapterSeg{title: title, content: strings.TrimSpace(body)})
		} else {
			// 没有正文（只有标题行），内容为空
			parts = append(parts, chapterSeg{title: title, content: ""})
		}
	}
	return parts
}

// matchChapterRegex 判断文本是否匹配章节标题模式
// 只匹配短标题（真正章节标题 < 25 字），排除「第四节课」「第一次月考」等口语误判
func matchChapterRegex(s string) bool {
	s = strings.TrimSpace(s)
	// 去掉加粗标记 **xxx**（docx 导出常见格式）
	s = strings.TrimPrefix(s, "**")
	s = strings.TrimSuffix(s, "**")
	s = strings.TrimSpace(s)
	// 只匹配短标题（真正章节标题 < 30 字）
	if len([]rune(s)) > 30 {
		return false
	}
	if !strings.HasPrefix(s, "第") && !strings.HasPrefix(s, "**第") {
		return false
	}
	// 第X卷不作为章节（是卷名，不是章名）
	if strings.Contains(s, "卷") && !strings.Contains(s, "章") {
		return false
	}
	// 排除明显误判
	if strings.Contains(s, "节课") || strings.Contains(s, "次月考") ||
		strings.Contains(s, "个同学") || strings.Contains(s, "一次考试") ||
		strings.Contains(s, "一次作业") || strings.Contains(s, "一个学期") ||
		strings.Contains(s, "四个人") {
		return false
	}
	// 排除"第二天""第三天"等时间状语误判
	if strings.HasPrefix(s, "第二") && !strings.Contains(s, "章") && !strings.Contains(s, "节") && !strings.Contains(s, "回") {
		return false
	}
	return strings.Contains(s, "章") || strings.Contains(s, "节") ||
		strings.HasSuffix(s, "回")
}

// ChapterCount 返回项目章节总数
func (s *Store) ChapterCount(ctx context.Context, projectID string) int {
	var n int
	_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM chapters WHERE project_id=? AND is_deleted=0`, projectID).Scan(&n)
	return n
}

// VolumeCount 返回项目卷总数
func (s *Store) VolumeCount(ctx context.Context, projectID string) int {
	var n int
	_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM volumes WHERE project_id=?`, projectID).Scan(&n)
	return n
}

// ===== Chapter-based context helpers (for pipeline context building) =====

// ChaptersAssembledText 拼接项目所有章节为可注入上下文文本
func (s *Store) ChaptersAssembledText(ctx context.Context, projectID string) (string, error) {
	chapters, err := s.ListChapters(ctx, projectID, "")
	if err != nil {
		return "", err
	}
	if len(chapters) == 0 {
		return "", nil
	}
	var b strings.Builder
	for _, c := range chapters {
		b.WriteString("【")
		b.WriteString(c.Title)
		b.WriteString("】\n")
		b.WriteString(c.Content)
		b.WriteString("\n\n")
	}
	return strings.TrimSpace(b.String()), nil
}

// MarshalJSON safe
func marshalJSON(v interface{}) ([]byte, error) { return json.Marshal(v) }

// wordCount 统计中文/英文混合文本的字数（中文按字符，英文按空格分词）
func wordCount(s string) int {
	count := 0
	inWord := false
	for _, r := range s {
		if unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r) {
			count++
			inWord = false
		} else if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if !inWord {
				count++
				inWord = true
			}
		} else if unicode.IsSpace(r) {
			inWord = false
		} else {
			inWord = false
		}
	}
	return count
}

// ProjectStats 项目统计信息
type ProjectStats struct {
	TotalChapters int            `json:"total_chapters"`
	TotalWords    int            `json:"total_words"`
	TotalChars    int            `json:"total_chars"`
	Volumes       []VolumeStats  `json:"volumes"`
}

// VolumeStats 卷统计信息
type VolumeStats struct {
	VolumeID   string `json:"volume_id"`
	Title      string `json:"title"`
	Chapters   int    `json:"chapters"`
	Words      int    `json:"words"`
}

// ProjectStats 获取项目统计
func (s *Store) GetProjectStats(ctx context.Context, projectID string) (*ProjectStats, error) {
	chapters, err := s.ListChapters(ctx, projectID, "")
	if err != nil {
		return nil, err
	}
	volumes, _ := s.ListVolumes(ctx, projectID)
	volMap := map[string]*VolumeStats{}
	for _, v := range volumes {
		volMap[v.ID] = &VolumeStats{VolumeID: v.ID, Title: v.Title}
	}
	stats := &ProjectStats{TotalChapters: len(chapters)}
	noVol := &VolumeStats{VolumeID: "", Title: "未分类"}
	for _, c := range chapters {
		stats.TotalWords += c.WordCount
		stats.TotalChars += len([]rune(c.Content))
		vs, ok := volMap[c.VolumeID]
		if !ok {
			vs = noVol
		}
		vs.Chapters++
		vs.Words += c.WordCount
	}
	for _, v := range volumes {
		if vs, ok := volMap[v.ID]; ok {
			stats.Volumes = append(stats.Volumes, *vs)
		}
	}
	if noVol.Chapters > 0 {
		stats.Volumes = append(stats.Volumes, *noVol)
	}
	return stats, nil
}

// MergeChapters 合并同一卷内连续章节
// - 必须是同一卷内的连续章节（按 sort_order 连续）
// - 以第一个选中章节为主章节，其余内容按顺序拼接至文末
// - 被合并章节软删除：保留最终版本快照后删除
// - 禁止跨卷合并、禁止非连续章节合并
func (s *Store) MergeChapters(ctx context.Context, chapterIDs []string, newTitle string) (*Chapter, error) {
	if len(chapterIDs) < 2 {
		return nil, fmt.Errorf("至少选择2个章节进行合并")
	}
	// 按 sort_order 排序所有选中章节
	type chInfo struct {
		Chapter
		idx int
	}
	chapters := make([]chInfo, len(chapterIDs))
	for i, id := range chapterIDs {
		ch, err := s.GetChapter(ctx, id)
		if err != nil || ch == nil {
			return nil, fmt.Errorf("章节 %s 不存在", id)
		}
		chapters[i] = chInfo{*ch, i}
	}
	// 排序
	for i := 0; i < len(chapters); i++ {
		for j := i + 1; j < len(chapters); j++ {
			if chapters[i].SortOrder > chapters[j].SortOrder {
				chapters[i], chapters[j] = chapters[j], chapters[i]
			}
		}
	}
	// 校验：同卷、连续
	vid := chapters[0].VolumeID
	for i, c := range chapters {
		if c.VolumeID != vid {
			return nil, fmt.Errorf("禁止跨卷合并，请选择同一卷内的章节")
		}
		if i > 0 && c.SortOrder != chapters[i-1].SortOrder+1 {
			return nil, fmt.Errorf("只能合并连续章节，\"%s\" 与 \"%s\" 之间存在间隔", chapters[i-1].Title, c.Title)
		}
	}
	// 为每个被合并章节保存最终版本后软删除
	for _, c := range chapters {
		_, _ = s.SaveChapterVersion(ctx, c.ID, c.Title+" (合并前)", c.Content)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	projectID := chapters[0].ProjectID
	volumeID := chapters[0].VolumeID
	minOrd := chapters[0].SortOrder
	var merged strings.Builder
	for i, c := range chapters {
		if i > 0 {
			merged.WriteString("\n\n---\n\n")
		}
		merged.WriteString(c.Content)
	}
	// 软删除旧章节（版本已保存）
	ids := make([]string, len(chapters))
	for i, c := range chapters {
		ids[i] = c.ID
	}
	placeholders := strings.Repeat("?,", len(ids)-1) + "?"
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE chapters SET is_deleted=1, deleted_at=? WHERE id IN (`+placeholders+`) AND project_id=?`,
		append([]interface{}{now()}, args...)...); err != nil {
		return nil, err
	}

	wc := wordCount(merged.String())
	newID := newID()
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO chapters(id, project_id, volume_id, title, content, word_count, sort_order, tags, synopsis, created_at, updated_at, is_deleted, deleted_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,0,'')`,
		newID, projectID, volumeID, newTitle, merged.String(), wc, minOrd, "", "", now(), now()); err != nil {
		return nil, err
	}
	// 为新章节创建初始版本
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO chapter_versions(id, chapter_id, title, content, version, created_at) VALUES(?,?,?,?,1,?)`,
		newID, newID, newTitle, merged.String(), now()); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetChapter(ctx, newID)
}

// SplitChapter 从光标位置拆分章节
// cursorPos 之前内容保留在当前章节；之后内容生成新章节，归属同一卷
// 禁止在文本首尾拆分；拆分完成后两个章节均自动保存版本快照
func (s *Store) SplitChapter(ctx context.Context, chapterID string, cursorPos int) ([]Chapter, error) {
	ch, err := s.GetChapter(ctx, chapterID)
	if err != nil || ch == nil {
		return nil, fmt.Errorf("章节不存在")
	}
	runes := []rune(ch.Content)
	totalLen := len(runes)
	if cursorPos <= 0 || cursorPos >= totalLen {
		return nil, fmt.Errorf("不能在文本首尾拆分（光标应在 1 到 %d 之间）", totalLen-1)
	}
	part1 := strings.TrimSpace(string(runes[:cursorPos]))
	part2 := strings.TrimSpace(string(runes[cursorPos:]))
	if part1 == "" || part2 == "" {
		return nil, fmt.Errorf("拆分后某段为空，请调整光标位置")
	}
	newTitle1 := ch.Title
	newTitle2 := ch.Title + "(续)"

	// 保存原始章节的最终版本
	_, _ = s.SaveChapterVersion(ctx, chapterID, ch.Title+" (拆分前)", ch.Content)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	// 软删除原始章节（拆分前版本已保存）
	if _, err := tx.ExecContext(ctx, `UPDATE chapters SET is_deleted=1, deleted_at=? WHERE id=?`, now(), chapterID); err != nil {
		return nil, err
	}

	// 创建两个新章节
	wc1, wc2 := wordCount(part1), wordCount(part2)
	id1, id2 := newID(), newID()
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO chapters(id, project_id, volume_id, title, content, word_count, sort_order, tags, synopsis, created_at, updated_at, is_deleted, deleted_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,0,'')`,
		id1, ch.ProjectID, ch.VolumeID, newTitle1, part1, wc1, ch.SortOrder, ch.Tags, ch.Synopsis, now(), now()); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO chapters(id, project_id, volume_id, title, content, word_count, sort_order, tags, synopsis, created_at, updated_at, is_deleted, deleted_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,0,'')`,
		id2, ch.ProjectID, ch.VolumeID, newTitle2, part2, wc2, ch.SortOrder+1, ch.Tags, ch.Synopsis, now(), now()); err != nil {
		return nil, err
	}
	// 为两个新章节创建初始版本快照
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO chapter_versions(id, chapter_id, title, content, version, created_at) VALUES(?,?,?,?,1,?)`,
		newID(), id1, newTitle1, part1, now()); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO chapter_versions(id, chapter_id, title, content, version, created_at) VALUES(?,?,?,?,1,?)`,
		newID(), id2, newTitle2, part2, now()); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	ch1, _ := s.GetChapter(ctx, id1)
	ch2, _ := s.GetChapter(ctx, id2)
	return []Chapter{*ch1, *ch2}, nil
}

// CreateChapterWithVersion 创建章节并自动保存初始版本快照
func (s *Store) CreateChapterWithVersion(ctx context.Context, projectID, volumeID, title, content string) (*Chapter, error) {
	ch, err := s.CreateChapter(ctx, projectID, volumeID, title, content)
	if err != nil {
		return nil, err
	}
	_, _ = s.SaveChapterVersion(ctx, ch.ID, title+" (初始)", content)
	return ch, nil
}
