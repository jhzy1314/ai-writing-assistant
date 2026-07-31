// Package search 提供无密钥的联网搜索能力（必应 RSS，国内可直连），供各 Agent 创作时检索背景资料。
package search

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"golang.org/x/text/encoding/simplifiedchinese"
)

// Result 单条搜索结果
type Result struct {
	Title   string
	URL     string
	Snippet string
}

var (
	reItem = regexp.MustCompile(`(?s)<item>(.*?)</item>`)
	reTag  = regexp.MustCompile(`<[^>]+>`)
)

// WebSearch 执行联网搜索，返回最多 limit 条结果（标题/链接/摘要）。
// 失败时返回错误，调用方应降级为“无联网信息”继续创作，不应中断流程。
func WebSearch(ctx context.Context, query string, limit int) ([]Result, error) {
	if limit <= 0 {
		limit = 5
	}
	if limit > 8 {
		limit = 8
	}
	if strings.TrimSpace(query) == "" {
		return nil, fmt.Errorf("搜索词为空")
	}

	endpoints := []string{
		"https://cn.bing.com/search?q=%s&format=rss&count=%d",
		"https://www.bing.com/search?q=%s&format=rss&count=%d",
	}
	var lastErr error
	for _, ep := range endpoints {
		u := fmt.Sprintf(ep, url.QueryEscape(query), limit)
		req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36")
		req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")

		client := &http.Client{Timeout: 8 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		// 必应国内版 RSS 常为 GBK/GB2312 编码，需转 UTF-8
		var reader io.Reader = resp.Body
		ct := resp.Header.Get("Content-Type")
		if !strings.Contains(strings.ToLower(ct), "utf-8") {
			reader = simplifiedchinese.GB18030.NewDecoder().Reader(resp.Body)
		}
		body, err := io.ReadAll(io.LimitReader(reader, 512*1024))
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}
		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("搜索服务返回状态 %d", resp.StatusCode)
			continue
		}
		results := parseRSS(string(body), limit)
		if len(results) > 0 {
			return results, nil
		}
		lastErr = fmt.Errorf("未检索到结果")
	}
	return nil, lastErr
}

// parseRSS 解析必应 RSS 结果
func parseRSS(xml string, limit int) []Result {
	items := reItem.FindAllStringSubmatch(xml, -1)
	out := make([]Result, 0, limit)
	for _, m := range items {
		if len(out) >= limit {
			break
		}
		block := m[1]
		title := extractTag(block, "title")
		link := extractTag(block, "link")
		desc := extractTag(block, "description")
		if title == "" {
			continue
		}
		out = append(out, Result{
			Title:   truncate(cleanText(title), 120),
			URL:     strings.TrimSpace(cleanText(link)),
			Snippet: truncate(cleanText(desc), 280),
		})
	}
	return out
}

func extractTag(block, tag string) string {
	re := regexp.MustCompile(`(?s)<` + tag + `[^>]*>(.*?)</` + tag + `>`)
	m := re.FindStringSubmatch(block)
	if len(m) > 1 {
		return m[1]
	}
	return ""
}

func cleanText(s string) string {
	s = reTag.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&quot;", `"`)
	s = strings.ReplaceAll(s, "&#x27;", "'")
	s = strings.ReplaceAll(s, "&#39;", "'")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&nbsp;", " ")
	return strings.TrimSpace(s)
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
