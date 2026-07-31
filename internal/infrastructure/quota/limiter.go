package quota

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ai-novel/studio/internal/infrastructure/database"
)

// Limiter 调用限制与成本控制器：
//   - 全局每日调用次数 / token 上限（configs 表，可后台改）
//   - 单 IP 每分钟请求数限流（内存）
//   - 并发请求数上限（信号量）
//   - 单模型当日用量与预警/降级
type Limiter struct {
	store *database.Store

	maxConcurrent int
	sem           chan struct{}

	ratePerMin int
	mu         sync.Mutex
	hits       map[string][]time.Time // ip -> 请求时间戳列表
}

// NewLimiter 构造限流器
func NewLimiter(store *database.Store) *Limiter {
	l := &Limiter{
		store: store,
		hits:  make(map[string][]time.Time),
	}
	ctx := context.Background()
	l.maxConcurrent = store.GetConfigInt(ctx, "max_concurrent", 5)
	if l.maxConcurrent < 1 {
		l.maxConcurrent = 5
	}
	l.sem = make(chan struct{}, l.maxConcurrent)
	l.ratePerMin = store.GetConfigInt(ctx, "rate_limit_per_minute", 20)
	if l.ratePerMin < 1 {
		l.ratePerMin = 20
	}
	return l
}

// Guard 表示一次获准的请求，Release 后释放并发槽位
type Guard struct {
	sem chan struct{}
}

// Release 释放并发槽位
func (g *Guard) Release() {
	if g != nil && g.sem != nil {
		select {
		case <-g.sem:
		default:
		}
	}
}

// AllowRequest 综合校验：每日配额 + 限流 + 并发，返回 Guard（用完需 Release）
func (l *Limiter) AllowRequest(ctx context.Context, ip string) (*Guard, error) {
	// 1. 每日配额
	if err := l.checkDailyQuota(ctx); err != nil {
		return nil, err
	}
	// 2. 并发（非阻塞快速失败，避免堆积）
	select {
	case l.sem <- struct{}{}:
		// 占用成功
	default:
		return nil, fmt.Errorf("当前并发请求过多，请稍后再试")
	}
	// 3. 限流（原子 check+record，避免 TOCTOU 竞态）
	if err := l.checkAndRecordRate(ip); err != nil {
		// 限流命中则释放刚占用的并发槽位
		select {
		case <-l.sem:
		default:
		}
		return nil, err
	}
	return &Guard{sem: l.sem}, nil
}

// checkDailyQuota 校验全局每日调用次数与 token 上限
func (l *Limiter) checkDailyQuota(ctx context.Context) error {
	limit := l.store.GetConfigInt(ctx, "daily_call_limit", 500)
	tokenLimit := l.store.GetConfigInt(ctx, "daily_token_limit", 2000000)
	if limit <= 0 && tokenLimit <= 0 {
		return nil
	}
	calls, tokens, err := l.store.DailyTotalUsage(ctx)
	if err != nil {
		return fmt.Errorf("用量统计读取失败: %w", err)
	}
	if limit > 0 && calls >= limit {
		return fmt.Errorf("今日调用额度已用完，请明日再试")
	}
	if tokenLimit > 0 && tokens >= tokenLimit {
		return fmt.Errorf("今日调用额度已用完，请明日再试")
	}
	return nil
}

// checkAndRecordRate 单 IP 每分钟请求数限流：原子地校验并记录命中（避免竞态）
// 每次请求时从数据库重新读取限流配置，确保热更新生效
func (l *Limiter) checkAndRecordRate(ip string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// 每次请求重新读取限流配置，确保 API 热更新 configs 表后立即生效
	l.ratePerMin = l.store.GetConfigInt(context.Background(), "rate_limit_per_minute", 20)
	if l.ratePerMin < 1 {
		l.ratePerMin = 20
	}
	if l.ratePerMin <= 0 {
		return nil
	}

	now := time.Now()
	cutoff := now.Add(-time.Minute)
	hits := l.hits[ip]
	// 清理过期
	kept := hits[:0]
	for _, t := range hits {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= l.ratePerMin {
		l.hits[ip] = kept
		return fmt.Errorf("请求过于频繁，请稍后再试")
	}
	// 校验通过即记录本次命中（同一把锁内完成，无竞态）
	l.hits[ip] = append(kept, now)
	return nil
}

// CheckModelBudget 校验某模型当日用量：
//   - 超过 daily_limit → 视为该模型不可用（触发降级到备用）
//   - 超过 warn_ratio*daily_limit → 返回 warning（非错误）
func (l *Limiter) CheckModelBudget(ctx context.Context, modelName string, dailyLimit int) (warning bool, blocked bool, err error) {
	if dailyLimit <= 0 {
		return false, false, nil
	}
	calls, tokens, err := l.store.DailyModelUsage(ctx, modelName)
	if err != nil {
		return false, false, err
	}
	if calls >= dailyLimit {
		return false, true, nil
	}
	warnRatio := l.store.GetConfigFloat(ctx, "warn_ratio", 0.8)
	if warnRatio > 0 && float64(calls) >= warnRatio*float64(dailyLimit) {
		return true, false, nil
	}
	_ = tokens
	return false, false, nil
}

// EstimateRequestTokens 预估单次请求 token 消耗（含流水线多轮调用系数）
// 标准流水线包含 3-4 次 Agent 调用（Thinker+Worker+Verifier+可能的微调），每次携带完整上下文
func EstimateRequestTokens(inputText string, targetWord int) int {
	in := estimateTokens(inputText)
	out := targetWord
	if out <= 0 {
		out = 1000
	}
	// 中文字符≈token，粗略 1:1.5 → output tokens ≈ word/1.5
	outTokens := int(float64(out)/1.5 + 0.9999)
	// 流水线系数：标准模式 3 次调用 + 1 次微调兜底 = 4x
	pipelineFactor := 4
	return (in + outTokens) * pipelineFactor
}

func estimateTokens(s string) int {
	r := []rune(s)
	if len(r) == 0 {
		return 0
	}
	v := float64(len(r)) / 1.5
	if v < 1 {
		v = 1
	}
	return int(v + 0.9999)
}
