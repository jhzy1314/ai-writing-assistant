package appearance

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// Background 背景条目
type Background struct {
	ID       string `json:"id,omitempty"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Theme    string `json:"theme"`
	File     string `json:"file"`
	Source   string `json:"source"`
	IsSystem bool   `json:"is_system"`
	URL      string `json:"url,omitempty"`
}

// CurrentState 当前背景配置
type CurrentState struct {
	SiderBg    string `json:"sider_bg"`
	SiderLight string `json:"sider_light_bg"`
	EmptyBg    string `json:"empty_bg"`
}

// Config 背景外观配置
type Config struct {
	BackgroundsDir      string        `yaml:"backgrounds_dir"`
	MaxFileSize         int64         `yaml:"max_file_size"`
	AllowedTypes        []string      `yaml:"allowed_types"`
	CurrentSiderBg      string        `yaml:"current_sider_bg"`
	CurrentSiderLightBg string        `yaml:"current_sider_light_bg"`
	CurrentEmptyBg      string        `yaml:"current_empty_bg"`
	Library             []LibraryItem `yaml:"library"`
}

// LibraryItem 背景库条目
type LibraryItem struct {
	Name  string `yaml:"name"`
	Type  string `yaml:"type"`
	File  string `yaml:"file"`
	Theme string `yaml:"theme"`
}

// Service 背景外观管理服务
type Service struct {
	cfg         *Config
	resources   []Resource
	cfgDir      string
	cfgFullPath string
	bgDir       string
	mu          sync.RWMutex
}

// ResourceStatus 资源下载状态（供 handle_appearance.go 使用）
type ResourceStatus struct {
	ID      string `json:"id"`
	Exists  bool   `json:"exists"`
	Success bool   `json:"success,omitempty"`
	Error   string `json:"error,omitempty"`
}

// NewService 创建外观服务（兼容旧版调用）
func NewService(resources []Resource) *Service {
	bgDir := "static/backgrounds"
	os.MkdirAll(bgDir, 0755)

	cfg := &Config{
		BackgroundsDir:      bgDir,
		MaxFileSize:         5242880,
		AllowedTypes:        []string{"png", "jpg", "jpeg", "webp"},
		CurrentSiderBg:      "sider-texture.svg",
		CurrentSiderLightBg: "sider-texture-light.svg",
		CurrentEmptyBg:      "empty-state.svg",
		Library: []LibraryItem{
			{Name: "深色质感纹理", Type: "sider", File: "sider-texture.svg", Theme: "dark"},
			{Name: "科技六边格", Type: "sider", File: "sider-texture-2.svg", Theme: "dark"},
			{Name: "浅色质感纹理", Type: "sider", File: "sider-texture-light.svg", Theme: "light"},
			{Name: "暖色波纹", Type: "sider", File: "sider-texture-light-2.svg", Theme: "light"},
			{Name: "极简空状态", Type: "empty", File: "empty-state.svg", Theme: "light"},
			{Name: "文档卡片", Type: "empty", File: "empty-state-2.svg", Theme: "light"},
			{Name: "禅意菱形", Type: "empty", File: "empty-state-3.svg", Theme: "light"},
		},
	}

	cfgFullPath := filepath.Join("configs", "config.yaml")
	if data, err := os.ReadFile(cfgFullPath); err == nil {
		var raw map[string]interface{}
		if err := yaml.Unmarshal(data, &raw); err == nil {
			if app, ok := raw["appearance"].(map[string]interface{}); ok {
				if v, ok := app["current_sider_bg"].(string); ok && v != "" {
					cfg.CurrentSiderBg = v
				}
				if v, ok := app["current_sider_light_bg"].(string); ok && v != "" {
					cfg.CurrentSiderLightBg = v
				}
				if v, ok := app["current_empty_bg"].(string); ok && v != "" {
					cfg.CurrentEmptyBg = v
				}
			}
		}
	}

	return &Service{
		cfg:         cfg,
		resources:   resources,
		cfgDir:      "configs",
		cfgFullPath: cfgFullPath,
		bgDir:       bgDir,
	}
}

// CheckStartup 启动时检查并下载缺失资源
func (s *Service) CheckStartup() {
	s.mu.RLock()
	resources := make([]Resource, len(s.resources))
	copy(resources, s.resources)
	s.mu.RUnlock()

	for i, r := range resources {
		if !r.Exists() {
			log.Printf("[appearance] 正在下载资源: %s -> %s", r.ID, r.LocalPath())
			if err := r.Download(); err != nil {
				log.Printf("[appearance] 下载 %s 失败: %v", r.ID, err)
			} else {
				log.Printf("[appearance] 下载完成: %s", r.ID)
				s.mu.Lock()
				s.resources[i] = r
				s.mu.Unlock()
			}
		}
	}
}

// DownloadAll 下载所有缺失资源
func (s *Service) DownloadAll() []ResourceStatus {
	s.mu.RLock()
	resources := make([]Resource, len(s.resources))
	copy(resources, s.resources)
	s.mu.RUnlock()

	var results []ResourceStatus
	for i, r := range resources {
		status := ResourceStatus{ID: r.ID, Exists: r.Exists()}
		if err := r.Download(); err != nil {
			status.Error = err.Error()
		} else {
			status.Success = true
			status.Exists = true
			s.mu.Lock()
			s.resources[i] = r
			s.mu.Unlock()
		}
		results = append(results, status)
	}
	return results
}

// Status 返回所有资源状态
func (s *Service) Status() []ResourceStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	statuses := make([]ResourceStatus, len(s.resources))
	for i, r := range s.resources {
		statuses[i] = ResourceStatus{
			ID:     r.ID,
			Exists: r.Exists(),
		}
	}
	return statuses
}

// BgDir 返回背景图片目录
func (s *Service) BgDir() string {
	return s.bgDir
}

// ===== 背景管理方法 =====

// GetCurrentState 获取当前背景配置
func (s *Service) GetCurrentState() *CurrentState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return &CurrentState{
		SiderBg:    s.resolveURL(s.cfg.CurrentSiderBg),
		SiderLight: s.resolveURL(s.cfg.CurrentSiderLightBg),
		EmptyBg:    s.resolveURL(s.cfg.CurrentEmptyBg),
	}
}

// GetCurrentFiles 获取当前背景文件名
func (s *Service) GetCurrentFiles() (sider, siderLight, empty string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg.CurrentSiderBg, s.cfg.CurrentSiderLightBg, s.cfg.CurrentEmptyBg
}

// GetLibrary 获取内置背景库
func (s *Service) GetLibrary() []Background {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var list []Background
	for _, item := range s.cfg.Library {
		list = append(list, Background{
			Name:     item.Name,
			Type:     item.Type,
			Theme:    item.Theme,
			File:     item.File,
			Source:   "builtin",
			IsSystem: true,
			URL:      s.resolveURL(item.File),
		})
	}
	return list
}

// SetBackground 从库中设置背景
func (s *Service) SetBackground(bgType, theme, file string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch bgType {
	case "sider":
		if theme == "dark" {
			s.cfg.CurrentSiderBg = file
		} else {
			s.cfg.CurrentSiderLightBg = file
		}
	case "empty":
		s.cfg.CurrentEmptyBg = file
	default:
		return fmt.Errorf("未知背景类型: %s", bgType)
	}
	return s.saveConfigLocked()
}

// UploadBackground 上传自定义背景
func (s *Service) UploadBackground(bgType, theme string, data io.Reader, filename string) (*Background, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	allowed := map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".webp": true}
	if !allowed[ext] {
		return nil, fmt.Errorf("不支持的文件格式: %s，仅支持 png/jpg/jpeg/webp", ext)
	}

	ts := time.Now().Format("20060730-150405")
	newName := fmt.Sprintf("custom-%s-%s%s", bgType, ts, ext)
	dest := filepath.Join(s.bgDir, newName)

	f, err := os.Create(dest)
	if err != nil {
		return nil, fmt.Errorf("创建文件失败: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, data); err != nil {
		os.Remove(dest)
		return nil, fmt.Errorf("写入文件失败: %w", err)
	}

	bg := &Background{
		Name:   fmt.Sprintf("自定义%s背景", bgType),
		Type:   bgType,
		Theme:  theme,
		File:   newName,
		Source: "upload",
		URL:    s.resolveURL(newName),
	}

	s.mu.Lock()
	switch bgType {
	case "sider":
		if theme == "dark" {
			s.cfg.CurrentSiderBg = newName
		} else {
			s.cfg.CurrentSiderLightBg = newName
		}
	case "empty":
		s.cfg.CurrentEmptyBg = newName
	}
	s.saveConfigLocked()
	s.mu.Unlock()

	return bg, nil
}

// GenerateFromPicsum 从 Picsum 随机生成背景
func (s *Service) GenerateFromPicsum(bgType, theme, seed string) (*Background, error) {
	if seed == "" {
		seed = fmt.Sprintf("bg-%d", time.Now().UnixNano())
	}
	ts := time.Now().Format("20060730-150405")
	newName := fmt.Sprintf("picsum-%s-%s-%s.jpg", bgType, ts, seed[:8])
	dest := filepath.Join(s.bgDir, newName)

	width := "1920"
	height := "1080"
	url := fmt.Sprintf("https://picsum.photos/seed/%s/%s/%s", seed, width, height)
	if theme == "dark" {
		url += "?grayscale&blur=2"
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("下载图片失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("图片API返回状态码: %d", resp.StatusCode)
	}

	f, err := os.Create(dest)
	if err != nil {
		return nil, fmt.Errorf("创建文件失败: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		os.Remove(dest)
		return nil, fmt.Errorf("写入文件失败: %w", err)
	}

	bg := &Background{
		Name:   fmt.Sprintf("Picsum %s", seed[:8]),
		Type:   bgType,
		Theme:  theme,
		File:   newName,
		Source: "api",
		URL:    s.resolveURL(newName),
	}

	s.mu.Lock()
	switch bgType {
	case "sider":
		if theme == "dark" {
			s.cfg.CurrentSiderBg = newName
		} else {
			s.cfg.CurrentSiderLightBg = newName
		}
	case "empty":
		s.cfg.CurrentEmptyBg = newName
	}
	s.saveConfigLocked()
	s.mu.Unlock()

	return bg, nil
}

// RandomBackground 随机切换背景
func (s *Service) RandomBackground(bgType, theme string) (*Background, error) {
	seed := fmt.Sprintf("random-%d", time.Now().UnixNano())
	return s.GenerateFromPicsum(bgType, theme, seed)
}

// ListCustom 列出用户上传/生成的自定义背景
func (s *Service) ListCustom() ([]Background, error) {
	entries, err := os.ReadDir(s.bgDir)
	if err != nil {
		return nil, err
	}
	var list []Background
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, "custom-") || strings.HasPrefix(name, "picsum-") {
			list = append(list, Background{
				Name:   name,
				Type:   guessType(name),
				Theme:  guessTheme(name),
				File:   name,
				Source: "upload",
				URL:    s.resolveURL(name),
			})
		}
	}
	return list, nil
}

// ResetToDefault 恢复默认背景
func (s *Service) ResetToDefault() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cfg.CurrentSiderBg = "sider-texture.svg"
	s.cfg.CurrentSiderLightBg = "sider-texture-light.svg"
	s.cfg.CurrentEmptyBg = "empty-state.svg"
	return s.saveConfigLocked()
}

func (s *Service) resolveURL(filename string) string {
	return fmt.Sprintf("/backgrounds/%s", filename)
}

func guessType(name string) string {
	if strings.Contains(name, "empty") {
		return "empty"
	}
	return "sider"
}

func guessTheme(name string) string {
	if strings.Contains(name, "light") {
		return "light"
	}
	return "dark"
}

func (s *Service) saveConfigLocked() error {
	data, err := os.ReadFile(s.cfgFullPath)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("解析YAML失败: %w", err)
	}

	appearance, ok := raw["appearance"].(map[string]interface{})
	if !ok {
		appearance = make(map[string]interface{})
		raw["appearance"] = appearance
	}

	appearance["current_sider_bg"] = s.cfg.CurrentSiderBg
	appearance["current_sider_light_bg"] = s.cfg.CurrentSiderLightBg
	appearance["current_empty_bg"] = s.cfg.CurrentEmptyBg

	out, err := yaml.Marshal(&raw)
	if err != nil {
		return fmt.Errorf("序列化YAML失败: %w", err)
	}

	return os.WriteFile(s.cfgFullPath, out, 0644)
}
