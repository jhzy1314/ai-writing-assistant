package rag

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ai-novel/studio/internal/infrastructure/database"
)

// ============================================================
// rag/service.go —— RAG 索引与检索服务
// 章节内容分块 -> 向量化 -> SQLite 存储 -> 语义检索
// ============================================================

// Chunk 向量块
type Chunk struct {
	ID        string  `json:"id"`
	ProjectID string  `json:"project_id"`
	ChapterID string  `json:"chapter_id"`
	ChapterNo int     `json:"chapter_no"`
	Title     string  `json:"title"`
	Text      string  `json:"text"`
	Vector    Vector  `json:"-"`
}

// Service RAG 服务
type Service struct {
	store *database.Store
}

// NewService 构造 RAG 服务
func NewService(store *database.Store) *Service {
	return &Service{store: store}
}

// ChunkSize 分块大小（字符）
const ChunkSize = 400

// 分块重叠（保留上下文衔接）
const chunkOverlap = 60

// IndexChapters 为项目所有章节建索引（幂等：先清旧索引再重建）
func (s *Service) IndexChapters(ctx context.Context, projectID string) error {
	chs, err := s.store.ListChapters(ctx, projectID, "")
	if err != nil {
		return fmt.Errorf("加载章节失败: %w", err)
	}
	// 清空旧索引
	if err := s.store.ClearRAGChunks(ctx, projectID); err != nil {
		return fmt.Errorf("清理旧索引失败: %w", err)
	}
	for i, ch := range chs {
		if strings.TrimSpace(ch.Content) == "" {
			continue
		}
		blocks := splitBlocks(ch.Content)
		for _, blk := range blocks {
			vec := Embed(blk)
			if len(vec) == 0 {
				continue
			}
			data, err := vec.Serialize()
			if err != nil {
				continue
			}
			if err := s.store.SaveRAGChunk(ctx, database.RAGChunk{
				ProjectID: projectID,
				ChapterID: ch.ID,
				ChapterNo: i + 1,
				Title:     ch.Title,
				Text:      blk,
				Vector:    string(data),
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

// IndexChapter 为单个章节建索引（增量，章节更新时调用）
func (s *Service) IndexChapter(ctx context.Context, projectID, chapterID string) error {
	ch, err := s.store.GetChapter(ctx, chapterID)
	if err != nil || ch == nil {
		return err
	}
	// 查章节序号
	chs, _ := s.store.ListChapters(ctx, projectID, "")
	no := 0
	for i, c := range chs {
		if c.ID == chapterID {
			no = i + 1
			break
		}
	}
	// 删除该章节旧块
	if err := s.store.DeleteRAGChunksByChapter(ctx, chapterID); err != nil {
		return err
	}
	if strings.TrimSpace(ch.Content) == "" {
		return nil
	}
	for _, blk := range splitBlocks(ch.Content) {
		vec := Embed(blk)
		if len(vec) == 0 {
			continue
		}
		data, _ := vec.Serialize()
		if err := s.store.SaveRAGChunk(ctx, database.RAGChunk{
			ProjectID: projectID,
			ChapterID: chapterID,
			ChapterNo: no,
			Title:     ch.Title,
			Text:      blk,
			Vector:    string(data),
		}); err != nil {
			return err
		}
	}
	return nil
}

// Search 语义检索：查询向量化 -> 与所有块余弦相似度 -> top-k
// excludeChapterID 排除当前章节（避免注入前文重复）
func (s *Service) Search(ctx context.Context, projectID, excludeChapterID, query string, topK int) ([]database.RAGChunk, error) {
	qvec := Embed(query)
	if len(qvec) == 0 {
		return nil, nil
	}
	chunks, err := s.store.ListRAGChunks(ctx, projectID)
	if err != nil {
		return nil, err
	}
	type scored struct {
		chunk database.RAGChunk
		score float64
	}
	var results []scored
	for _, c := range chunks {
		if excludeChapterID != "" && c.ChapterID == excludeChapterID {
			continue
		}
		vec, err := Deserialize([]byte(c.Vector))
		if err != nil {
			continue
		}
		score := Cosine(qvec, vec)
		if score > 0.05 { // 低相似度过滤
			results = append(results, scored{c, score})
		}
	}
	// 按分数降序
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].score > results[i].score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}
	if len(results) > topK {
		results = results[:topK]
	}
	out := make([]database.RAGChunk, 0, len(results))
	for _, r := range results {
		out = append(out, r.chunk)
	}
	return out, nil
}

// BuildContextText 将检索结果拼装为注入文本
func BuildContextText(chunks []database.RAGChunk) string {
	var b strings.Builder
	for _, c := range chunks {
		title := c.Title
		if title == "" {
			title = "未命名"
		}
		b.WriteString(fmt.Sprintf("【第%d章 %s（相关片段）】\n%s\n\n", c.ChapterNo, title, c.Text))
	}
	return strings.TrimSpace(b.String())
}

// splitBlocks 按段落分块，块大小 ChunkSize，带重叠
func splitBlocks(text string) []string {
	// 先按段落切（\n\n 或 \n）
	paras := strings.FieldsFunc(text, func(r rune) bool { return r == '\n' })
	var blocks []string
	var cur strings.Builder
	for _, p := range paras {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if cur.Len() > 0 && cur.Len()+len(p) > ChunkSize {
			blocks = append(blocks, strings.TrimSpace(cur.String()))
			// 重叠：保留末尾 chunkOverlap 字符
			curStr := cur.String()
			runes := []rune(curStr)
			if len(runes) > chunkOverlap {
				cur.Reset()
				cur.WriteString(string(runes[len(runes)-chunkOverlap:]))
			} else {
				cur.Reset()
			}
		}
		cur.WriteString(p)
		cur.WriteString("\n")
	}
	if cur.Len() > 0 {
		blocks = append(blocks, strings.TrimSpace(cur.String()))
	}
	// 超长段落内部再切
	var out []string
	for _, b := range blocks {
		if len([]rune(b)) <= ChunkSize {
			out = append(out, b)
			continue
		}
		runes := []rune(b)
		for i := 0; i < len(runes); i += ChunkSize - chunkOverlap {
			end := i + ChunkSize
			if end > len(runes) {
				end = len(runes)
			}
			out = append(out, string(runes[i:end]))
		}
	}
	return out
}

var _ = json.Marshal // keep import
