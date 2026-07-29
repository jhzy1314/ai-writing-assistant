package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/ai-novel/studio/internal/infrastructure/database"
	"github.com/go-chi/chi/v5"
)

// HandleGenerateCover 调用 Pollinations.ai 免费 API 生成封面图
func (s *Server) HandleGenerateCover(w http.ResponseWriter, r *http.Request) {
	pid := chi.URLParam(r, "id")
	if pid == "" {
		writeError(w, http.StatusBadRequest, "缺少 project_id")
		return
	}

	proj, err := s.store.GetProject(r.Context(), pid)
	if err != nil || proj == nil {
		writeError(w, http.StatusNotFound, "项目不存在")
		return
	}

	prompt := buildCoverPrompt(r.Context(), s.store, pid, proj.Name, proj.Type)

	urlStr := "https://image.pollinations.ai/prompt/" +
		url.PathEscape(sanitizePrompt(prompt)) +
		"?width=512&height=768&nologo=true"

	resp, err := http.Get(urlStr)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "封面生成失败：网络错误")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		writeError(w, http.StatusServiceUnavailable,
			fmt.Sprintf("封面生成失败（%d）", resp.StatusCode))
		return
	}

	coversDir := filepath.Join("data", "covers")
	os.MkdirAll(coversDir, 0755)

	safeName := safeFileName(proj.Name)
	coverPath := filepath.Join(coversDir, safeName+".png")
	f, err := os.Create(coverPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "封面保存失败")
		return
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		writeError(w, http.StatusInternalServerError, "封面写入失败")
		return
	}

	writeOK(w, map[string]string{
		"status": "ok",
		"url":    "/covers/" + safeName + ".png",
		"prompt": prompt,
	})
}

func buildCoverPrompt(ctx context.Context, store *database.Store, pid, projName, projType string) string {
	var parts []string

	parts = append(parts, "Chinese novel cover illustration")

	if projType != "" {
		parts = append(parts, "genre: "+projType)
	}

	base := "for novel titled " + projName

	chars, _ := store.ListCharacters(ctx, pid)
	n := 0
	for _, c := range chars {
		if n >= 2 {
			break
		}
		if c.Name != "" && c.Description != "" {
			desc := extractCharBrief(c.Description)
			if desc != "" {
				base += ", character: " + c.Name + ", " + desc
				n++
			}
		}
	}

	ws, _ := store.ListWorldSettings(ctx, pid)
	for _, w := range ws {
		if w.Title != "" {
			era := extractField(w.Content, "时代背景")
			if era != "" {
				base += ", era: " + era
			}
			tags := extractField(w.Content, "标签")
			if tags != "" {
				base += ", tags: " + tags
			}
		}
		break
	}

	base += ", atmospheric lighting, book cover design, vertical portrait, Chinese art style"
	parts = append(parts, base)
	return strings.Join(parts, ". ")
}

func extractCharBrief(desc string) string {
	gender := extractField(desc, "性别")
	personality := extractField(desc, "性格")
	appearance := extractField(desc, "外貌")
	out := ""
	if gender != "" {
		out = gender
	}
	if personality != "" {
		if out != "" {
			out += ", "
		}
		out += personality
	}
	if appearance != "" {
		r := []rune(appearance)
		if len(r) > 30 {
			appearance = string(r[:30]) + "..."
		}
		if out != "" {
			out += ", "
		}
		out += appearance
	}
	return out
}

func extractField(content, field string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, field+"：") || strings.HasPrefix(line, field+":") {
			v := strings.TrimPrefix(strings.TrimPrefix(line, field+"："), field+":")
			if v != "" {
				return strings.TrimSpace(v)
			}
		}
	}
	return ""
}

func sanitizePrompt(p string) string {
	p = strings.ReplaceAll(p, "#", "")
	p = strings.ReplaceAll(p, "&", "and")
	p = strings.ReplaceAll(p, "?", "")
	return p
}

func safeFileName(name string) string {
	r := []rune(name)
	var b strings.Builder
	for _, c := range r {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') ||
			c == '.' || c == '-' || c == '_' {
			b.WriteRune(c)
		} else if c >= 128 {
			b.WriteRune(c)
		} else {
			b.WriteRune('_')
		}
	}
	out := b.String()
	rr := []rune(out)
	if len(rr) > 100 {
		return string(rr[:100])
	}
	return out
}
