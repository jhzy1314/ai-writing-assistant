package api

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"time"
)

// sessionStore 内存中存储会话（单用户场景足够）
var sessionKeys = make(map[string]time.Time)

func init() {
	// 每 10 分钟清理过期会话
	go func() {
		for {
			time.Sleep(10 * time.Minute)
			now := time.Now()
			for k, exp := range sessionKeys {
				if now.After(exp) {
					delete(sessionKeys, k)
				}
			}
		}
	}()
}

// AuthConfig 认证配置
type AuthConfig struct {
	Password    string // SHA256 哈希
	SessionTTL  time.Duration
}

var authCfg *AuthConfig

// SetAuthConfig 设置认证配置（从 config.yaml 读取）
func SetAuthConfig(password string) {
	h := sha256.Sum256([]byte(password))
	authCfg = &AuthConfig{
		Password:   hex.EncodeToString(h[:]),
		SessionTTL: 24 * time.Hour,
	}
}

func genSessionKey() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// HandleAuthLogin 处理登录请求
func (s *Server) HandleAuthLogin(w http.ResponseWriter, r *http.Request) {
	if authCfg == nil || authCfg.Password == "" {
		writeError(w, http.StatusInternalServerError, "认证功能未启用")
		return
	}
	var req struct {
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	h := sha256.Sum256([]byte(req.Password))
	if hex.EncodeToString(h[:]) != authCfg.Password {
		writeError(w, http.StatusUnauthorized, "密码错误")
		return
	}
	key := genSessionKey()
	sessionKeys[key] = time.Now().Add(authCfg.SessionTTL)
	http.SetCookie(w, &http.Cookie{
		Name:     "ai_novel_sid",
		Value:    key,
		Path:     "/",
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(authCfg.SessionTTL.Seconds()),
	})
	writeOK(w, map[string]string{"status": "ok"})
}

// HandleAuthCheck 检查登录态
func (s *Server) HandleAuthCheck(w http.ResponseWriter, r *http.Request) {
	if authCfg == nil || authCfg.Password == "" {
		writeOK(w, map[string]interface{}{"authenticated": true, "required": false})
		return
	}
	authenticated := false
	if c, err := r.Cookie("ai_novel_sid"); err == nil {
		if exp, ok := sessionKeys[c.Value]; ok && time.Now().Before(exp) {
			authenticated = true
			sessionKeys[c.Value] = time.Now().Add(authCfg.SessionTTL)
		}
	}
	writeOK(w, map[string]interface{}{"authenticated": authenticated, "required": true})
}

// authRequired 认证中间件
func authRequired(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 未配置密码则跳过认证
		if authCfg == nil || authCfg.Password == "" {
			next.ServeHTTP(w, r)
			return
		}
		// 放行登录和检查接口
		path := r.URL.Path
		if path == "/api/auth/login" || path == "/api/auth/check" {
			next.ServeHTTP(w, r)
			return
		}
		// 静态资源放行
		if strings.HasPrefix(path, "/static/") || strings.HasPrefix(path, "/covers/") {
			next.ServeHTTP(w, r)
			return
		}
		// 非 API 路径放行（HTML 页面，由前端负责拦截）
		if !strings.HasPrefix(path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}
		c, err := r.Cookie("ai_novel_sid")
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]interface{}{"error": "未登录", "code": "auth_required"})
			return
		}
		exp, ok := sessionKeys[c.Value]
		if !ok || time.Now().After(exp) {
			writeJSON(w, http.StatusUnauthorized, map[string]interface{}{"error": "登录已过期", "code": "auth_required"})
			return
		}
		// 刷新过期时间
		sessionKeys[c.Value] = time.Now().Add(authCfg.SessionTTL)
		next.ServeHTTP(w, r)
	})
}
