package api

import (
	"archive/zip"
	"bytes"
	"fmt"
	"net/http"
	"strings"

	"github.com/ai-novel/studio/internal/infrastructure/database"
)

// HandleExportDOCX 服务端生成标准 DOCX 文件
func (s *Server) HandleExportDOCX(w http.ResponseWriter, r *http.Request) {
	pid := r.URL.Query().Get("project_id")
	if pid == "" {
		writeError(w, http.StatusBadRequest, "缺少 project_id 查询参数")
		return
	}
	chapters, err := s.store.ListChapters(r.Context(), pid, "")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "读取章节失败: "+err.Error())
		return
	}
	if len(chapters) == 0 {
		writeError(w, http.StatusBadRequest, "项目无章节可导出")
		return
	}
	proj, _ := s.store.GetProject(r.Context(), pid)
	filename := sanitizeFn(proj.Name)
	var buf bytes.Buffer
	if err := buildDocxFile(&buf, filename, chapters); err != nil {
		writeError(w, http.StatusInternalServerError, "生成 DOCX 失败: "+err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.wordprocessingml.document")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.docx"`, filename))
	w.Write(buf.Bytes())
}

func buildDocxFile(buf *bytes.Buffer, title string, chapters []database.ChapterWithVolume) error {
	z := zip.NewWriter(buf)
	w := func(name, content string) error {
		f, err := z.Create(name)
		if err != nil { return err }
		_, err = f.Write([]byte(content))
		return err
	}

	// mimetype (must be first & uncompressed)
	if err := w("mimetype", "application/vnd.openxmlformats-officedocument.wordprocessingml.document"); err != nil {
		return err
	}

	if err := w("[Content_Types].xml", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
  <Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/>
</Types>`); err != nil {
		return err
	}

	if err := w("_rels/.rels", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`); err != nil {
		return err
	}

	if err := w("word/_rels/document.xml.rels", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>
</Relationships>`); err != nil {
		return err
	}

	if err := w("word/styles.xml", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:style w:type="paragraph" w:styleId="Heading1">
    <w:name w:val="heading 1"/><w:pPr><w:spacing w:before="240" w:after="120"/></w:pPr>
    <w:rPr><w:rFonts w:ascii="SimHei" w:hAnsi="SimHei" w:eastAsia="SimHei"/><w:b/><w:sz w:val="32"/></w:rPr>
  </w:style>
  <w:style w:type="paragraph" w:styleId="Normal">
    <w:name w:val="Normal"/><w:pPr><w:spacing w:line="360" w:lineRule="auto" w:before="60" w:after="60"/></w:pPr>
    <w:rPr><w:rFonts w:ascii="SimSun" w:hAnsi="SimSun" w:eastAsia="SimSun"/><w:sz w:val="24"/></w:rPr>
  </w:style>
</w:styles>`); err != nil {
		return err
	}

	// word/document.xml
	var doc strings.Builder
	doc.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>`)
	for _, ch := range chapters {
		doc.WriteString(`<w:p><w:pPr><w:pStyle w:val="Heading1"/></w:pPr><w:r><w:t xml:space="preserve">`)
		doc.WriteString(xmlEsc(ch.Title))
		doc.WriteString(`</w:t></w:r></w:p>`)
		for _, line := range strings.Split(ch.Content, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				doc.WriteString(`<w:p><w:r><w:t xml:space="preserve"></w:t></w:r></w:p>`)
				continue
			}
			doc.WriteString(`<w:p><w:r><w:t xml:space="preserve">`)
			doc.WriteString(xmlEsc(line))
			doc.WriteString(`</w:t></w:r></w:p>`)
		}
	}
	doc.WriteString(`</w:body></w:document>`)
	if err := w("word/document.xml", doc.String()); err != nil {
		return err
	}
	return z.Close()
}

func xmlEsc(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}

func sanitizeFn(name string) string {
	if name == "" { return "output" }
	name = strings.Map(func(r rune) rune {
		if strings.ContainsRune(`<>:"/\|?*`, r) { return '_' }
		return r
	}, name)
	return name
}

// HandleExportTXT 导出全本 TXT
func (s *Server) HandleExportTXT(w http.ResponseWriter, r *http.Request) {
	pid := r.URL.Query().Get("project_id")
	if pid == "" {
		writeError(w, http.StatusBadRequest, "缺少 project_id 查询参数")
		return
	}
	vols, err := s.store.ListVolumes(r.Context(), pid)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "读取卷列表失败: "+err.Error())
		return
	}
	chapters, err := s.store.ListChapters(r.Context(), pid, "")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "读取章节失败: "+err.Error())
		return
	}
	if len(chapters) == 0 {
		writeError(w, http.StatusBadRequest, "项目无章节可导出")
		return
	}
	proj, _ := s.store.GetProject(r.Context(), pid)
	filename := sanitizeFn(proj.Name)
	var buf strings.Builder
	// 按卷分组
	volMap := map[string]string{}
	for _, v := range vols { volMap[v.ID] = v.Title }
	chaptersByVol := map[string][]database.ChapterWithVolume{}
	for _, ch := range chapters {
		vid := ch.VolumeID
		chaptersByVol[vid] = append(chaptersByVol[vid], ch)
	}
	for vid, volTitle := range volMap {
		if chs, ok := chaptersByVol[vid]; ok {
			buf.WriteString("\n══════ " + volTitle + " ══════\n\n")
			for _, ch := range chs {
				buf.WriteString("=== " + ch.Title + " ===\n\n")
				buf.WriteString(ch.Content + "\n\n")
			}
		}
	}
	if chs, ok := chaptersByVol[""]; ok {
		buf.WriteString("\n══════ 未分类 ══════\n\n")
		for _, ch := range chs {
			buf.WriteString("=== " + ch.Title + " ===\n\n")
			buf.WriteString(ch.Content + "\n\n")
		}
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.txt"`, filename))
	w.Write([]byte(buf.String()))
}

// HandleExportMD 导出全本 Markdown
func (s *Server) HandleExportMD(w http.ResponseWriter, r *http.Request) {
	pid := r.URL.Query().Get("project_id")
	if pid == "" {
		writeError(w, http.StatusBadRequest, "缺少 project_id 查询参数")
		return
	}
	vols, err := s.store.ListVolumes(r.Context(), pid)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "读取卷列表失败: "+err.Error())
		return
	}
	chapters, err := s.store.ListChapters(r.Context(), pid, "")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "读取章节失败: "+err.Error())
		return
	}
	if len(chapters) == 0 {
		writeError(w, http.StatusBadRequest, "项目无章节可导出")
		return
	}
	proj, _ := s.store.GetProject(r.Context(), pid)
	filename := sanitizeFn(proj.Name)
	var buf strings.Builder
	buf.WriteString("# " + proj.Name + "\n\n")
	volMap := map[string]string{}
	for _, v := range vols { volMap[v.ID] = v.Title }
	chaptersByVol := map[string][]database.ChapterWithVolume{}
	for _, ch := range chapters {
		chaptersByVol[ch.VolumeID] = append(chaptersByVol[ch.VolumeID], ch)
	}
	for vid, volTitle := range volMap {
		if chs, ok := chaptersByVol[vid]; ok {
			buf.WriteString("## " + volTitle + "\n\n")
			for _, ch := range chs {
				buf.WriteString("### " + ch.Title + "\n\n")
				buf.WriteString(ch.Content + "\n\n")
			}
		}
	}
	if chs, ok := chaptersByVol[""]; ok {
		buf.WriteString("## 未分类\n\n")
		for _, ch := range chs {
			buf.WriteString("### " + ch.Title + "\n\n")
			buf.WriteString(ch.Content + "\n\n")
		}
	}
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.md"`, filename))
	w.Write([]byte(buf.String()))
}
