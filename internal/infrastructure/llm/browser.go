package llm

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

var providerBaseURLs = map[string]string{
	"kimi-free":     "https://kimi.moonshot.cn/",
	"doubao-free":   "https://www.doubao.com/chat/",
	"qwen-free":     "https://tongyi.aliyun.com/",
	"deepseek-free": "https://chat.deepseek.com/",
	"zhipu-free":    "https://chatglm.cn/",
}

var (
	cookieSessions sync.Map
	sessionSeq     int
)

type CookieSession struct {
	ID        string
	Status    string
	Cookies   string
	Error     string
	StartedAt time.Time
	Provider  string
	// 检测进度（供前端展示，非登录态也可用）
	DetectedCookies int
	DetectedLen     int
	cancel          context.CancelFunc
}

func StartCookieCapture(provider string) (*CookieSession, error) {
	sessionSeq++
	id := fmt.Sprintf("cs_%d_%d", time.Now().UnixNano(), sessionSeq)
	ctx, cancel := context.WithCancel(context.Background())

	s := &CookieSession{
		ID:        id,
		Status:    "pending",
		StartedAt: time.Now(),
		Provider:  provider,
		cancel:    cancel,
	}

	targetURL := providerBaseURLs[provider]
	if targetURL == "" {
		targetURL = "https://kimi.moonshot.cn/"
	}

	go func() {
		defer cancel()

		launchURL := launcher.New().Headless(false).MustLaunch()
		browser := rod.New().ControlURL(launchURL).MustConnect()
		defer browser.Close()

		page := browser.MustPage()
		page.MustNavigate(targetURL).MustWaitLoad()

		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		timeout := time.After(10 * time.Minute)

		for {
			select {
			case <-ctx.Done():
				return
			case <-timeout:
				s.setFailed("获取Cookie超时（10分钟）")
				return
			case <-ticker.C:
				cookies, err := getCookiesString(page)
				if err != nil || cookies == "" {
					continue
				}
				// 无论是否登录都记录检测进度，前端可展示“已检测到 N 个 Cookie”
				s.DetectedCookies = len(strings.Split(cookies, ";"))
				s.DetectedLen = len(cookies)
				if hasSessionCookie(cookies) {
					s.Cookies = cookies
					s.Status = "completed"
					cookieSessions.Store(s.ID, s)
					return
				}
			}
		}
	}()

	cookieSessions.Store(s.ID, s)
	return s, nil
}

func GetCookieSession(id string) *CookieSession {
	v, ok := cookieSessions.Load(id)
	if !ok {
		return nil
	}
	return v.(*CookieSession)
}

func CancelCookieSession(id string) {
	v, ok := cookieSessions.Load(id)
	if !ok {
		return
	}
	s := v.(*CookieSession)
	s.cancel()
	s.Status = "cancelled"
	cookieSessions.Delete(id)
}

func (s *CookieSession) setFailed(msg string) {
	s.Status = "failed"
	s.Error = msg
	cookieSessions.Store(s.ID, s)
}

// junkCookie 判断是否为垃圾/风控/统计类 Cookie（未登录也会大量出现，不能作为登录依据）
func junkCookie(name string) bool {
	n := strings.ToLower(name)
	if strings.HasPrefix(n, "_") {
		return true
	}
	switch n {
	case "ab", "cna", "unb", "unp", "umt", "isg", "tfstk", "x5sec", "acw_tc", "aliyungf_tc", "sensorsdata", "hwwafsesid", "hwwafsesid_hw":
		return true
	}
	for _, j := range []string{"csrf", "waf", "sensors", "hawk", "aliyungf", "x5sec", "acw_"} {
		if strings.Contains(n, j) {
			return true
		}
	}
	return false
}

func getCookiesString(page *rod.Page) (string, error) {
	cookies, err := page.Cookies(nil)
	if err != nil {
		return "", err
	}
	if len(cookies) == 0 {
		return "", nil
	}
	var parts []string
	for _, c := range cookies {
		if junkCookie(c.Name) {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s=%s", c.Name, c.Value))
	}
	return strings.Join(parts, "; "), nil
}

// hasSessionCookie 判断是否已登录：必须有“登录态”特征的 Cookie 才算数。
// 判定规则（满足其一）：
//  1. 存在名称含 session/token/sid/auth/login/uid/member/user 的 Cookie 且值较长（>=8）
//  2. 至少 2 个 Cookie 的值都很长（>=40，登录态通常才有）
//  3. 整个 Cookie 串 >=200 字符（单条肥 Cookie 的站点也能命中）
func hasSessionCookie(cookieStr string) bool {
	if cookieStr == "" {
		return false
	}
	parts := strings.Split(cookieStr, ";")
	if len(parts) < 2 && len(cookieStr) < 200 {
		return false
	}
	authRe := regexp.MustCompile(`(?i)(session|token|sid|auth|login|uid|member|user)`)
	longCount := 0
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		eq := strings.Index(p, "=")
		if eq <= 0 {
			continue
		}
		val := p[eq+1:]
		if len(val) >= 8 && authRe.MatchString(p[:eq]) {
			return true
		}
		if len(val) >= 40 {
			longCount++
		}
	}
	return longCount >= 2 || len(cookieStr) >= 200
}
