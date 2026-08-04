package pipeline

import (
	"regexp"
	"strings"
)

// scrubChapterMeta 删除正文中的章节元信息残留行（AI 输出"（第X章正文）""本章完""待续"等）。
// 移植自对标项目 show-me-the-story 的 stripChapterMetaProse（writing.go:36-51）思路，
// 纯规则、零成本；仅删除独立的元信息行，不动正文内容。
// 在终稿汇总前调用（dispatcher.Run），保证交付给用户的是"印刷版"。
func scrubChapterMeta(content string) string {
	if strings.TrimSpace(content) == "" {
		return content
	}
	lines := strings.Split(content, "\n")
	kept := make([]string, 0, len(lines))
	for _, ln := range lines {
		if isChapterMetaLine(ln) {
			continue
		}
		kept = append(kept, ln)
	}
	out := strings.Join(kept, "\n")
	// 元信息行删除后可能留下连续空行，压缩为最多一个空行
	out = collapseBlankLines(out)
	return strings.TrimSpace(out) + "\n"
}

// isChapterMetaLine 判断一行是否为章节元信息/作者说明行（整行匹配，不误伤正文中的对话或叙述）。
var (
	chapterMetaLineRe = regexp.MustCompile(`(?i)^[\s\p{Zs}]*[（(【\[]?\s*(第[\d一二三四五六七八九十百千零两]+章|Chapter\s*\d+|第[\d一二三四五六七八九十百千零两]+节|(本章|本卷)?(完|终)|待续|未完待续|以下是(修订后|修改后)?的?(第[\d一二三四五六七八九十百千零两]+章|本章|全文)|以下为(修订后|修改后)?的?(第[\d一二三四五六七八九十百千零两]+章|本章|全文)|章节标题|作者[话按说]|分[隔割]线)[^。！？!?]*[）)】\]：:，,]?\s*$`)
	metaOnlyRe = regexp.MustCompile(`(?i)^[\s\p{Zs}]*[（(【\[]?\s*(第[\d一二三四五六七八九十百千零两]+章|Chapter\s*\d+|本章完|本卷完|待续|未完待续)[）)】\]。.]?\s*$`)
)

func isChapterMetaLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return false
	}
	// 纯"（第3章）"这类整行元信息
	if metaOnlyRe.MatchString(trimmed) {
		return true
	}
	// 含"以下是…正文"引导语的整行（长度限制，避免误杀正文长句）
	if chapterMetaLineRe.MatchString(trimmed) && len([]rune(trimmed)) <= 40 {
		return true
	}
	return false
}

// collapseBlankLines 把连续空行压缩为最多一个空行
func collapseBlankLines(s string) string {
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	blank := false
	for _, ln := range lines {
		if strings.TrimSpace(ln) == "" {
			if blank {
				continue
			}
			blank = true
		} else {
			blank = false
		}
		out = append(out, ln)
	}
	return strings.Join(out, "\n")
}
