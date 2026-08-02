package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"math/rand"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	webstatic "github.com/ai-novel/studio/web"
	"github.com/ai-novel/studio/internal/appearance"
	"github.com/ai-novel/studio/internal/domain/pipeline"
	"github.com/ai-novel/studio/internal/infrastructure/database"
	"github.com/ai-novel/studio/internal/infrastructure/llm"
	"github.com/ai-novel/studio/internal/infrastructure/quota"
	"github.com/ai-novel/studio/internal/infrastructure/rag"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// buildVersion 每次启动时随机生成，用于静态资源缓存破坏
var buildVersion = fmt.Sprintf("%d", rand.Int31())
var einoProxyURL string

func SetEinoProxy(addr string) {
	einoProxyURL = addr
}

// Server 接口层：RESTful API + SSE 流式输出，并托管前端静态资源
type Server struct {
	store      *database.Store
	registry   *llm.Registry
	dispatcher *pipeline.Dispatcher
	limiter    *quota.Limiter
	router     *chi.Mux
	appearance *appearance.Service
	rag        *rag.Service
}

// NewServer 构造 API 服务
func NewServer(store *database.Store, registry *llm.Registry, dispatcher *pipeline.Dispatcher, limiter *quota.Limiter, appSvc *appearance.Service) *Server {
	s := &Server{
		store:      store,
		registry:   registry,
		dispatcher: dispatcher,
		limiter:    limiter,
		router:     chi.NewRouter(),
		appearance: appSvc,
		rag:        rag.NewService(store),
	}
	s.routes()
	return s
}

// routes 注册全部路由（对应规格第四章 API 接口定义）
func (s *Server) routes() {
	s.router.Use(middleware.Logger)
	s.router.Use(middleware.Recoverer)

	// Eino 增强代理（auth cookie 透传，由 Eino 内部自行校验）
	if einoProxyURL != "" {
		if u, err := url.Parse(einoProxyURL); err == nil {
			proxy := &httputil.ReverseProxy{
				Rewrite: func(r *httputil.ProxyRequest) {
					r.SetURL(u)
					r.Out.Host = r.In.Host
					// 透传认证 cookie
					if cookie, err := r.In.Cookie("auth_token"); err == nil {
						r.Out.AddCookie(cookie)
					}
				},
				FlushInterval: -1, // SSE 即时冲刷
			}
			s.router.Get("/api/v1/*", func(w http.ResponseWriter, r *http.Request) { proxy.ServeHTTP(w, r) })
			s.router.Post("/api/v1/*", func(w http.ResponseWriter, r *http.Request) { proxy.ServeHTTP(w, r) })
			s.router.Put("/api/v1/*", func(w http.ResponseWriter, r *http.Request) { proxy.ServeHTTP(w, r) })
			s.router.Delete("/api/v1/*", func(w http.ResponseWriter, r *http.Request) { proxy.ServeHTTP(w, r) })
			s.router.Options("/api/v1/*", s.handleOptions)
		}
	}

	s.router.Use(s.securityHeadersMiddleware)
	s.router.Use(authRequired)
	s.router.Use(s.corsMiddleware)

	// 预检
	s.router.Options("/*", s.handleOptions)

	// 0. 认证
	s.router.Post("/api/auth/login", s.HandleAuthLogin)
	s.router.Get("/api/auth/check", s.HandleAuthCheck)

	// 1. 创作请求（SSE 流式）
	s.router.Post("/api/generate", s.HandleGenerate)
	// 1.5 需求-字数预检（Helper 轻量分析）
	s.router.Post("/api/precheck", s.HandlePrecheck)
	// 7. 逻辑自检独立接口
	s.router.Post("/api/verify", s.HandleVerify)
	s.router.Post("/api/rag/preview", s.HandleRAGPreview)
	s.router.Post("/api/ai-tells", s.HandleAITells)
	s.router.Post("/api/ai-polish", s.HandleAIPolish)

	// 8. Helper 工具辅助接口
	s.router.Post("/api/tools/execute", s.HandleToolExecute)

	// 2. 项目管理
	s.router.Get("/api/projects", s.HandleListProjects)
	s.router.Post("/api/projects", s.HandleCreateProject)
	s.router.Get("/api/projects/{id}", s.HandleGetProject)
	s.router.Get("/api/projects/{id}/search", s.HandleSearchChapters)
	s.router.Put("/api/projects/{id}", s.HandleUpdateProject)
	s.router.Delete("/api/projects/{id}", s.HandleDeleteProject)
	s.router.Post("/api/projects/{id}/duplicate", s.HandleDuplicateProject)
	s.router.Post("/api/projects/{id}/cover", s.HandleGenerateCover)

	// 3. 稿件版本
	s.router.Get("/api/projects/{id}/versions", s.HandleListVersions)
	s.router.Post("/api/versions", s.HandleSaveVersion)
	s.router.Get("/api/versions/{id}", s.HandleGetVersion)

	// 4. 设定资源
	s.router.Get("/api/characters", s.HandleListCharacters)
	s.router.Post("/api/characters", s.HandleCreateCharacter)
	s.router.Put("/api/characters/{id}", s.HandleUpdateCharacter)
	s.router.Delete("/api/characters/{id}", s.HandleDeleteCharacter)

	s.router.Get("/api/worldsettings", s.HandleListWorldSettings)
	s.router.Post("/api/worldsettings", s.HandleCreateWorldSetting)
	s.router.Put("/api/worldsettings/{id}", s.HandleUpdateWorldSetting)
	s.router.Delete("/api/worldsettings/{id}", s.HandleDeleteWorldSetting)

	s.router.Post("/api/materials/upload", s.HandleUploadMaterial)
	s.router.Get("/api/materials", s.HandleListMaterials)
	s.router.Delete("/api/materials/{id}", s.HandleDeleteMaterial)

	// 5. 模板
	s.router.Get("/api/templates", s.HandleListTemplates)
	s.router.Post("/api/templates", s.HandleCreateTemplate)
	s.router.Put("/api/templates/{id}", s.HandleUpdateTemplate)
	s.router.Delete("/api/templates/{id}", s.HandleDeleteTemplate)

	// 6. 模型配置（后台管理）
	s.router.Get("/api/models", s.HandleListModels)
	s.router.Post("/api/models", s.HandleCreateModel)
	s.router.Put("/api/models/{id}", s.HandleUpdateModel)
	s.router.Delete("/api/models/{id}", s.HandleDeleteModel)
	s.router.Post("/api/models/{id}/test", s.HandleTestModelConnection)
	s.router.Put("/api/models/{id}/default", s.HandleSetDefaultModel)
	s.router.Get("/api/roles/{role}/models", s.HandleGetRoleModels)
	s.router.Put("/api/roles/{role}/models", s.HandleSetRoleModels)
	// 6.5 网页AI模型
	s.router.Get("/api/webai/providers", s.HandleListWebAIProviders)
	s.router.Post("/api/webai/models", s.HandleCreateWebAIModel)
	s.router.Put("/api/webai/models/{id}", s.HandleUpdateWebAIModel)
	s.router.Post("/api/webai/test", s.HandleTestWebAIConnection)
	s.router.Post("/api/webai/auto-cookie", s.HandleAutoCookieStart)
	s.router.Get("/api/webai/auto-cookie/{session_id}", s.HandleAutoCookiePoll)
	s.router.Delete("/api/webai/auto-cookie/{session_id}", s.HandleAutoCookieCancel)

	// 7. 章节层级管理
	s.router.Get("/api/projects/{id}/volumes", s.HandleListVolumes)
	s.router.Post("/api/volumes", s.HandleCreateVolume)
	s.router.Post("/api/volumes/reorder", s.HandleReorderVolumes)
	s.router.Put("/api/volumes/{id}", s.HandleUpdateVolume)
	s.router.Delete("/api/volumes/{id}", s.HandleDeleteVolume)

	s.router.Get("/api/chapters", s.HandleListChapters)
	s.router.Post("/api/chapters", s.HandleCreateChapter)
	// 字面量路由必须在参数化路由 /api/chapters/{id} 之前，否则 chi 会将 "export"/"import" 等当作 {id} 匹配
	s.router.Get("/api/chapters/export", s.HandleExportChapters)
	s.router.Post("/api/chapters/import", s.HandleImportChapters)
	s.router.Post("/api/chapters/split", s.HandleSplitChapters)
	s.router.Post("/api/chapters/merge", s.HandleMergeChapters)
	s.router.Get("/api/export/docx", s.HandleExportDOCX)
	s.router.Get("/api/export/txt", s.HandleExportTXT)
	s.router.Get("/api/export/md", s.HandleExportMD)
	s.router.Post("/api/chapters/reorder", s.HandleReorderChapters)
	s.router.Get("/api/chapters/versions/{id}", s.HandleGetChapterVersion)
	s.router.Get("/api/chapters/{id}", s.HandleGetChapter)
	s.router.Put("/api/chapters/{id}", s.HandleUpdateChapter)
	s.router.Delete("/api/chapters/{id}", s.HandleDeleteChapter)
	s.router.Post("/api/chapters/{id}/copy", s.HandleCopyChapter)
	s.router.Get("/api/chapters/trash", s.HandleListTrashChapters)
	s.router.Post("/api/chapters/{id}/restore", s.HandleRestoreChapter)
	s.router.Post("/api/chapters/{id}/permanent-delete", s.HandlePermanentDeleteChapter)
	s.router.Get("/api/chapters/{id}/versions", s.HandleListChapterVersions)
	s.router.Post("/api/chapters/{id}/versions", s.HandleSaveChapterVersion)
	s.router.Post("/api/chapters/{id}/split", s.HandleSplitChapter)
	s.router.Get("/api/projects/{id}/stats", s.HandleProjectStats)

	// 运维统计（规格第七章 5/7：调用记录与用量）
	s.router.Get("/api/logs", s.HandleListLogs)
	s.router.Get("/api/usage", s.HandleUsage)
	s.router.Get("/api/configs", s.HandleListConfigs)
	s.router.Put("/api/configs/{key}", s.HandleSetConfig)

	// 9. 背景外观 API
	s.router.Get("/api/appearance/status", s.HandleAppearanceStatus)
	s.router.Get("/api/appearance/resources", s.HandleAppearanceDownload)
	s.router.Get("/api/appearance/backgrounds", s.HandleGetBackgrounds)
	s.router.Post("/api/appearance/backgrounds/set", s.HandleSetBackground)
	s.router.Post("/api/appearance/backgrounds/upload", s.HandleUploadBackground)
	s.router.Post("/api/appearance/backgrounds/generate", s.HandleGenerateBackground)
	s.router.Post("/api/appearance/backgrounds/random", s.HandleRandomBackground)
	s.router.Post("/api/appearance/backgrounds/reset", s.HandleResetDefault)

	// 静态前端（embed）——注入版本号破坏缓存
	indexHTML, _ := fs.ReadFile(webstatic.FS, "index.html")
	if indexHTML == nil {
		indexHTML = []byte("<html><body>index.html not embedded</body></html>")
	}
	// 给 JS/CSS 链接加版本号，强制浏览器加载最新
	cacheBusted := strings.NewReplacer(
		".js\"", ".js?v="+buildVersion+"\"",
		".css\"", ".css?v="+buildVersion+"\"",
	).Replace(string(indexHTML))

	s.router.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(cacheBusted))
	})
	// 静态资源（css/js 等，从 embed 根目录读取）
	s.router.Get("/static/*", func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, "/static/")
		if p == "" {
			http.NotFound(w, r)
			return
		}
		data, err := webstatic.FS.ReadFile(p)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		switch {
		case strings.HasSuffix(p, ".css"):
			w.Header().Set("Content-Type", "text/css; charset=utf-8")
		case strings.HasSuffix(p, ".js"):
			w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		case strings.HasSuffix(p, ".html"):
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
		default:
			w.Header().Set("Content-Type", "application/octet-stream")
		}
		_, _ = w.Write(data)
	})
	// 封面图片（从文件系统读取，不在 embed 中）
	s.router.Get("/covers/*", func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/covers/")
		if name == "" || strings.Contains(name, "..") {
			http.NotFound(w, r)
			return
		}
		data, err := os.ReadFile(filepath.Join("data", "covers", name))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "public, max-age=86400")
		w.Header().Set("Content-Type", "image/png")
		w.Write(data)
	})

	// 背景图片（从文件系统读取，支持 SVG/PNG/JPG/WEBP）
	s.router.Get("/backgrounds/*", func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/backgrounds/")
		if name == "" || strings.Contains(name, "..") {
			http.NotFound(w, r)
			return
		}
		ext := strings.ToLower(filepath.Ext(name))
		contentType := "application/octet-stream"
		switch ext {
		case ".svg":
			contentType = "image/svg+xml"
		case ".png":
			contentType = "image/png"
		case ".jpg", ".jpeg":
			contentType = "image/jpeg"
		case ".webp":
			contentType = "image/webp"
		}
		// 从多个可能的位置查找背景文件
		var data []byte
		var err error
		searchPaths := []string{
			filepath.Join("static", "backgrounds", name),       // 相对路径 (运行目录)
			filepath.Join("web", "static", "backgrounds", name), // web 同步目录
		}
		for _, sp := range searchPaths {
			data, err = os.ReadFile(sp)
			if err == nil {
				break
			}
		}
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "public, max-age=86400")
		w.Header().Set("Content-Type", contentType)
		w.Write(data)
	})

	s.router.Handle("/*", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 单页应用回退到 index.html（排除 /api 路径）
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(indexHTML)
	}))
}

// Start 启动 HTTP 服务
func (s *Server) Start(addr string) error {
	return http.ListenAndServe(addr, s.router)
}

// Router 暴露路由给外部（用于测试或自定义中间件）
func (s *Server) Router() http.Handler { return s.router }

// SetAppearance 设置背景外观服务（必须在 routes 注册前由 main.go 调用）
func (s *Server) SetAppearance(app *appearance.Service) {
	s.appearance = app
}

// ===== 通用响应助手 =====

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeOK(w http.ResponseWriter, v interface{}) { writeJSON(w, http.StatusOK, v) }
func writeCreated(w http.ResponseWriter, v interface{}) {
	writeJSON(w, http.StatusCreated, v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func (s *Server) handleOptions(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	allowOrigin := "http://localhost:8081"
	if origin == "" || strings.HasPrefix(origin, "http://localhost") || strings.HasPrefix(origin, "http://127.0.0.1") || strings.HasPrefix(origin, "null") {
		allowOrigin = origin
	} else if origin != "" {
		allowOrigin = "http://localhost:8081"
	}
	w.Header().Set("Access-Control-Allow-Origin", allowOrigin)
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept")
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		allowOrigin := "http://localhost:8081"
		if origin == "" || strings.HasPrefix(origin, "http://localhost") || strings.HasPrefix(origin, "http://127.0.0.1") || strings.HasPrefix(origin, "null") {
			allowOrigin = origin
		} else if origin != "" {
			allowOrigin = "http://localhost:8081"
		}
		w.Header().Set("Access-Control-Allow-Origin", allowOrigin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept, Authorization")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		if r.URL.Path != "/" && !strings.HasPrefix(r.URL.Path, "/static/") {
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		}
		next.ServeHTTP(w, r)
	})
}

// decodeJSON 解析请求体（限制 10MB，避免超大请求耗尽内存）
func decodeJSON(r *http.Request, v interface{}) error {
	r.Body = http.MaxBytesReader(nil, r.Body, 10<<20)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return fmt.Errorf("读取请求体失败: %w", err)
	}
	body = bytes.TrimPrefix(body, []byte{0xEF, 0xBB, 0xBF}) // 剥离 UTF-8 BOM
	if !utf8.Valid(body) {
		return fmt.Errorf("请求体包含无效的 UTF-8 编码，请确保使用 UTF-8 编码发送数据")
	}
	return json.Unmarshal(body, v)
}
