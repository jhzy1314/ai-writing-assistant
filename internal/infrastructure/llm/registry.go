package llm

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ai-novel/studio/internal/infrastructure/database"
)

// Role 角色枚举（与规格「模型角色池」对应）
type Role string

const (
	RoleThinker  Role = "thinker"  // 规划师
	RoleWorker   Role = "worker"   // 文笔创作者
	RoleVerifier Role = "verifier" // 校验官
	RoleHelper   Role = "helper"   // 轻助手
)

// AllRoles 全部角色
var AllRoles = []Role{RoleThinker, RoleWorker, RoleVerifier, RoleHelper}

func (r Role) String() string { return string(r) }

// ValidRole 校验角色合法性
func ValidRole(r string) bool {
	for _, v := range AllRoles {
		if string(v) == r {
			return true
		}
	}
	return false
}

// Registry 模型适配层注册中心：
//   - 依据 models 表 + role_models 表，将「角色」解析为按优先级排序的 ModelAdapter 列表
//   - 适配器实例按模型名缓存，模型配置变更时通过 Invalidate 失效
type Registry struct {
	store      *database.Store
	mu         sync.RWMutex
	adapters   map[string]ModelAdapter // key: model name
	backoff    map[string]time.Time    // 429 冷却截止时间
	errCounts  map[string]int          // 错误计数（同一模型）
}

// NewRegistry 构造注册中心
func NewRegistry(store *database.Store) *Registry {
	return &Registry{store: store, adapters: make(map[string]ModelAdapter), backoff: make(map[string]time.Time), errCounts: make(map[string]int)}
}

// AdaptersForRole 返回某角色绑定的有序适配器列表（主模型在前，备用在后）
func (r *Registry) AdaptersForRole(ctx context.Context, role Role) ([]ModelAdapter, error) {
	names, err := r.store.RoleModelNames(ctx, role.String())
	if err != nil {
		return nil, fmt.Errorf("读取角色 %s 模型绑定失败: %w", role, err)
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("角色 %s 未绑定任何模型，请在后台配置", role)
	}
	out := make([]ModelAdapter, 0, len(names))
	for _, name := range names {
		// 429 冷却检查：1分钟同模型3次限流后冷却10分钟
		if r.isBackoff(name) {
			continue
		}
		a, err := r.adapterForName(ctx, name)
		if err != nil {
			continue
		}
		out = append(out, a)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("角色 %s 绑定的模型全部不可用", role)
	}
	return out, nil
}

// AdapterByName 按名称取适配器（手动模式使用）
func (r *Registry) AdapterByName(ctx context.Context, name string) (ModelAdapter, error) {
	return r.adapterForName(ctx, name)
}

// adapterForName 取或创建某模型的适配器（带缓存）
func (r *Registry) adapterForName(ctx context.Context, name string) (ModelAdapter, error) {
	r.mu.RLock()
	if a, ok := r.adapters[name]; ok {
		r.mu.RUnlock()
		return a, nil
	}
	r.mu.RUnlock()

	m, err := r.store.GetModelByName(ctx, name)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, fmt.Errorf("模型 %s 不存在", name)
	}
	if m.Status != "active" {
		return nil, fmt.Errorf("模型 %s 已停用", name)
	}
	a, err := NewOpenAICompatible(ctx, m.Vendor, m.APIKey, m.APIEndpoint, m.Name, m.MaxTokens)
	if err != nil {
		return nil, err
	}

	r.mu.Lock()
	r.adapters[name] = a
	r.mu.Unlock()
	return a, nil
}

// Invalidate 失效某模型缓存（配置变更后调用）
func (r *Registry) Invalidate(name string) {
	r.mu.Lock()
	delete(r.adapters, name)
	r.mu.Unlock()
}

// InvalidateAll 失效全部缓存
func (r *Registry) InvalidateAll() {
	r.mu.Lock()
	r.adapters = make(map[string]ModelAdapter)
	r.mu.Unlock()
}

// Mark429 标记模型收到 429 限流，累计 3 次后冷却 10 分钟
func (r *Registry) Mark429(name string) {
	r.mu.Lock()
	r.errCounts[name]++
	if r.errCounts[name] >= 3 {
		r.backoff[name] = time.Now().Add(10 * time.Minute)
		r.errCounts[name] = 0
	}
	r.mu.Unlock()
}

func (r *Registry) isBackoff(name string) bool {
	r.mu.RLock()
	until, ok := r.backoff[name]
	r.mu.RUnlock()
	return ok && time.Now().Before(until)
}
