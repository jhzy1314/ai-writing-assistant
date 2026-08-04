package database

import (
	"context"
	"errors"
	"strings"
	"time"
)

// errSystemTemplate 系统内置模板不可删除/修改的哨兵错误
var errSystemTemplate = errors.New("系统内置模板不可删除或修改")

// GenerationLog generation_logs 表一行
type GenerationLog struct {
	ID               string `json:"id"`
	ProjectID        string `json:"project_id"`
	Role             string `json:"role"`
	ModelName        string `json:"model_name"`
	PromptTokens     int    `json:"prompt_tokens"`
	CompletionTokens int    `json:"completion_tokens"`
	CacheHitTokens   int    `json:"cache_hit_tokens"` // 前缀缓存命中 token
	DurationMs       int    `json:"duration_ms"`
	Status           string `json:"status"`
	ErrorMsg         string `json:"error_msg"`
	CreatedAt        string `json:"created_at"`
}

// InsertLog 写入一条模型调用日志
func (s *Store) InsertLog(ctx context.Context, log GenerationLog) error {
	if log.ID == "" {
		log.ID = newID()
	}
	if log.Status == "" {
		log.Status = "ok"
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO generation_logs(id, project_id, role, model_name, prompt_tokens, completion_tokens, cache_hit_tokens, duration_ms, status, error_msg, created_at)
		 VALUES(?,?,?,?,?,?,?,?,?,?,?)`,
		log.ID, log.ProjectID, log.Role, log.ModelName, log.PromptTokens,
		log.CompletionTokens, log.CacheHitTokens, log.DurationMs, log.Status, log.ErrorMsg, now())
	return err
}

// ListLogs 按项目或模型查询日志（支持分页）
func (s *Store) ListLogs(ctx context.Context, projectID string, limit, offset int) ([]GenerationLog, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	q := `SELECT id, project_id, role, model_name, prompt_tokens, completion_tokens, COALESCE(cache_hit_tokens,0), duration_ms, status, error_msg, created_at FROM generation_logs`
	args := []interface{}{}
	if projectID != "" {
		q += ` WHERE project_id=?`
		args = append(args, projectID)
	}
	q += ` ORDER BY created_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []GenerationLog{}
	for rows.Next() {
		var l GenerationLog
		if err := rows.Scan(&l.ID, &l.ProjectID, &l.Role, &l.ModelName, &l.PromptTokens,
			&l.CompletionTokens, &l.CacheHitTokens, &l.DurationMs, &l.Status, &l.ErrorMsg, &l.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// CleanOldLogs 清理30天前的调用日志
func (s *Store) CleanOldLogs(ctx context.Context) error {
	cutoff := time.Now().AddDate(0, 0, -30).Format("2006-01-02")
	_, err := s.db.ExecContext(ctx, `DELETE FROM generation_logs WHERE created_at < ?`, cutoff)
	return err
}

// SearchChapters 全文搜索项目章节，返回匹配结果
func (s *Store) SearchChapters(ctx context.Context, projectID, query string) ([]ChapterSearchResult, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, title, content, sort_order FROM chapters WHERE project_id=? AND content LIKE '%'||?||'%' ORDER BY sort_order`, projectID, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []ChapterSearchResult
	for rows.Next() {
		var r ChapterSearchResult
		var sortOrder int
		if err := rows.Scan(&r.ID, &r.Title, &r.Content, &sortOrder); err != nil {
			return nil, err
		}
		r.Pos = strings.Index(r.Content, query)
		r.Snippet = r.Content[max(0, r.Pos-30):min(len(r.Content), r.Pos+len(query)+30)]
		results = append(results, r)
	}
	return results, rows.Err()
}

// ChapterSearchResult 章节搜索结果
type ChapterSearchResult struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Content string `json:"-"`
	Pos     int    `json:"pos"`
	Snippet string `json:"snippet"`
}

// IsSystemTemplateError 判断是否系统模板不可操作错误
func IsSystemTemplateError(err error) bool { return errors.Is(err, errSystemTemplate) }
