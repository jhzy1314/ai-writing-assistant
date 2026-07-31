package database

import (
	"context"
	"strings"
)

// ============================================================
// rag.go —— RAG 向量块存储（表 rag_chunks）
// ============================================================

// RAGChunk 向量块行
type RAGChunk struct {
	ID        string `json:"id"`
	ProjectID string `json:"project_id"`
	ChapterID string `json:"chapter_id"`
	ChapterNo int    `json:"chapter_no"`
	Title     string `json:"title"`
	Text      string `json:"text"`
	Vector    string `json:"vector"`
}

// SaveRAGChunk 保存一个向量块
func (s *Store) SaveRAGChunk(ctx context.Context, c RAGChunk) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO rag_chunks(id, project_id, chapter_id, chapter_no, title, text, vector)
		VALUES(?,?,?,?,?,?,?)`,
		newID(), c.ProjectID, c.ChapterID, c.ChapterNo, c.Title, c.Text, c.Vector)
	return err
}

// ListRAGChunks 列出项目全部向量块
func (s *Store) ListRAGChunks(ctx context.Context, projectID string) ([]RAGChunk, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, project_id, chapter_id, chapter_no, title, text, vector
		FROM rag_chunks WHERE project_id=? ORDER BY chapter_no, rowid`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RAGChunk
	for rows.Next() {
		var c RAGChunk
		if err := rows.Scan(&c.ID, &c.ProjectID, &c.ChapterID, &c.ChapterNo, &c.Title, &c.Text, &c.Vector); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ClearRAGChunks 清空项目全部向量块
func (s *Store) ClearRAGChunks(ctx context.Context, projectID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM rag_chunks WHERE project_id=?`, projectID)
	return err
}

// DeleteRAGChunksByChapter 删除某章节的向量块
func (s *Store) DeleteRAGChunksByChapter(ctx context.Context, chapterID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM rag_chunks WHERE chapter_id=?`, chapterID)
	return err
}

// CountRAGChunks 统计项目向量块数量（判断是否已建索引）
func (s *Store) CountRAGChunks(ctx context.Context, projectID string) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM rag_chunks WHERE project_id=?`, projectID).Scan(&n)
	if err != nil {
		// 表不存在（旧库）时返回 0
		if strings.Contains(err.Error(), "no such table") {
			return 0, nil
		}
		return 0, err
	}
	return n, nil
}
