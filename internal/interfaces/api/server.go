package api

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ai-novel/studio/ent"
	"github.com/ai-novel/studio/ent/chapter"
	ent_character "github.com/ai-novel/studio/ent/character"
	"github.com/ai-novel/studio/ent/novel"
	"github.com/ai-novel/studio/internal/application/workflows"
	"github.com/ai-novel/studio/internal/domain/agents"
	"github.com/ai-novel/studio/internal/domain/events"
	"github.com/ai-novel/studio/internal/infrastructure/config"
	infra_llm "github.com/ai-novel/studio/internal/infrastructure/llm"
	webstatic "github.com/ai-novel/studio/web"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type Server struct {
	engine    *workflows.WorkflowEngine
	eventBus  events.Bus
	db        *ent.Client
	llm       agents.LLMService
	reviewLLM agents.LLMService
	router    *chi.Mux
}

func NewServer(engine *workflows.WorkflowEngine, eventBus events.Bus, db *ent.Client, llm agents.LLMService, reviewLLM agents.LLMService) *Server {
	s := &Server{
		engine:    engine,
		eventBus:  eventBus,
		db:        db,
		llm:       llm,
		reviewLLM: reviewLLM,
		router:    chi.NewRouter(),
	}

	s.router.Use(middleware.Logger)
	s.router.Use(middleware.Recoverer)

	s.router.Get("/api/v1/novels", s.HandleListNovels)
	s.router.Post("/api/v1/novels", s.HandleCreateNovel)
	s.router.Options("/api/v1/novels", s.HandleOptions)
	s.router.Get("/api/v1/novels/{id}", s.HandleGetNovel)
	s.router.Put("/api/v1/novels/{id}", s.HandleUpdateNovel)
	s.router.Options("/api/v1/novels/{id}", s.HandleOptions)
	s.router.Get("/api/v1/novels/{id}/chapters", s.HandleListChapters)
	s.router.Post("/api/v1/novels/{id}/chapters", s.HandleCreateChapter)
	s.router.Options("/api/v1/novels/{id}/chapters", s.HandleOptions)
	s.router.Get("/api/v1/chapters/{id}", s.HandleGetChapter)
	s.router.Put("/api/v1/chapters/{id}", s.HandleUpdateChapter)
	s.router.Delete("/api/v1/chapters/{id}", s.HandleDeleteChapter)
	s.router.Post("/api/v1/chapters/{id}/generate-title", s.HandleGenerateTitle)
	s.router.Post("/api/v1/chapters/{id}/review", s.HandleReviewChapter)
	s.router.Post("/api/v1/chapters/{id}/rewrite", s.HandleRewriteChapter)
	s.router.Get("/api/v1/settings", s.HandleGetSettings)
	s.router.Put("/api/v1/settings", s.HandleUpdateSettings)
	s.router.Options("/api/v1/settings", s.HandleOptions)
	s.router.Options("/api/v1/chapters/{id}", s.HandleOptions)
	s.router.Options("/api/v1/chapters/{id}/generate-title", s.HandleOptions)
	s.router.Options("/api/v1/chapters/{id}/review", s.HandleOptions)
	s.router.Get("/api/v1/novel/generate", s.HandleGenerateChapter)
	s.router.Get("/api/v1/novel/preview-context", s.HandlePreviewContext)
	s.router.Post("/api/v1/novels/{id}/import", s.HandleImportChapters)
	s.router.Post("/api/v1/novels/{id}/extract-docx", s.HandleExtractDocx)
	s.router.Get("/api/v1/novels/{id}/characters", s.HandleListCharacters)
	s.router.Post("/api/v1/novels/{id}/characters", s.HandleCreateCharacter)
	s.router.Post("/api/v1/novels/{id}/analyze-characters", s.HandleAnalyzeCharacters)
	s.router.Put("/api/v1/characters/{id}", s.HandleUpdateCharacter)
	s.router.Delete("/api/v1/characters/{id}", s.HandleDeleteCharacter)
	s.router.Options("/api/v1/novels/{id}/import", s.HandleOptions)
	s.router.Options("/api/v1/novels/{id}/extract-docx", s.HandleOptions)

	// Static file serving for the web frontend (embedded)
	indexHTML, _ := fs.ReadFile(webstatic.FS, "index.html")
	if indexHTML == nil {
		indexHTML = []byte("<html><body>index.html not embedded</body></html>")
	}
	s.router.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(indexHTML)
	})
	s.router.Handle("/*", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(indexHTML)
	}))

	return s
}

type NovelSummary struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description,omitempty"`
	Status      string    `json:"status"`
	Tags        []string  `json:"tags,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type NovelDetail struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description,omitempty"`
	Idea        string    `json:"idea,omitempty"`
	Outline     string    `json:"outline,omitempty"`
	Status      string    `json:"status"`
	Tags        []string  `json:"tags,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type ChapterItem struct {
	ID        string    `json:"id"`
	NovelID   string    `json:"novel_id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	WordCount int       `json:"word_count"`
	Order     int       `json:"order"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type CreateNovelRequest struct {
	Title       string   `json:"title"`
	Description string   `json:"description,omitempty"`
	Type        string   `json:"type,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

type UpdateNovelRequest struct {
	Idea    *string `json:"idea,omitempty"`
	Outline *string `json:"outline,omitempty"`
}

type CreateChapterRequest struct {
	Title   string `json:"title,omitempty"`
	Content string `json:"content,omitempty"`
	Order   int    `json:"order,omitempty"`
	Status  string `json:"status,omitempty"`
}

type UpdateChapterRequest struct {
	Title   *string `json:"title,omitempty"`
	Content *string `json:"content,omitempty"`
	Order   *int    `json:"order,omitempty"`
	Status  *string `json:"status,omitempty"`
}

func (s *Server) HandleListNovels(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if s.db == nil {
		http.Error(w, "database not configured", http.StatusInternalServerError)
		return
	}

	rows, err := s.db.Novel.
		Query().
		Order(ent.Desc(novel.FieldUpdatedAt), ent.Desc(novel.FieldCreatedAt)).
		All(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	items := make([]NovelSummary, 0, len(rows))
	for _, n := range rows {
		items = append(items, NovelSummary{
			ID:          fmt.Sprintf("%d", n.ID),
			Title:       n.Title,
			Description: n.Description,
			Status:      n.Status,
			Tags:        n.Tags,
			CreatedAt:   n.CreatedAt,
			UpdatedAt:   n.UpdatedAt,
		})
	}

	_ = json.NewEncoder(w).Encode(map[string]any{"items": items})
}

func (s *Server) HandleOptions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept")
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) HandleCreateNovel(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if s.db == nil {
		http.Error(w, "database not configured", http.StatusInternalServerError)
		return
	}

	var req CreateNovelRequest
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("invalid json: %v", err), http.StatusBadRequest)
		return
	}

	title := strings.TrimSpace(req.Title)
	if title == "" {
		http.Error(w, "title is required", http.StatusBadRequest)
		return
	}

	description := strings.TrimSpace(req.Description)
	novelType := strings.TrimSpace(req.Type)
	tags := make([]string, 0, len(req.Tags)+1)
	if novelType != "" {
		tags = append(tags, novelType)
	}
	for _, t := range req.Tags {
		tt := strings.TrimSpace(t)
		if tt == "" {
			continue
		}
		if novelType != "" && tt == novelType {
			continue
		}
		tags = append(tags, tt)
	}

	row, err := s.db.Novel.
		Create().
		SetTitle(title).
		SetDescription(description).
		SetIdea("").
		SetOutline("").
		SetStatus("Draft").
		SetTags(tags).
		Save(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	item := NovelSummary{
		ID:          fmt.Sprintf("%d", row.ID),
		Title:       row.Title,
		Description: row.Description,
		Status:      row.Status,
		Tags:        row.Tags,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}

	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{"item": item})
}

func (s *Server) HandleGetNovel(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if s.db == nil {
		http.Error(w, "database not configured", http.StatusInternalServerError)
		return
	}

	id, parseErr := parseIntParam(chi.URLParam(r, "id"))
	if parseErr != nil {
		http.Error(w, parseErr.Error(), http.StatusBadRequest)
		return
	}

	row, err := s.db.Novel.
		Query().
		Where(novel.ID(id)).
		WithChapters(func(q *ent.ChapterQuery) {
			q.Order(ent.Asc(chapter.FieldOrder), ent.Asc(chapter.FieldCreatedAt))
		}).
		Only(r.Context())
	if err != nil {
		if ent.IsNotFound(err) {
			http.Error(w, "novel not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	item := NovelDetail{
		ID:          fmt.Sprintf("%d", row.ID),
		Title:       row.Title,
		Description: row.Description,
		Idea:        row.Idea,
		Outline:     row.Outline,
		Status:      row.Status,
		Tags:        row.Tags,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}

	chapters := make([]ChapterItem, 0, len(row.Edges.Chapters))
	for _, c := range row.Edges.Chapters {
		chapters = append(chapters, ChapterItem{
			ID:        fmt.Sprintf("%d", c.ID),
			NovelID:   item.ID,
			Title:     c.Title,
			Content:   c.Content,
			WordCount: c.WordCount,
			Order:     c.Order,
			Status:    c.Status,
			CreatedAt: c.CreatedAt,
			UpdatedAt: c.UpdatedAt,
		})
	}

	_ = json.NewEncoder(w).Encode(map[string]any{
		"item":     item,
		"chapters": chapters,
	})
}

func (s *Server) HandleUpdateNovel(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if s.db == nil {
		http.Error(w, "database not configured", http.StatusInternalServerError)
		return
	}

	id, parseErr := parseIntParam(chi.URLParam(r, "id"))
	if parseErr != nil {
		http.Error(w, parseErr.Error(), http.StatusBadRequest)
		return
	}

	var req UpdateNovelRequest
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 5<<20))
	dec.DisallowUnknownFields()
	if decodeErr := dec.Decode(&req); decodeErr != nil {
		http.Error(w, fmt.Sprintf("invalid json: %v", decodeErr), http.StatusBadRequest)
		return
	}

	upd := s.db.Novel.UpdateOneID(id)
	if req.Idea != nil {
		upd.SetIdea(strings.TrimSpace(*req.Idea))
	}
	if req.Outline != nil {
		upd.SetOutline(strings.TrimSpace(*req.Outline))
	}

	row, saveErr := upd.Save(r.Context())
	if saveErr != nil {
		if ent.IsNotFound(saveErr) {
			http.Error(w, "novel not found", http.StatusNotFound)
			return
		}
		http.Error(w, saveErr.Error(), http.StatusInternalServerError)
		return
	}

	item := NovelDetail{
		ID:          fmt.Sprintf("%d", row.ID),
		Title:       row.Title,
		Description: row.Description,
		Idea:        row.Idea,
		Outline:     row.Outline,
		Status:      row.Status,
		Tags:        row.Tags,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}

	_ = json.NewEncoder(w).Encode(map[string]any{"item": item})
}

func (s *Server) HandleListChapters(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if s.db == nil {
		http.Error(w, "database not configured", http.StatusInternalServerError)
		return
	}

	novelID, err := parseIntParam(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	limit := 50
	offset := 0
	if v := strings.TrimSpace(r.URL.Query().Get("limit")); v != "" {
		if n, convErr := strconv.Atoi(v); convErr == nil && n > 0 && n <= 200 {
			limit = n
		}
	}
	if v := strings.TrimSpace(r.URL.Query().Get("offset")); v != "" {
		if n, convErr := strconv.Atoi(v); convErr == nil && n >= 0 {
			offset = n
		}
	}

	rows, err := s.db.Chapter.
		Query().
		Where(chapter.HasNovelWith(novel.ID(novelID))).
		Order(ent.Asc(chapter.FieldOrder), ent.Asc(chapter.FieldCreatedAt)).
		Limit(limit).
		Offset(offset).
		All(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	items := make([]ChapterItem, 0, len(rows))
	for _, c := range rows {
		items = append(items, ChapterItem{
			ID:        fmt.Sprintf("%d", c.ID),
			NovelID:   fmt.Sprintf("%d", novelID),
			Title:     c.Title,
			Content:   c.Content,
			WordCount: c.WordCount,
			Order:     c.Order,
			Status:    c.Status,
			CreatedAt: c.CreatedAt,
			UpdatedAt: c.UpdatedAt,
		})
	}

	_ = json.NewEncoder(w).Encode(map[string]any{"items": items})
}

func (s *Server) HandleGetChapter(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if s.db == nil {
		http.Error(w, "database not configured", http.StatusInternalServerError)
		return
	}

	id, err := parseIntParam(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	row, err := s.db.Chapter.
		Query().
		Where(chapter.ID(id)).
		WithNovel().
		Only(r.Context())
	if err != nil {
		if ent.IsNotFound(err) {
			http.Error(w, "chapter not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	novelID := ""
	if row.Edges.Novel != nil {
		novelID = fmt.Sprintf("%d", row.Edges.Novel.ID)
	}

	item := ChapterItem{
		ID:        fmt.Sprintf("%d", row.ID),
		NovelID:   novelID,
		Title:     row.Title,
		Content:   row.Content,
		WordCount: row.WordCount,
		Order:     row.Order,
		Status:    row.Status,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}

	_ = json.NewEncoder(w).Encode(map[string]any{"item": item})
}

func (s *Server) HandleCreateChapter(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if s.db == nil {
		http.Error(w, "database not configured", http.StatusInternalServerError)
		return
	}

	novelID, err := parseIntParam(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var req CreateChapterRequest
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 5<<20))
	dec.DisallowUnknownFields()
	if decodeErr := dec.Decode(&req); decodeErr != nil {
		http.Error(w, fmt.Sprintf("invalid json: %v", decodeErr), http.StatusBadRequest)
		return
	}

	order := req.Order
	if order <= 0 {
		last, queryErr := s.db.Chapter.
			Query().
			Where(chapter.HasNovelWith(novel.ID(novelID))).
			Order(ent.Desc(chapter.FieldOrder)).
			First(r.Context())
		if queryErr == nil && last != nil {
			order = last.Order + 1
		} else {
			order = 1
		}
	}

	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = chapterTitle(order)
	}
	content := req.Content
	status := strings.TrimSpace(req.Status)
	if status == "" {
		status = "Draft"
	}

	row, err := s.db.Chapter.
		Create().
		SetNovelID(novelID).
		SetTitle(title).
		SetContent(content).
		SetWordCount(wordCountOf(content)).
		SetOrder(order).
		SetStatus(status).
		Save(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	item := ChapterItem{
		ID:        fmt.Sprintf("%d", row.ID),
		NovelID:   fmt.Sprintf("%d", novelID),
		Title:     row.Title,
		Content:   row.Content,
		WordCount: row.WordCount,
		Order:     row.Order,
		Status:    row.Status,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}

	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{"item": item})
}

func (s *Server) HandleUpdateChapter(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if s.db == nil {
		http.Error(w, "database not configured", http.StatusInternalServerError)
		return
	}

	id, err := parseIntParam(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var req UpdateChapterRequest
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 10<<20))
	dec.DisallowUnknownFields()
	if decodeErr := dec.Decode(&req); decodeErr != nil {
		http.Error(w, fmt.Sprintf("invalid json: %v", decodeErr), http.StatusBadRequest)
		return
	}

	upd := s.db.Chapter.UpdateOneID(id)
	if req.Title != nil {
		upd.SetTitle(strings.TrimSpace(*req.Title))
	}
	if req.Order != nil {
		if *req.Order <= 0 {
			http.Error(w, "order must be > 0", http.StatusBadRequest)
			return
		}
		upd.SetOrder(*req.Order)
	}
	if req.Status != nil {
		upd.SetStatus(strings.TrimSpace(*req.Status))
	}
	if req.Content != nil {
		upd.SetContent(*req.Content)
		upd.SetWordCount(wordCountOf(*req.Content))
	}

	row, saveErr := upd.Save(r.Context())
	if saveErr != nil {
		if ent.IsNotFound(saveErr) {
			http.Error(w, "chapter not found", http.StatusNotFound)
			return
		}
		http.Error(w, saveErr.Error(), http.StatusInternalServerError)
		return
	}

	novelID := ""
	n, queryErr := row.QueryNovel().Only(r.Context())
	if queryErr == nil && n != nil {
		novelID = fmt.Sprintf("%d", n.ID)
	}

	item := ChapterItem{
		ID:        fmt.Sprintf("%d", row.ID),
		NovelID:   novelID,
		Title:     row.Title,
		Content:   row.Content,
		WordCount: row.WordCount,
		Order:     row.Order,
		Status:    row.Status,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}

	_ = json.NewEncoder(w).Encode(map[string]any{"item": item})
}

func (s *Server) HandleDeleteChapter(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if s.db == nil {
		http.Error(w, "database not configured", http.StatusInternalServerError)
		return
	}

	id, err := parseIntParam(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := s.db.Chapter.DeleteOneID(id).Exec(r.Context()); err != nil {
		if ent.IsNotFound(err) {
			http.Error(w, "chapter not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) HandleGenerateTitle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if s.db == nil {
		http.Error(w, "database not configured", http.StatusInternalServerError)
		return
	}
	if s.llm == nil {
		http.Error(w, "LLM not configured", http.StatusInternalServerError)
		return
	}

	id, err := parseIntParam(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	row, err := s.db.Chapter.Query().Where(chapter.ID(id)).Only(r.Context())
	if err != nil {
		http.Error(w, "chapter not found", http.StatusNotFound)
		return
	}

	content := row.Content
	if strings.TrimSpace(content) == "" {
		http.Error(w, "chapter has no content to generate title from", http.StatusBadRequest)
		return
	}

	// Trim content to avoid token limits
	preview := content
	if len([]rune(preview)) > 3000 {
		preview = string([]rune(preview)[:3000])
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	title, genErr := s.llm.Generate(ctx,
		"你是一位专业的小说编辑。请根据以下章节内容，生成一个精炼的中文章节标题（5-15个字）。只返回标题本身，不要任何解释或标点。",
		fmt.Sprintf("章节内容：\n%s\n\n请为这个章节起一个标题：", preview),
	)
	if genErr != nil {
		http.Error(w, fmt.Sprintf("title generation failed: %v", genErr), http.StatusInternalServerError)
		return
	}

	title = strings.TrimSpace(title)
	if title == "" {
		title = row.Title
	}

	// Save the title
	updated, saveErr := s.db.Chapter.UpdateOneID(id).SetTitle(title).Save(r.Context())
	if saveErr != nil {
		http.Error(w, fmt.Sprintf("failed to save title: %v", saveErr), http.StatusInternalServerError)
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]any{
		"title":      updated.Title,
		"chapter_id": fmt.Sprintf("%d", updated.ID),
	})
}

func (s *Server) HandleReviewChapter(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if s.db == nil {
		http.Error(w, "database not configured", http.StatusInternalServerError)
		return
	}
	revLLM := s.reviewLLM
	if revLLM == nil {
		revLLM = s.llm
	}
	if revLLM == nil {
		http.Error(w, "LLM not configured", http.StatusInternalServerError)
		return
	}

	id, err := parseIntParam(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	row, err := s.db.Chapter.Query().Where(chapter.ID(id)).Only(r.Context())
	if err != nil {
		http.Error(w, "chapter not found", http.StatusNotFound)
		return
	}

	content := strings.TrimSpace(row.Content)
	if content == "" {
		http.Error(w, "chapter has no content to review", http.StatusBadRequest)
		return
	}

	preview := string([]rune(content))
	if len([]rune(preview)) > 5000 {
		preview = string([]rune(preview)[:5000])
	}

	systemPrompt := `你是一位资深小说编辑。请仔细审阅以下章节内容，从以下几个维度给出专业意见：

1. 【剧情节奏】情节推进是否流畅？有无拖沓或跳跃？
2. 【人物塑造】角色言行是否一致？形象是否鲜明？
3. 【文笔风格】语言是否生动？有无冗余或生硬的表述？
4. 【逻辑一致性】与常识或前文设定有无矛盾？
5. 【改进建议】一句话总结本章最大的优点和最需要改进的地方。

请用简洁专业的中文回复，每个维度2-3句话。`

	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
	defer cancel()

	review, genErr := revLLM.Generate(ctx, systemPrompt,
		fmt.Sprintf("请审阅以下章节内容：\n\n标题：%s\n字数：%d\n\n正文：\n%s",
			row.Title, row.WordCount, preview),
	)
	if genErr != nil {
		http.Error(w, fmt.Sprintf("review failed: %v", genErr), http.StatusInternalServerError)
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]any{
		"chapter_id": fmt.Sprintf("%d", row.ID),
		"title":      row.Title,
		"word_count": row.WordCount,
		"review":     strings.TrimSpace(review),
	})
}

func (s *Server) HandleRewriteChapter(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if s.db == nil { http.Error(w, "no db", http.StatusInternalServerError); return }
	if s.llm == nil { http.Error(w, "LLM not configured", http.StatusInternalServerError); return }

	id, err := parseIntParam(chi.URLParam(r, "id"))
	if err != nil { http.Error(w, err.Error(), http.StatusBadRequest); return }

	var req struct {
		Instruction string `json:"instruction"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest); return
	}
	if strings.TrimSpace(req.Instruction) == "" {
		http.Error(w, "instruction required", http.StatusBadRequest); return
	}

	row, err := s.db.Chapter.Query().Where(chapter.ID(id)).Only(r.Context())
	if err != nil { http.Error(w, "chapter not found", http.StatusNotFound); return }

	ctx, cancel := context.WithTimeout(r.Context(), 180*time.Second)
	defer cancel()

	rewritten, genErr := s.llm.Generate(ctx,
		`你是一位资深小说编辑和改写专家。请根据用户的修改指令，对以下小说章节进行改写。
要求：
- 保持原文的叙事风格和人设不崩坏。
- 精确理解用户指令，只修改相关部分，不改变未涉及的内容。
- 保持本章的字数大致不变。
- 仅返回改写后的完整正文，不要任何解释。`,
		fmt.Sprintf("【原文标题】%s\n\n【原文正文】\n%s\n\n【修改指令】\n%s\n\n请改写：", row.Title, row.Content, req.Instruction),
	)
	if genErr != nil { http.Error(w, genErr.Error(), http.StatusInternalServerError); return }

	if rewritten == "" { http.Error(w, "rewrite produced empty output", http.StatusInternalServerError); return }

	s.db.Chapter.UpdateOneID(id).
		SetContent(strings.TrimSpace(rewritten)).
		SetWordCount(wordCountOf(rewritten)).
		Exec(r.Context())

	_ = json.NewEncoder(w).Encode(map[string]any{
		"chapter_id": fmt.Sprintf("%d", row.ID),
		"title":      row.Title,
		"content":    rewritten,
		"word_count": wordCountOf(rewritten),
	})
}

// ===== CHARACTERS =====
type CharacterItem struct {
	ID            string    `json:"id"`
	NovelID       string    `json:"novel_id"`
	Name          string    `json:"name"`
	Gender        string    `json:"gender,omitempty"`
	Age           int       `json:"age,omitempty"`
	Appearance    string    `json:"appearance,omitempty"`
	Personality   string    `json:"personality,omitempty"`
	Background    string    `json:"background,omitempty"`
	CurrentStatus string    `json:"current_status,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

type CreateCharacterRequest struct {
	Name          string `json:"name"`
	Gender        string `json:"gender,omitempty"`
	Age           int    `json:"age,omitempty"`
	Appearance    string `json:"appearance,omitempty"`
	Personality   string `json:"personality,omitempty"`
	Background    string `json:"background,omitempty"`
	CurrentStatus string `json:"current_status,omitempty"`
}

func (s *Server) HandleListCharacters(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if s.db == nil { http.Error(w, "no db", http.StatusInternalServerError); return }

	novelID, err := parseIntParam(chi.URLParam(r, "id"))
	if err != nil { http.Error(w, err.Error(), http.StatusBadRequest); return }

	rows, err := s.db.Character.Query().
		Where(ent_character.NovelID(fmt.Sprintf("%d", novelID))).
		All(r.Context())
	if err != nil { http.Error(w, err.Error(), http.StatusInternalServerError); return }

	items := make([]CharacterItem, 0, len(rows))
	for _, c := range rows {
		items = append(items, toCharItem(c))
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"items": items})
}

func (s *Server) HandleCreateCharacter(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if s.db == nil { http.Error(w, "no db", http.StatusInternalServerError); return }

	novelID, err := parseIntParam(chi.URLParam(r, "id"))
	if err != nil { http.Error(w, err.Error(), http.StatusBadRequest); return }

	var req CreateCharacterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest); return
	}
	if strings.TrimSpace(req.Name) == "" {
		http.Error(w, "name required", http.StatusBadRequest); return
	}

	row, err := s.db.Character.Create().
		SetNovelID(fmt.Sprintf("%d", novelID)).
		SetName(strings.TrimSpace(req.Name)).
		SetGender(req.Gender).
		SetAge(req.Age).
		SetAppearance(req.Appearance).
		SetPersonality(req.Personality).
		SetBackground(req.Background).
		SetCurrentStatus(req.CurrentStatus).
		Save(r.Context())
	if err != nil { http.Error(w, err.Error(), http.StatusInternalServerError); return }

	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{"item": toCharItem(row)})
}

func (s *Server) HandleUpdateCharacter(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if s.db == nil { http.Error(w, "no db", http.StatusInternalServerError); return }

	id, err := parseIntParam(chi.URLParam(r, "id"))
	if err != nil { http.Error(w, err.Error(), http.StatusBadRequest); return }

	var req CreateCharacterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest); return
	}

	upd := s.db.Character.UpdateOneID(id)
	if req.Name != "" { upd.SetName(strings.TrimSpace(req.Name)) }
	if req.Gender != "" { upd.SetGender(req.Gender) }
	if req.Age > 0 { upd.SetAge(req.Age) }
	upd.SetAppearance(req.Appearance)
	upd.SetPersonality(req.Personality)
	upd.SetBackground(req.Background)
	upd.SetCurrentStatus(req.CurrentStatus)

	row, err := upd.Save(r.Context())
	if err != nil { http.Error(w, err.Error(), http.StatusInternalServerError); return }
	_ = json.NewEncoder(w).Encode(map[string]any{"item": toCharItem(row)})
}

func (s *Server) HandleDeleteCharacter(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if s.db == nil { http.Error(w, "no db", http.StatusInternalServerError); return }

	id, err := parseIntParam(chi.URLParam(r, "id"))
	if err != nil { http.Error(w, err.Error(), http.StatusBadRequest); return }

	if err := s.db.Character.DeleteOneID(id).Exec(r.Context()); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError); return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) HandleAnalyzeCharacters(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if s.db == nil { http.Error(w, "no db", http.StatusInternalServerError); return }
	if s.llm == nil { http.Error(w, "LLM not configured", http.StatusInternalServerError); return }

	novelID, err := parseIntParam(chi.URLParam(r, "id"))
	if err != nil { http.Error(w, err.Error(), http.StatusBadRequest); return }
	novelIDStr := fmt.Sprintf("%d", novelID)

	chapters, err := s.db.Chapter.Query().
		Where(chapter.HasNovelWith(novel.ID(novelID))).
		Order(ent.Asc(chapter.FieldOrder)).
		All(r.Context())
	if err != nil { http.Error(w, err.Error(), http.StatusInternalServerError); return }

	var allText strings.Builder
	for _, ch := range chapters {
		content := []rune(ch.Content)
		if len(content) > 1500 { content = content[:1500] }
		allText.WriteString(fmt.Sprintf("[%s] ", ch.Title))
		allText.WriteString(string(content))
		allText.WriteString("\n")
	}
	if allText.Len() == 0 {
		http.Error(w, "no chapter content", http.StatusBadRequest); return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 180*time.Second)
	defer cancel()

	result, err := s.llm.Generate(ctx,
		`你是小说人物分析师。请阅读以下小说内容，提取所有出场角色。
返回一个JSON数组（不要任何其他文字），每个元素格式：
{"name":"角色名","gender":"男/女/未知","age":25,"appearance":"外貌特征","personality":"性格特点","background":"背景经历","current_status":"当前状态"}
年龄请用数字，不确定则填0。请严格返回JSON数组。`,
		fmt.Sprintf("小说内容：\n%s", allText.String()),
	)
	if err != nil { http.Error(w, err.Error(), http.StatusInternalServerError); return }

	// Parse JSON from LLM response
	result = strings.TrimSpace(result)
	if idx := strings.Index(result, "["); idx >= 0 {
		result = result[idx:]
	}
	if idx := strings.LastIndex(result, "]"); idx >= 0 {
		result = result[:idx+1]
	}

	type charData struct {
		Name          string `json:"name"`
		Gender        string `json:"gender"`
		Age           int    `json:"age"`
		Appearance    string `json:"appearance"`
		Personality   string `json:"personality"`
		Background    string `json:"background"`
		CurrentStatus string `json:"current_status"`
	}
	var charList []charData
	if parseErr := json.Unmarshal([]byte(result), &charList); parseErr != nil {
		// Fallback: return raw text
		_ = json.NewEncoder(w).Encode(map[string]any{
			"analysis": result,
			"created":  0,
			"raw":      true,
		})
		return
	}

	// Upsert characters
	created := 0
	for _, cd := range charList {
		if strings.TrimSpace(cd.Name) == "" { continue }
		existing, _ := s.db.Character.Query().
			Where(ent_character.NovelID(novelIDStr), ent_character.Name(cd.Name)).
			First(ctx)
		if existing != nil {
			s.db.Character.UpdateOneID(existing.ID).
				SetGender(cd.Gender).
				SetAge(cd.Age).
				SetAppearance(cd.Appearance).
				SetPersonality(cd.Personality).
				SetBackground(cd.Background).
				SetCurrentStatus(cd.CurrentStatus).
				Exec(ctx)
		} else {
			s.db.Character.Create().
				SetNovelID(novelIDStr).
				SetName(cd.Name).
				SetGender(cd.Gender).
				SetAge(cd.Age).
				SetAppearance(cd.Appearance).
				SetPersonality(cd.Personality).
				SetBackground(cd.Background).
				SetCurrentStatus(cd.CurrentStatus).
				Save(ctx)
			created++
		}
	}

	// Build setting summary and update novel outline
	var summary strings.Builder
	summary.WriteString("【人物设定】\n")
	for _, cd := range charList {
		summary.WriteString(fmt.Sprintf("%s（%s", cd.Name, cd.Gender))
		if cd.Age > 0 { summary.WriteString(fmt.Sprintf("，%d岁", cd.Age)) }
		summary.WriteString("）")
		if cd.Personality != "" { summary.WriteString(fmt.Sprintf(" 性格：%s", cd.Personality)) }
		if cd.CurrentStatus != "" { summary.WriteString(fmt.Sprintf(" 状态：%s", cd.CurrentStatus)) }
		summary.WriteString("\n")
	}

	// Append character summary to novel outline
	row, _ := s.db.Novel.Query().Where(novel.ID(novelID)).Only(ctx)
	if row != nil {
		newOutline := strings.TrimSpace(row.Outline)
		if newOutline == "" {
			newOutline = summary.String()
		} else if !strings.Contains(newOutline, "【人物设定】") {
			newOutline += "\n\n" + summary.String()
		}
		s.db.Novel.UpdateOneID(novelID).SetOutline(newOutline).Exec(ctx)
	}

	_ = json.NewEncoder(w).Encode(map[string]any{
		"analysis": strings.Join(func() []string {
			lines := make([]string, len(charList))
			for i, c := range charList {
				lines[i] = fmt.Sprintf("%s：%s %d岁 — %s", c.Name, c.Gender, c.Age, c.Personality)
			}
			return lines
		}(), "\n"),
		"created":  created,
		"total":    len(charList),
		"raw":      false,
	})
}

func toCharItem(c *ent.Character) CharacterItem {
	return CharacterItem{
		ID:            fmt.Sprintf("%d", c.ID),
		NovelID:       c.NovelID,
		Name:          c.Name,
		Gender:        c.Gender,
		Age:           c.Age,
		Appearance:    c.Appearance,
		Personality:   c.Personality,
		Background:    c.Background,
		CurrentStatus: c.CurrentStatus,
		CreatedAt:     c.CreatedAt,
	}
}

func (s *Server) HandleGetSettings(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	_ = json.NewEncoder(w).Encode(config.GetSettings())
}

func (s *Server) HandleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	var settings config.AppSettings
	if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
		http.Error(w, fmt.Sprintf("invalid json: %v", err), http.StatusBadRequest)
		return
	}

	if err := config.UpdateSettings(settings); err != nil {
		http.Error(w, fmt.Sprintf("failed to save settings: %v", err), http.StatusInternalServerError)
		return
	}

	// Hot-reload review LLM from new settings
	s.syncReviewLLM()

	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) syncReviewLLM() {
	revSettings := config.GetSettings().Review
	if revSettings.APIKey == "" || revSettings.APIKey == "你的Key" {
		return
	}
	adapter, err := infra_llm.NewOpenAIAdapter(context.Background(), revSettings.APIKey, revSettings.BaseURL, revSettings.Model)
	if err != nil {
		fmt.Printf("[Settings] Failed to create review LLM: %v\n", err)
		return
	}
	s.reviewLLM = adapter
	fmt.Printf("[Settings] Review LLM updated: %s @ %s\n", revSettings.Model, revSettings.BaseURL)
}

type ImportChapterRequest struct {
	Title   string `json:"title,omitempty"`
	Content string `json:"content"`
	Order   int    `json:"order,omitempty"`
}

type ImportRequest struct {
	Chapters []ImportChapterRequest `json:"chapters"`
}

func (s *Server) HandleImportChapters(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if s.db == nil {
		http.Error(w, "database not configured", http.StatusInternalServerError)
		return
	}

	novelID, err := parseIntParam(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var req ImportRequest
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 10<<20))
	dec.DisallowUnknownFields()
	if decodeErr := dec.Decode(&req); decodeErr != nil {
		http.Error(w, fmt.Sprintf("invalid json: %v", decodeErr), http.StatusBadRequest)
		return
	}

	if len(req.Chapters) == 0 {
		http.Error(w, "chapters is required", http.StatusBadRequest)
		return
	}

	imported := make([]ChapterItem, 0, len(req.Chapters))

	for _, ch := range req.Chapters {
		content := strings.TrimSpace(ch.Content)
		if content == "" {
			continue
		}
		title := strings.TrimSpace(ch.Title)
		if title == "" {
			title = chapterTitle(ch.Order)
		}
		order := ch.Order
		if order <= 0 {
			last, queryErr := s.db.Chapter.
				Query().
				Where(chapter.HasNovelWith(novel.ID(novelID))).
				Order(ent.Desc(chapter.FieldOrder)).
				First(r.Context())
			if queryErr == nil && last != nil {
				order = last.Order + 1
			} else {
				order = 1
			}
		}

		row, createErr := s.db.Chapter.
			Create().
			SetNovelID(novelID).
			SetTitle(title).
			SetContent(content).
			SetWordCount(wordCountOf(content)).
			SetOrder(order).
			SetStatus("Draft").
			Save(r.Context())
		if createErr != nil {
			http.Error(w, fmt.Sprintf("failed to import chapter %q: %v", title, createErr), http.StatusInternalServerError)
			return
		}

		chapterID := fmt.Sprintf("%d", row.ID)
		item := ChapterItem{
			ID:        chapterID,
			NovelID:   fmt.Sprintf("%d", novelID),
			Title:     row.Title,
			Content:   row.Content,
			WordCount: row.WordCount,
			Order:     row.Order,
			Status:    row.Status,
			CreatedAt: row.CreatedAt,
			UpdatedAt: row.UpdatedAt,
		}
		imported = append(imported, item)

		// Publish event to trigger RAG ingestion
		if s.eventBus != nil {
			novelIDStr := fmt.Sprintf("%d", novelID)
			go func(cID string, cContent string) {
				ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
				defer cancel()
				if pubErr := s.eventBus.Publish(ctx, events.ChapterGeneratedEvent{
					NovelID:   novelIDStr,
					ChapterID: cID,
					Content:   cContent,
					Timestamp: time.Now(),
				}); pubErr != nil {
					fmt.Printf("[Import] Publish chapter.generated failed for %s: %v\n", cID, pubErr)
				}
			}(chapterID, content)
		}
	}

	_ = json.NewEncoder(w).Encode(map[string]any{
		"imported": imported,
		"count":    len(imported),
	})
}

func (s *Server) HandleExtractDocx(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	r.Body = http.MaxBytesReader(w, r.Body, 20<<20)
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, fmt.Sprintf("failed to parse form: %v", err), http.StatusBadRequest)
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to get file: %v", err), http.StatusBadRequest)
		return
	}
	defer file.Close()

	text, err := extractDocxText(file)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to extract docx: %v", err), http.StatusBadRequest)
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]any{
		"text": text,
	})
}

func extractDocxText(r io.Reader) (string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return "", fmt.Errorf("read failed: %w", err)
	}

	zipReader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("not a valid docx/zip file: %w", err)
	}

	var docXML []byte
	for _, f := range zipReader.File {
		if f.Name == "word/document.xml" {
			rc, err := f.Open()
			if err != nil {
				return "", fmt.Errorf("open document.xml failed: %w", err)
			}
			docXML, err = io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return "", fmt.Errorf("read document.xml failed: %w", err)
			}
			break
		}
	}

	if docXML == nil {
		return "", fmt.Errorf("word/document.xml not found in docx")
	}

	return parseDocxXML(docXML), nil
}

func parseDocxXML(data []byte) string {
	xmlStr := string(data)

	// Remove namespace prefixes from tags (w:xxx -> xxx)
	replacer := strings.NewReplacer(
		"<w:", "<",
		"</w:", "</",
		"<w ", "< ",
		"xmlns:w=\"http://schemas.openxmlformats.org/wordprocessingml/2006/main\"", "",
		"xmlns:r=\"http://schemas.openxmlformats.org/officeDocument/2006/relationships\"", "",
		"xmlns:mc=\"http://schemas.openxmlformats.org/markup-compatibility/2006\"", "",
		"xmlns:w14=\"http://schemas.microsoft.com/office/word/2010/wordml\"", "",
		"xmlns:wpg=\"http://schemas.microsoft.com/office/word/2010/wordprocessingGroup\"", "",
		"xmlns:wpi=\"http://schemas.microsoft.com/office/word/2010/wordprocessingInk\"", "",
		"xmlns:wne=\"http://schemas.microsoft.com/office/word/2006/wordml\"", "",
		"xmlns:wps=\"http://schemas.microsoft.com/office/word/2010/wordprocessingShape\"", "",
	)
	cleaned := replacer.Replace(xmlStr)

	type docxRun struct {
		Text string `xml:"t"`
	}
	type docxParagraph struct {
		Runs []docxRun `xml:"r"`
	}
	type docxBody struct {
		Paragraphs []docxParagraph `xml:"body>p"`
	}

	var body docxBody
	if err := xml.Unmarshal([]byte(cleaned), &body); err != nil {
		return ""
	}

	var buf strings.Builder
	for _, p := range body.Paragraphs {
		line := ""
		for _, r := range p.Runs {
			line += r.Text
		}
		line = strings.TrimSpace(line)
		if line != "" {
			buf.WriteString(line)
			buf.WriteString("\n")
		}
	}
	return strings.TrimSpace(buf.String())
}

func (s *Server) Start(addr string) error {
	fmt.Printf("🚀 API Server started at %s\n", addr)
	return http.ListenAndServe(addr, s.router)
}

func parseIntParam(v string) (int, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0, fmt.Errorf("empty id")
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid id: %q", v)
	}
	return n, nil
}

func wordCountOf(s string) int {
	return len([]rune(strings.TrimSpace(s)))
}

func chapterTitle(index int) string {
	if index <= 0 {
		return "未命名章节"
	}
	return fmt.Sprintf("第%d章", index)
}

func (s *Server) ensureChapterRecord(ctx context.Context, novelID int, chapterIndex int) (int, error) {
	if s.db == nil {
		return 0, fmt.Errorf("database not configured")
	}
	if novelID <= 0 {
		return 0, fmt.Errorf("invalid novel id")
	}
	if chapterIndex <= 0 {
		return 0, fmt.Errorf("invalid chapter index")
	}

	row, queryErr := s.db.Chapter.
		Query().
		Where(
			chapter.OrderEQ(chapterIndex),
			chapter.HasNovelWith(novel.ID(novelID)),
		).
		Only(ctx)
	if queryErr == nil && row != nil {
		if execErr := s.db.Chapter.
			UpdateOneID(row.ID).
			SetTitle(chapterTitle(chapterIndex)).
			SetContent("").
			SetWordCount(0).
			SetStatus("Generating").
			Exec(ctx); execErr != nil {
			return 0, execErr
		}
		return row.ID, nil
	}
	if queryErr != nil && !ent.IsNotFound(queryErr) {
		return 0, queryErr
	}

	created, err := s.db.Chapter.
		Create().
		SetNovelID(novelID).
		SetTitle(chapterTitle(chapterIndex)).
		SetContent("").
		SetWordCount(0).
		SetOrder(chapterIndex).
		SetStatus("Generating").
		Save(ctx)
	if err != nil {
		return 0, err
	}
	return created.ID, nil
}

func (s *Server) HandleGenerateChapter(w http.ResponseWriter, r *http.Request) {
	if s.engine == nil {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		http.Error(w, "engine not configured", http.StatusInternalServerError)
		return
	}

	// 1. 设置 SSE 响应头
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

	novelID := r.URL.Query().Get("novel_id")
	outline := strings.TrimSpace(r.URL.Query().Get("outline"))
	idea := strings.TrimSpace(r.URL.Query().Get("idea"))
	editorNotes := strings.TrimSpace(r.URL.Query().Get("editor_notes"))
	manualContext := strings.TrimSpace(r.URL.Query().Get("manual_context"))
	existingOutline := strings.TrimSpace(r.URL.Query().Get("existing_outline"))
	outlineStart, _ := strconv.Atoi(r.URL.Query().Get("outline_start"))
	outlineEnd, _ := strconv.Atoi(r.URL.Query().Get("outline_end"))
	chapterIDStr := r.URL.Query().Get("chapter_id")
	persistStr := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("persist")))
	chapterIndexStr := r.URL.Query().Get("chapter_index")
	chapterIndex := 1
	if chapterIndexStr != "" {
		fmt.Sscanf(chapterIndexStr, "%d", &chapterIndex)
	}

	if strings.TrimSpace(novelID) == "" {
		fmt.Fprintf(w, "event: error\ndata: %s\n\n", "Missing novel_id")
		flusher.Flush()
		return
	}

	novelIDInt, err := parseIntParam(novelID)
	if err != nil {
		fmt.Fprintf(w, "event: error\ndata: %s\n\n", err.Error())
		flusher.Flush()
		return
	}

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	if s.db != nil && (idea == "" || outline == "" || existingOutline == "") {
		loadCtx, loadCancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		row, qErr := s.db.Novel.Query().Where(novel.ID(novelIDInt)).Only(loadCtx)
		loadCancel()
		if qErr == nil && row != nil {
			if idea == "" {
				idea = strings.TrimSpace(row.Idea)
			}
			if outline == "" {
				outline = strings.TrimSpace(row.Outline)
			}
			if existingOutline == "" {
				existingOutline = strings.TrimSpace(row.Outline)
			}
		}
	}

	// Fallback: use existing chapters as context when no idea/outline
	if outline == "" && idea == "" && existingOutline == "" {
		if s.db != nil {
			loadCtx, loadCancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			chapters, chErr := s.db.Chapter.Query().
				Where(chapter.HasNovelWith(novel.ID(novelIDInt))).
				Order(ent.Asc(chapter.FieldOrder)).
				Limit(10).
				All(loadCtx)
			loadCancel()
			if chErr == nil && len(chapters) > 0 {
				var buf strings.Builder
				buf.WriteString("继续写作。以下是已写章节摘要：\n")
				for _, ch := range chapters {
					rContent := []rune(ch.Content)
					if len(rContent) > 500 {
						rContent = rContent[:500]
					}
					buf.WriteString(fmt.Sprintf("[%s] %s\n", ch.Title, string(rContent)))
				}
				idea = buf.String()
			}
		}
	}
	if outline == "" && idea == "" && existingOutline == "" {
		fmt.Fprintf(w, "event: error\ndata: %s\n\n", "Missing outline and idea (no saved outline found)")
		flusher.Flush()
		return
	}

	persist := true
	if persistStr == "0" || persistStr == "false" || persistStr == "no" {
		persist = false
	}

	chapterIDInt := 0
	if s.db != nil && persist {
		saveCtx, saveCancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		if strings.TrimSpace(chapterIDStr) != "" {
			chapterIDInt, err = parseIntParam(chapterIDStr)
			if err != nil {
				saveCancel()
				fmt.Fprintf(w, "event: error\ndata: %s\n\n", err.Error())
				flusher.Flush()
				return
			}
			_, queryErr := s.db.Chapter.
				Query().
				Where(
					chapter.ID(chapterIDInt),
					chapter.HasNovelWith(novel.ID(novelIDInt)),
				).
				Only(saveCtx)
			if queryErr != nil {
				saveCancel()
				if ent.IsNotFound(queryErr) {
					fmt.Fprintf(w, "event: error\ndata: %s\n\n", "chapter not found")
					flusher.Flush()
					return
				}
				fmt.Fprintf(w, "event: error\ndata: %v\n\n", queryErr)
				flusher.Flush()
				return
			}
			if execErr := s.db.Chapter.
				UpdateOneID(chapterIDInt).
				SetContent("").
				SetWordCount(0).
				SetStatus("Generating").
				Exec(saveCtx); execErr != nil {
				saveCancel()
				fmt.Fprintf(w, "event: error\ndata: %v\n\n", execErr)
				flusher.Flush()
				return
			}
		} else {
			chapterIDInt, err = s.ensureChapterRecord(saveCtx, novelIDInt, chapterIndex)
		}
		saveCancel()
		if err != nil {
			fmt.Fprintf(w, "event: error\ndata: %v\n\n", err)
			flusher.Flush()
			return
		}
	}

	// 2. 订阅 Token 生成事件
	tokenChan := make(chan string, 100)
	retryChan := make(chan events.ChapterRetryEvent, 8)
	subID := s.eventBus.Subscribe("token.generated", func(ctx context.Context, event events.Event) error {
		e, ok := event.(events.TokenGeneratedEvent)
		if ok && e.NovelID == novelID {
			// 非阻塞发送，防止 EventBus 协程阻塞
			select {
			case tokenChan <- e.Token:
			default:
			}
		}
		return nil
	})
	retrySubID := s.eventBus.Subscribe("chapter.retry", func(ctx context.Context, event events.Event) error {
		e, ok := event.(events.ChapterRetryEvent)
		if ok && e.NovelID == novelID {
			select {
			case retryChan <- e:
			default:
			}
		}
		return nil
	})
	// 确保在请求结束时取消订阅
	defer s.eventBus.Unsubscribe("token.generated", subID)
	defer s.eventBus.Unsubscribe("chapter.retry", retrySubID)

	// 3. 先推送 start，保证前端立即进入流式状态
	fmt.Fprintf(w, "event: start\ndata: %s\n\n", "Generation started")
	flusher.Flush()

	// 4. 预先生成场景卡与背景资料（只改不写），并推送元信息
	chapterID := ""
	if chapterIDInt > 0 {
		chapterID = fmt.Sprintf("%d", chapterIDInt)
	}
	state := &agents.GenerationState{
		NovelID:         novelID,
		ChapterID:       chapterID,
		ChapterIndex:    chapterIndex,
		Idea:            idea,
		FullOutline:     outline,
		EditorNotes:     editorNotes,
		ManualContext:   manualContext,
		ExistingOutline: existingOutline,
		OutlineStart:    outlineStart,
		OutlineEnd:      outlineEnd,
	}

	prepared, prepErr := s.engine.PrepareContext(ctx, state)
	if prepErr != nil {
		fmt.Fprintf(w, "event: error\ndata: %v\n\n", prepErr)
		flusher.Flush()
		return
	}

	meta := map[string]interface{}{
		"type":                 "context_meta",
		"novel_id":             prepared.NovelID,
		"chapter_index":        prepared.ChapterIndex,
		"chapter_id":           chapterID,
		"persist":              persist,
		"editor_notes":         prepared.EditorNotes,
		"manual_context":       prepared.ManualContext,
		"full_outline_preview": truncate(prepared.FullOutline, 400),
		"outline_preview":      truncate(prepared.Outline, 300),
		"scene_card_preview":   truncate(prepared.SceneCard, 500),
		"context_preview":      truncate(prepared.Context, 800),
		"context_stats": map[string]int{
			"context_lines":    1 + strings.Count(prepared.Context, "\n"),
			"scene_card_lines": 1 + strings.Count(prepared.SceneCard, "\n"),
		},
	}
	metaBytes, _ := json.Marshal(meta)
	fmt.Fprintf(w, "event: context_meta\ndata: %s\n\n", string(metaBytes))
	flusher.Flush()

	// 5. 异步启动生成任务（writer/reviewer）
	errChan := make(chan error, 1)
	go func() {
		finalState, genErr := s.engine.RunChapterGeneration(ctx, prepared)

		// Always persist what was generated, even on error
		if s.db != nil && persist && chapterIDInt > 0 {
			saveCtx, saveCancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
			draftText := ""
			status := "Draft"
			if finalState != nil && finalState.Draft != "" {
				draftText = finalState.Draft
				if genErr != nil {
					status = "Draft" // keep as draft even on error
				}
			}
			if draftText != "" {
				_, saveErr := s.db.Chapter.
					UpdateOneID(chapterIDInt).
					SetTitle(chapterTitle(prepared.ChapterIndex)).
					SetContent(draftText).
					SetWordCount(wordCountOf(draftText)).
					SetStatus(status).
					Save(saveCtx)
				if saveErr != nil {
					fmt.Printf("[Generate] Failed to save chapter %d: %v\n", chapterIDInt, saveErr)
				}
			} else {
				// No draft, mark as failed
				s.db.Chapter.UpdateOneID(chapterIDInt).SetStatus("Failed").Exec(saveCtx)
			}
			saveCancel()
		}

		if genErr != nil {
			errChan <- genErr
			close(tokenChan)
			return
		}
		close(tokenChan)
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case err := <-errChan:
			fmt.Fprintf(w, "event: error\ndata: %v\n\n", err)
			flusher.Flush()
			return
		case retryEvt := <-retryChan:
			payload, _ := json.Marshal(map[string]interface{}{
				"retry_count": retryEvt.RetryCount,
				"critique":    retryEvt.Critique,
			})
			fmt.Fprintf(w, "event: retry\ndata: %s\n\n", string(payload))
			flusher.Flush()
		case token, ok := <-tokenChan:
			if !ok {
				fmt.Fprintf(w, "event: end\ndata: %s\n\n", "Generation finished")
				flusher.Flush()
				return
			}
			// 发送 SSE 格式数据
			data, _ := json.Marshal(map[string]string{"token": token})
			fmt.Fprintf(w, "data: %s\n\n", string(data))
			flusher.Flush()
		}
	}
}

func truncate(s string, max int) string {
	s = strings.TrimSpace(s)
	if max <= 0 || s == "" {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "..."
}

// HandlePreviewContext 仅生成“场景卡 + 背景资料 + 共创指令”的合成上下文，不进入写作
func (s *Server) HandlePreviewContext(w http.ResponseWriter, r *http.Request) {
	if s.engine == nil {
		http.Error(w, "engine not configured", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	novelID := r.URL.Query().Get("novel_id")
	outline := strings.TrimSpace(r.URL.Query().Get("outline"))
	idea := strings.TrimSpace(r.URL.Query().Get("idea"))
	editorNotes := strings.TrimSpace(r.URL.Query().Get("editor_notes"))
	manualContext := strings.TrimSpace(r.URL.Query().Get("manual_context"))
	existingOutline := strings.TrimSpace(r.URL.Query().Get("existing_outline"))
	outlineStart, _ := strconv.Atoi(r.URL.Query().Get("outline_start"))
	outlineEnd, _ := strconv.Atoi(r.URL.Query().Get("outline_end"))

	chapterIndexStr := r.URL.Query().Get("chapter_index")
	chapterIndex := 1
	if chapterIndexStr != "" {
		fmt.Sscanf(chapterIndexStr, "%d", &chapterIndex)
	}

	if strings.TrimSpace(novelID) == "" {
		http.Error(w, "Missing novel_id and all of outline/idea/existing_outline", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	novelIDInt, err := parseIntParam(novelID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if s.db != nil && (idea == "" || outline == "" || existingOutline == "") {
		loadCtx, loadCancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		row, qErr := s.db.Novel.Query().Where(novel.ID(novelIDInt)).Only(loadCtx)
		loadCancel()
		if qErr == nil && row != nil {
			if idea == "" {
				idea = strings.TrimSpace(row.Idea)
			}
			if outline == "" {
				outline = strings.TrimSpace(row.Outline)
			}
			if existingOutline == "" {
				existingOutline = strings.TrimSpace(row.Outline)
			}
		}
	}

	if outline == "" && idea == "" && existingOutline == "" {
		http.Error(w, "Missing outline and idea (no saved outline found)", http.StatusBadRequest)
		return
	}

	state := &agents.GenerationState{
		NovelID:         novelID,
		ChapterIndex:    chapterIndex,
		FullOutline:     outline,
		Idea:            idea,
		EditorNotes:     editorNotes,
		ManualContext:   manualContext,
		ExistingOutline: existingOutline,
		OutlineStart:    outlineStart,
		OutlineEnd:      outlineEnd,
	}

	res, err := s.engine.PrepareContext(ctx, state)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	payload := map[string]interface{}{
		"novel_id":       res.NovelID,
		"chapter_index":  res.ChapterIndex,
		"full_outline":   res.FullOutline,
		"outline":        res.Outline,
		"scene_card":     res.SceneCard,
		"context":        res.Context,
		"editor_notes":   res.EditorNotes,
		"manual_context": res.ManualContext,
	}
	enc := json.NewEncoder(w)
	_ = enc.Encode(payload)
}


