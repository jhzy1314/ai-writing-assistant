package api

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/ai-novel/studio/internal/infrastructure/database"
	"github.com/go-chi/chi/v5"
	"golang.org/x/text/encoding/simplifiedchinese"
)

// ===== 人物卡 =====

func (s *Server) HandleListCharacters(w http.ResponseWriter, r *http.Request) {
	pid := r.URL.Query().Get("project_id")
	if pid == "" {
		writeError(w, http.StatusBadRequest, "缺少 project_id 查询参数")
		return
	}
	items, err := s.store.ListCharacters(r.Context(), pid)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, map[string]interface{}{"items": items})
}

func (s *Server) HandleCreateCharacter(w http.ResponseWriter, r *http.Request) {
	var c database.Character
	if err := decodeJSON(r, &c); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(c.ProjectID) == "" || strings.TrimSpace(c.Name) == "" {
		writeError(w, http.StatusBadRequest, "project_id 与 name 必填")
		return
	}
	item, err := s.store.CreateCharacter(r.Context(), c.ProjectID, c.Name, c.Description, c.AvatarURL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeCreated(w, map[string]interface{}{"item": item})
}

func (s *Server) HandleUpdateCharacter(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
		AvatarURL   *string `json:"avatar_url"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	item, err := s.store.UpdateCharacter(r.Context(), id, req.Name, req.Description, req.AvatarURL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if item == nil {
		writeError(w, http.StatusNotFound, "人物不存在")
		return
	}
	writeOK(w, map[string]interface{}{"item": item})
}

func (s *Server) HandleDeleteCharacter(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := s.store.DeleteCharacter(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ===== 世界观 =====

func (s *Server) HandleListWorldSettings(w http.ResponseWriter, r *http.Request) {
	pid := r.URL.Query().Get("project_id")
	if pid == "" {
		writeError(w, http.StatusBadRequest, "缺少 project_id 查询参数")
		return
	}
	items, err := s.store.ListWorldSettings(r.Context(), pid)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, map[string]interface{}{"items": items})
}

func (s *Server) HandleCreateWorldSetting(w http.ResponseWriter, r *http.Request) {
	var w_ database.WorldSetting
	if err := decodeJSON(r, &w_); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(w_.ProjectID) == "" || strings.TrimSpace(w_.Title) == "" {
		writeError(w, http.StatusBadRequest, "project_id 与 title 必填")
		return
	}
	item, err := s.store.CreateWorldSetting(r.Context(), w_.ProjectID, w_.Title, w_.Content)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeCreated(w, map[string]interface{}{"item": item})
}

func (s *Server) HandleUpdateWorldSetting(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		Title   *string `json:"title"`
		Content *string `json:"content"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	item, err := s.store.UpdateWorldSetting(r.Context(), id, req.Title, req.Content)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if item == nil {
		writeError(w, http.StatusNotFound, "世界观不存在")
		return
	}
	writeOK(w, map[string]interface{}{"item": item})
}

func (s *Server) HandleDeleteWorldSetting(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := s.store.DeleteWorldSetting(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ===== 素材 =====

func (s *Server) HandleListMaterials(w http.ResponseWriter, r *http.Request) {
	pid := r.URL.Query().Get("project_id")
	if pid == "" {
		writeError(w, http.StatusBadRequest, "缺少 project_id 查询参数")
		return
	}
	items, err := s.store.ListMaterials(r.Context(), pid)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, map[string]interface{}{"items": items})
}

// HandleUploadMaterial 上传素材文档并解析为纯文本入库
// 表单字段：project_id, name(可选), file
func (s *Server) HandleUploadMaterial(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(20 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "表单解析失败: "+err.Error())
		return
	}
	pid := strings.TrimSpace(r.FormValue("project_id"))
	if pid == "" {
		writeError(w, http.StatusBadRequest, "project_id 必填")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "未获取到上传文件")
		return
	}
	defer file.Close()

	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		name = header.Filename
	}
	ext := strings.ToLower(filepath.Ext(header.Filename))
	fileType := strings.TrimPrefix(ext, ".")

	text, err := extractText(header.Filename, file)
	if err != nil {
		writeError(w, http.StatusBadRequest, "文件解析失败: "+err.Error())
		return
	}
	// PDF 暂不支持解析，返回友好提示
	if fileType == "pdf" && strings.TrimSpace(text) == "" {
		text = "PDF格式暂不支持自动解析，请先将PDF转换为TXT文件后再导入。"
	}
	item, err := s.store.CreateMaterial(r.Context(), pid, name, text, fileType)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeCreated(w, map[string]interface{}{"item": item, "text": text})
}

func (s *Server) HandleDeleteMaterial(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := s.store.DeleteMaterial(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// extractText 根据扩展名解析文档为纯文本
func extractText(filename string, r io.Reader) (string, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".docx":
		return extractDocxText(r)
	case ".html", ".htm":
		data, err := io.ReadAll(r)
		if err != nil {
			return "", err
		}
		return stripHTML(string(data)), nil
	case ".epub":
		return extractEpubText(r)
	case ".doc", ".rtf":
		data, err := io.ReadAll(r)
		if err != nil {
			return "", err
		}
		return stripBinaryText(string(data)), nil
	case ".pdf":
		return "", nil // 返回空文本，调用方显示提示
	case ".txt", ".md", ".markdown", "":
		data, err := io.ReadAll(r)
		if err != nil {
			return "", err
		}
		// UTF-8 校验失败则按 GB18030 转码（GBK 的超集，兼容旧版 TXT 书源的 GBK/GB18030 编码）
		if !utf8.Valid(data) {
			dec := simplifiedchinese.GB18030.NewDecoder()
			if out, err := dec.Bytes(data); err == nil {
				return string(out), nil
			}
		}
		return string(data), nil
	default:
		data, err := io.ReadAll(r)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
}

// extractDocxText 解析 .docx（zip 内 word/document.xml）为纯文本
func extractDocxText(r io.Reader) (string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", err
	}
	var docXML []byte
	for _, f := range zr.File {
		if f.Name == "word/document.xml" {
			rc, err := f.Open()
			if err != nil {
				return "", err
			}
			docXML, err = io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return "", err
			}
			break
		}
	}
	if docXML == nil {
		return "", nil
	}
	return parseDocxXML(docXML), nil
}

func parseDocxXML(data []byte) string {
	cleaned := strings.NewReplacer(
		"<w:", "<", "</w:", "</", "<w ", "< ",
		`xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"`, "",
	).Replace(string(data))

	type run struct {
		Text string `xml:"t"`
	}
	type para struct {
		Runs []run `xml:"r"`
	}
	type body struct {
		Paras []para `xml:"body>p"`
	}
	var b body
	if err := xml.Unmarshal([]byte(cleaned), &b); err != nil {
		return ""
	}
	var buf strings.Builder
	for _, p := range b.Paras {
		line := ""
		for _, rn := range p.Runs {
			line += rn.Text
		}
		line = strings.TrimSpace(line)
		if line != "" {
			buf.WriteString(line)
			buf.WriteString("\n")
		}
	}
	return strings.TrimSpace(buf.String())
}

// stripHTML 清除 HTML 标签、script、style、HTML 实体，提取纯文本
func stripHTML(s string) string {
	// 移除 script / style 标签及其内容
	reScript := regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	s = reScript.ReplaceAllString(s, "")
	reStyle := regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	s = reStyle.ReplaceAllString(s, "")
	// 移除所有 HTML 标签
	reTag := regexp.MustCompile(`<[^>]+>`)
	s = reTag.ReplaceAllString(s, " ")
	// 解码常用 HTML 实体
	s = strings.ReplaceAll(s, "&nbsp;", " ")
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&quot;", "\"")
	s = strings.ReplaceAll(s, "&#39;", "'")
	s = strings.ReplaceAll(s, "&ldquo;", "\u201C")
	s = strings.ReplaceAll(s, "&rdquo;", "\u201D")
	// 压缩多余空白与换行
	reSpace := regexp.MustCompile(`[ \t]+`)
	s = reSpace.ReplaceAllString(s, " ")
	reNL := regexp.MustCompile(`\n{3,}`)
	s = reNL.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}

// stripBinaryText 从 .doc/.rtf 等二进制文本中过滤不可见控制字符，仅保留中英文、标点、换行
func stripBinaryText(s string) string {
	var buf strings.Builder
	for _, r := range s {
		if unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' {
			continue
		}
		if r > 0x7F || unicode.IsPrint(r) || r == '\n' || r == '\r' || r == '\t' {
			buf.WriteRune(r)
		}
	}
	return strings.TrimSpace(buf.String())
}

// extractEpubText 解压 epub（zip格式），遍历内部 xhtml/html 文件提取文本
func extractEpubText(r io.Reader) (string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", err
	}
	var buf strings.Builder
	for _, f := range zr.File {
		name := strings.ToLower(f.Name)
		if !strings.HasSuffix(name, ".xhtml") && !strings.HasSuffix(name, ".html") && !strings.HasSuffix(name, ".htm") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			continue
		}
		content, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			continue
		}
		text := stripHTML(string(content))
		if strings.TrimSpace(text) != "" {
			buf.WriteString(text)
			buf.WriteString("\n\n")
		}
	}
	return strings.TrimSpace(buf.String()), nil
}
