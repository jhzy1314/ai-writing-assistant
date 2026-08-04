package api

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// LibraryFile 文库中的一本书
type LibraryFile struct {
	Name string `json:"name"` // 相对文库根目录的路径（子目录用 / 分隔）
	Size int64  `json:"size"`
	Ext  string `json:"ext"`
}

var supportedLibExts = map[string]bool{
	".txt": true, ".md": true, ".markdown": true,
	".epub": true, ".docx": true, ".html": true, ".htm": true,
}

// HandleListLibrary GET /api/library 扫描本地文库目录，列出支持的书籍文件（递归）
func (s *Server) HandleListLibrary(w http.ResponseWriter, r *http.Request) {
	if s.libraryDir == "" {
		writeOK(w, map[string]interface{}{"items": []LibraryFile{}, "dir": "", "message": "未配置 library_dir"})
		return
	}
	root := filepath.Clean(s.libraryDir)
	if _, err := os.Stat(root); err != nil {
		writeOK(w, map[string]interface{}{"items": []LibraryFile{}, "dir": s.libraryDir, "message": "文库目录不存在或不可访问"})
		return
	}
	var items []LibraryFile
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // 跳过不可读的条目
		}
		if d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(d.Name()))
		if !supportedLibExts[ext] {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		items = append(items, LibraryFile{
			Name: filepath.ToSlash(rel),
			Size: info.Size(),
			Ext:  strings.TrimPrefix(ext, "."),
		})
		return nil
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "扫描文库失败: "+err.Error())
		return
	}
	writeOK(w, map[string]interface{}{"items": items, "dir": s.libraryDir})
}

// HandleAnalyzeLibrary POST /api/library/analyze
// {"file": "诡秘之主(1-500章).txt"} → 异步拆书任务：全文分块通读（不截断）→ 返回 task_id，轮询进度
func (s *Server) HandleAnalyzeLibrary(w http.ResponseWriter, r *http.Request) {
	var req struct {
		File  string `json:"file"`
		Model string `json:"model"` // 可选：指定拆书模型（如免费模型 glm-4-flash），空则用 helper 角色绑定
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if s.libraryDir == "" {
		writeError(w, http.StatusBadRequest, "未配置 library_dir")
		return
	}
	// 路径安全：只允许文库目录内的文件（防目录穿越）
	cleanName := filepath.Clean(filepath.FromSlash(req.File))
	if cleanName == "." || strings.HasPrefix(cleanName, "..") || filepath.IsAbs(cleanName) {
		writeError(w, http.StatusBadRequest, "非法的文件路径")
		return
	}
	// 只允许受支持的书籍格式（防把任意文件喂给模型）
	ext := strings.ToLower(filepath.Ext(cleanName))
	if !supportedLibExts[ext] {
		writeError(w, http.StatusBadRequest, "不支持的格式（"+ext+"），支持 txt/md/epub/docx/html")
		return
	}
	full := filepath.Join(filepath.Clean(s.libraryDir), cleanName)
	if !strings.HasPrefix(filepath.Clean(full), filepath.Clean(s.libraryDir)+string(os.PathSeparator)) &&
		filepath.Clean(full) != filepath.Clean(s.libraryDir) {
		writeError(w, http.StatusBadRequest, "文件不在文库目录内")
		return
	}

	taskID := analyzeTasks.start(func(t *analyzeTask) {
		t.set("读取文件…", 3)
		ff, err := os.Open(full)
		if err != nil {
			t.finish(false, "", "", "文库中找不到该文件: "+err.Error(), 0, 0)
			return
		}
		defer ff.Close()
		text, err := extractText(filepath.Base(full), ff)
		if err != nil {
			t.finish(false, "", "", "解析文件失败: "+err.Error(), 0, 0)
			return
		}
		if strings.TrimSpace(text) == "" {
			t.finish(false, "", "", "未能从文件中提取到文本（azw3 暂不支持，请转成 txt/epub）", 0, 0)
			return
		}
		// 全文通读，不截断（固定 8 万字/块，不按 context_limit 配置限制）
		bookName := strings.TrimSuffix(filepath.Base(full), filepath.Ext(full))
		s.runBookAnalyze(t, text, req.Model, bookName)
	})
	writeOK(w, map[string]interface{}{"task_id": taskID})
}

// HandleAnalyzeProject POST /api/library/analyze-project
// {"project_id": "...", "model": "..."} → 拆解项目本身：把项目全部章节正文作为拆书源，四类素材入库（source_file=项目名）
func (s *Server) HandleAnalyzeProject(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProjectID string `json:"project_id"`
		Model     string `json:"model"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(req.ProjectID) == "" {
		writeError(w, http.StatusBadRequest, "project_id 不能为空")
		return
	}
	taskID := analyzeTasks.start(func(t *analyzeTask) {
		t.set("读取项目正文…", 3)
		ctx := context.Background()
		bookName := "未命名项目"
		if proj, err := s.store.GetProject(ctx, req.ProjectID); err == nil && proj != nil && strings.TrimSpace(proj.Name) != "" {
			bookName = strings.TrimSpace(proj.Name)
		}
		chs, err := s.store.ListChapters(ctx, req.ProjectID, "")
		if err != nil {
			t.finish(false, "", "", "读取章节失败: "+err.Error(), 0, 0)
			return
		}
		var b strings.Builder
		for _, ch := range chs {
			if strings.TrimSpace(ch.Content) == "" {
				continue
			}
			b.WriteString(ch.Title + "\n")
			b.WriteString(ch.Content + "\n\n")
		}
		text := b.String()
		if len([]rune(text)) < 500 {
			t.finish(false, "", "", "项目正文太少（<500字），无法拆解", 0, 0)
			return
		}
		// 复用全文通读拆解：分块特征提取 → 汇总 → 四类素材自动入库（按 source_file 幂等重建）
		s.runBookAnalyze(t, text, req.Model, bookName)
	})
	writeOK(w, map[string]interface{}{"task_id": taskID})
}
