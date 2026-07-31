package api

import (
	"regexp"
	"time"
)

const reqTimeout = 3 * time.Minute // 单次非流式 LLM 请求超时

var reHTMLTag = regexp.MustCompile(`<[^>]*>`)

// sanitizeName 去除HTML标签，防止XSS存储
func sanitizeName(s string) string {
	return reHTMLTag.ReplaceAllString(s, "")
}
