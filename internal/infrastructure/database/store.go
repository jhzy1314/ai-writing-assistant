package database

import (
	"context"
	"database/sql"
	"strconv"
	"time"

	"github.com/google/uuid"
)

// Store 数据访问层统一入口，聚合各实体仓库方法
type Store struct {
	db *DB
}

// NewStore 由 *DB 构造 Store
func NewStore(db *DB) *Store {
	return &Store{db: db}
}

// DB 暴露底层句柄（供需要原生 SQL 的场景使用）
func (s *Store) DB() *sql.DB { return s.db.DB }

func newID() string { return uuid.NewString() }

func now() string { return time.Now().Format(time.RFC3339) }

// ===== configs 表（全局配置/限额） =====

// ConfigRow configs 表一行
type ConfigRow struct {
	Key         string `json:"key"`
	Value       string `json:"value"`
	Description string `json:"description"`
}

// GetConfig 读取单个配置值，不存在返回默认值
func (s *Store) GetConfig(ctx context.Context, key string, defaultVal string) string {
	var v string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM configs WHERE key=?`, key).Scan(&v)
	if err != nil {
		return defaultVal
	}
	return v
}

// GetConfigInt 读取整型配置
func (s *Store) GetConfigInt(ctx context.Context, key string, defaultVal int) int {
	v := s.GetConfig(ctx, key, "")
	if v == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return defaultVal
	}
	return n
}

// GetConfigFloat 读取浮点配置
func (s *Store) GetConfigFloat(ctx context.Context, key string, defaultVal float64) float64 {
	v := s.GetConfig(ctx, key, "")
	if v == "" {
		return defaultVal
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return defaultVal
	}
	return f
}

// SetConfig 写入配置（幂等）
func (s *Store) SetConfig(ctx context.Context, key, value, description string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO configs(id, key, value, description) VALUES(?,?,?,?)
		 ON CONFLICT(key) DO UPDATE SET value=excluded.value, description=COALESCE(NULLIF(excluded.description,''), configs.description)`,
		key, key, value, description)
	return err
}

// ListConfigs 列出全部配置
func (s *Store) ListConfigs(ctx context.Context) ([]ConfigRow, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT key, value, description FROM configs ORDER BY key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ConfigRow{}
	for rows.Next() {
		var c ConfigRow
		if err := rows.Scan(&c.Key, &c.Value, &c.Description); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ===== usage_daily 表（每日用量统计，用于配额/降级） =====

// IncrUsage 累加某模型当日调用次数与 token 消耗
func (s *Store) IncrUsage(ctx context.Context, modelName string, calls, tokens int) error {
	day := time.Now().Format("2006-01-02")
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO usage_daily(day, model_name, calls, tokens) VALUES(?,?,?,?)
		 ON CONFLICT(day, model_name) DO UPDATE SET calls=calls+excluded.calls, tokens=tokens+excluded.tokens`,
		day, modelName, calls, tokens)
	return err
}

// DailyTotalUsage 返回当日全局总调用次数与总 token
func (s *Store) DailyTotalUsage(ctx context.Context) (calls int, tokens int, err error) {
	day := time.Now().Format("2006-01-02")
	err = s.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(calls),0), COALESCE(SUM(tokens),0) FROM usage_daily WHERE day=?`, day).
		Scan(&calls, &tokens)
	return
}

// DailyModelUsage 返回某模型当日调用次数与 token
func (s *Store) DailyModelUsage(ctx context.Context, modelName string) (calls int, tokens int, err error) {
	day := time.Now().Format("2006-01-02")
	err = s.db.QueryRowContext(ctx,
		`SELECT COALESCE(calls,0), COALESCE(tokens,0) FROM usage_daily WHERE day=? AND model_name=?`, day, modelName).
		Scan(&calls, &tokens)
	if err == sql.ErrNoRows {
		return 0, 0, nil
	}
	return
}

// DailyUsageByModel 返回当日各模型用量明细
type ModelUsageRow struct {
	ModelName string `json:"model_name"`
	Calls     int    `json:"calls"`
	Tokens    int    `json:"tokens"`
}

func (s *Store) DailyUsageByModel(ctx context.Context) ([]ModelUsageRow, error) {
	day := time.Now().Format("2006-01-02")
	rows, err := s.db.QueryContext(ctx, `SELECT model_name, calls, tokens FROM usage_daily WHERE day=?`, day)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ModelUsageRow{}
	for rows.Next() {
		var r ModelUsageRow
		if err := rows.Scan(&r.ModelName, &r.Calls, &r.Tokens); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// PeriodUsage 统计最近 N 天各模型用量
func (s *Store) PeriodUsage(ctx context.Context, days int) ([]ModelUsageRow, error) {
	since := time.Now().AddDate(0, 0, -days).Format("2006-01-02")
	q := `SELECT model_name, SUM(calls), SUM(tokens) FROM usage_daily WHERE day>=? GROUP BY model_name ORDER BY SUM(tokens) DESC`
	rows, err := s.db.QueryContext(ctx, q, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ModelUsageRow{}
	for rows.Next() {
		var r ModelUsageRow
		if err := rows.Scan(&r.ModelName, &r.Calls, &r.Tokens); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
