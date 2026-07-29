package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ai-novel/studio/internal/infrastructure/config"
	_ "modernc.org/sqlite"
)

// DB 包装底层 *sql.DB，并持有各仓库方法
type DB struct {
	*sql.DB
}

// Open 打开/创建 SQLite 数据库并执行建表迁移，返回 *DB
func Open(ctx context.Context, sqlitePath string) (*DB, error) {
	if dir := filepath.Dir(sqlitePath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("创建数据目录失败: %w", err)
		}
	}

	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)&_pragma=busy_timeout(10000)&_pragma=synchronous(NORMAL)", sqlitePath)
	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("打开 SQLite 失败: %w", err)
	}
	// SQLite 单写多读，限制连接数避免锁竞争
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)

	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("SQLite 连通失败: %w", err)
	}

	db := &DB{sqlDB}
	if err := db.migrate(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, err
	}
	return db, nil
}

// migrate 建表 DDL（CREATE TABLE IF NOT EXISTS，幂等）
func (db *DB) migrate(ctx context.Context) error {
	_, err := db.ExecContext(ctx, schemaDDL)
	if err != nil {
		return fmt.Errorf("数据库迁移失败: %w", err)
	}
	if err := db.migrateModelsColumns(ctx); err != nil {
		return fmt.Errorf("模型表列迁移失败: %w", err)
	}
	if err := db.migrateChaptersColumns(ctx); err != nil {
		return fmt.Errorf("章节表列迁移失败: %w", err)
	}
	return nil
}

// migrateModelsColumns 为 models 表补充新增列（SQLite 不支持 ALTER TABLE ADD COLUMN IF NOT EXISTS，通过 PRAGMA 检测后添加）
func (db *DB) migrateModelsColumns(ctx context.Context) error {
	cols := map[string]string{
		"is_custom":      "INTEGER NOT NULL DEFAULT 0",
		"context_limit":  "INTEGER NOT NULL DEFAULT 4096",
		"support_stream": "INTEGER NOT NULL DEFAULT 1",
		"is_default":     "INTEGER NOT NULL DEFAULT 0",
		"description":    "TEXT NOT NULL DEFAULT ''",
		"temperature":    "REAL NOT NULL DEFAULT 0.7",
		"top_p":          "REAL NOT NULL DEFAULT 0.9",
	}
	rows, err := db.QueryContext(ctx, "PRAGMA table_info(models)")
	if err != nil {
		return fmt.Errorf("读取 models 表结构失败: %w", err)
	}
	defer rows.Close()
	existing := map[string]bool{}
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull int
		var dflt sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			return fmt.Errorf("解析 models 列信息失败: %w", err)
		}
		existing[name] = true
	}
	for col, def := range cols {
		if existing[col] {
			continue
		}
		if _, err := db.ExecContext(ctx, "ALTER TABLE models ADD COLUMN "+col+" "+def); err != nil {
			return fmt.Errorf("添加列 %s 失败: %w", col, err)
		}
	}
	return nil
}


// migrateChaptersColumns 为 chapters 表补充 tags、synopsis 列
func (db *DB) migrateChaptersColumns(ctx context.Context) error {
	cols := map[string]string{
		"tags":     "TEXT NOT NULL DEFAULT ''",
		"synopsis": "TEXT NOT NULL DEFAULT ''",
	}
	rows, err := db.QueryContext(ctx, "PRAGMA table_info(chapters)")
	if err != nil {
		return err
	}
	defer rows.Close()
	existing := map[string]bool{}
	for rows.Next() {
		var cid int; var name, typ string; var notnull int; var dflt sql.NullString; var pk int
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			return err
		}
		existing[name] = true
	}
	for col, def := range cols {
		if existing[col] { continue }
		if _, err := db.ExecContext(ctx, "ALTER TABLE chapters ADD COLUMN "+col+" "+def); err != nil {
			return err
		}
	}
	return nil
}
// schemaDDL 数据库表结构 DDL（对应规格第六章）
const schemaDDL = `
CREATE TABLE IF NOT EXISTS projects (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    type        TEXT NOT NULL DEFAULT '',
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS documents (
    id          TEXT PRIMARY KEY,
    project_id  TEXT NOT NULL,
    title       TEXT NOT NULL DEFAULT '',
    content     TEXT NOT NULL DEFAULT '',
    version     INTEGER NOT NULL DEFAULT 1,
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_documents_project ON documents(project_id);

CREATE TABLE IF NOT EXISTS characters (
    id          TEXT PRIMARY KEY,
    project_id  TEXT NOT NULL,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    avatar_url  TEXT NOT NULL DEFAULT '',
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at  TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_characters_project ON characters(project_id);

CREATE TABLE IF NOT EXISTS world_settings (
    id          TEXT PRIMARY KEY,
    project_id  TEXT NOT NULL,
    title       TEXT NOT NULL,
    content     TEXT NOT NULL DEFAULT '',
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at  TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_worldsettings_project ON world_settings(project_id);

CREATE TABLE IF NOT EXISTS materials (
    id          TEXT PRIMARY KEY,
    project_id  TEXT NOT NULL,
    name        TEXT NOT NULL,
    content     TEXT NOT NULL DEFAULT '',
    file_type   TEXT NOT NULL DEFAULT '',
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_materials_project ON materials(project_id);

CREATE TABLE IF NOT EXISTS templates (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    category    TEXT NOT NULL DEFAULT '',
    content     TEXT NOT NULL DEFAULT '',
    is_system   INTEGER NOT NULL DEFAULT 0,
    created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS models (
    id           TEXT PRIMARY KEY,
    name         TEXT NOT NULL UNIQUE,
    vendor       TEXT NOT NULL DEFAULT '',
    api_endpoint TEXT NOT NULL DEFAULT '',
    api_key      TEXT NOT NULL DEFAULT '',
    status       TEXT NOT NULL DEFAULT 'active',
    daily_limit  INTEGER NOT NULL DEFAULT 0,
    created_at   TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS role_models (
    id       TEXT PRIMARY KEY,
    role     TEXT NOT NULL,
    model_id TEXT NOT NULL,
    priority INTEGER NOT NULL DEFAULT 0,
    UNIQUE(role, model_id),
    FOREIGN KEY (model_id) REFERENCES models(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_role_models_role ON role_models(role, priority);

CREATE TABLE IF NOT EXISTS generation_logs (
    id                 TEXT PRIMARY KEY,
    project_id         TEXT NOT NULL DEFAULT '',
    role               TEXT NOT NULL DEFAULT '',
    model_name         TEXT NOT NULL DEFAULT '',
    prompt_tokens      INTEGER NOT NULL DEFAULT 0,
    completion_tokens  INTEGER NOT NULL DEFAULT 0,
    duration_ms        INTEGER NOT NULL DEFAULT 0,
    status             TEXT NOT NULL DEFAULT 'ok',
    error_msg          TEXT NOT NULL DEFAULT '',
    created_at         TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_logs_created ON generation_logs(created_at);
CREATE INDEX IF NOT EXISTS idx_logs_model ON generation_logs(model_name, created_at);

CREATE TABLE IF NOT EXISTS configs (
    id          TEXT PRIMARY KEY,
    key         TEXT NOT NULL UNIQUE,
    value       TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS usage_daily (
    day         TEXT NOT NULL,
    model_name  TEXT NOT NULL,
    calls       INTEGER NOT NULL DEFAULT 0,
    tokens      INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (day, model_name)
);

CREATE TABLE IF NOT EXISTS volumes (
    id          TEXT PRIMARY KEY,
    project_id  TEXT NOT NULL,
    title       TEXT NOT NULL DEFAULT '',
    sort_order  INTEGER NOT NULL DEFAULT 0,
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at  TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_volumes_project ON volumes(project_id, sort_order);

CREATE TABLE IF NOT EXISTS chapters (
    id          TEXT PRIMARY KEY,
    project_id  TEXT NOT NULL,
    volume_id   TEXT NOT NULL DEFAULT '',
    title       TEXT NOT NULL DEFAULT '',
    content     TEXT NOT NULL DEFAULT '',
    word_count  INTEGER NOT NULL DEFAULT 0,
    sort_order  INTEGER NOT NULL DEFAULT 0,
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at  TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_chapters_project ON chapters(project_id, sort_order);

CREATE TABLE IF NOT EXISTS chapter_versions (
    id          TEXT PRIMARY KEY,
    chapter_id  TEXT NOT NULL,
    title       TEXT NOT NULL DEFAULT '',
    content     TEXT NOT NULL DEFAULT '',
    version     INTEGER NOT NULL DEFAULT 1,
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (chapter_id) REFERENCES chapters(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_chapter_versions ON chapter_versions(chapter_id);
`

// Seed 从配置种子初始化 configs / models / role_models（幂等：存在则跳过/更新）
func (db *DB) Seed(ctx context.Context, cfg *config.Config) error {
	if cfg == nil {
		return nil
	}
	// 1. 种子 configs 表（限额参数）
	quotaSet := [][3]string{
		{"daily_call_limit", itoa(cfg.Quotas.DailyCallLimit), "全局每日总调用次数上限"},
		{"daily_token_limit", itoa(cfg.Quotas.DailyTokenLimit), "全局每日总 token 消耗上限"},
		{"per_request_token_limit", itoa(cfg.Quotas.PerRequestTokenLimit), "单次请求 token 上限"},
		{"light_input_char_limit", itoa(cfg.Quotas.LightInputCharLimit), "轻量化模式输入字符上限"},
		{"max_iterations", itoa(cfg.Quotas.MaxIterations), "流水线最大迭代轮次"},
		{"rate_limit_per_minute", itoa(cfg.Quotas.RateLimitPerMinute), "单 IP 每分钟请求数"},
		{"max_concurrent", itoa(cfg.Quotas.MaxConcurrent), "并发请求数上限"},
		{"warn_ratio", ftoa(cfg.Quotas.WarnRatio), "单模型当日消耗预警阈值比例"},
	}
	for _, q := range quotaSet {
		if _, err := db.ExecContext(ctx,
			`INSERT INTO configs(id, key, value, description) VALUES(?,?,?,?)
			 ON CONFLICT(key) DO UPDATE SET value=excluded.value, description=excluded.description`,
			q[0], q[0], q[1], q[2]); err != nil {
			return fmt.Errorf("seed configs %s: %w", q[0], err)
		}
	}

	// 2. 种子 models 表
	for _, m := range cfg.Models {
		status := m.Status
		if status == "" {
			status = "active"
		}
		if _, err := db.ExecContext(ctx,
			`INSERT INTO models(id, name, vendor, api_endpoint, api_key, status, daily_limit, is_custom, context_limit, support_stream, is_default, description, temperature, top_p)
			 VALUES(?,?,?,?,?,?,?,1,65536,1,0,'',0.7,0.9)
			 ON CONFLICT(name) DO UPDATE SET vendor=excluded.vendor, api_endpoint=excluded.api_endpoint,
			 api_key=excluded.api_key, status=excluded.status, daily_limit=excluded.daily_limit,
			 context_limit=excluded.context_limit`,
			m.Name, m.Name, m.Vendor, m.APIEndpoint, m.APIKey, status, m.DailyLimit); err != nil {
			return fmt.Errorf("seed model %s: %w", m.Name, err)
		}
	}

	// 3. 种子 role_models 表（按 priority 顺序）
	for role, modelNames := range cfg.RoleModels {
		// 清空该角色旧绑定后重建，保证与配置一致
		if _, err := db.ExecContext(ctx, `DELETE FROM role_models WHERE role=?`, role); err != nil {
			return fmt.Errorf("clear role_models %s: %w", role, err)
		}
		for priority, name := range modelNames {
			var modelID string
			if err := db.QueryRowContext(ctx, `SELECT id FROM models WHERE name=?`, name).Scan(&modelID); err != nil {
				if err == sql.ErrNoRows {
					return fmt.Errorf("role_models: 模型 %s 未在 models 表中定义", name)
				}
				return fmt.Errorf("query model %s: %w", name, err)
			}
			if _, err := db.ExecContext(ctx,
				`INSERT INTO role_models(id, role, model_id, priority) VALUES(?,?,?,?)`,
				role+"-"+modelID, role, modelID, priority); err != nil {
				return fmt.Errorf("seed role_models %s/%s: %w", role, name, err)
			}
		}
	}
	return nil
}

func itoa(i int) string  { return fmt.Sprintf("%d", i) }
func ftoa(f float64) string { return fmt.Sprintf("%g", f) }

// DBMigration 记录已执行的迁移版本，幂等执行
type DBMigration struct {
	Version int
	Name    string
	SQL     string
}

// RunMigrations 按版本顺序执行数据库迁移（幂等，跳过已执行版本）
func (db *DB) RunMigrations(ctx context.Context, migrations []DBMigration) error {
	for _, m := range migrations {
		var exists int
		_ = db.QueryRowContext(ctx, `SELECT 1 FROM configs WHERE key='migration_'+?`, fmt.Sprint(m.Version)).Scan(&exists)
		if exists == 1 {
			continue
		}
		if _, err := db.ExecContext(ctx, m.SQL); err != nil {
			return fmt.Errorf("迁移 v%d %s 失败: %w", m.Version, m.Name, err)
		}
		_, _ = db.ExecContext(ctx, `INSERT OR REPLACE INTO configs(id,key,value,description) VALUES(?,?,?,?)`,
			"migration_"+fmt.Sprint(m.Version), "migration_"+fmt.Sprint(m.Version), fmt.Sprint(m.Version), m.Name)
	}
	return nil
}
