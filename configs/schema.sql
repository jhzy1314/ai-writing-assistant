-- ============================================================
-- AI Novel Studio 数据库表结构 DDL (SQLite)
-- 对应规格第六章。运行时由 internal/infrastructure/database/db.go
-- 的 migrate() 自动执行（CREATE TABLE IF NOT EXISTS，幂等）。
-- 本文件仅作交付参考，无需手动执行。
-- ============================================================

-- 1. 项目
CREATE TABLE projects (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    type        TEXT NOT NULL DEFAULT '',
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

-- 2. 稿件版本（documents 即版本快照）
CREATE TABLE documents (
    id          TEXT PRIMARY KEY,
    project_id  TEXT NOT NULL,
    title       TEXT NOT NULL DEFAULT '',
    content     TEXT NOT NULL DEFAULT '',
    version     INTEGER NOT NULL DEFAULT 1,
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
);
CREATE INDEX idx_documents_project ON documents(project_id);

-- 3. 人物卡
CREATE TABLE characters (
    id          TEXT PRIMARY KEY,
    project_id  TEXT NOT NULL,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    avatar_url  TEXT NOT NULL DEFAULT '',
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at  TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
);
CREATE INDEX idx_characters_project ON characters(project_id);

-- 4. 世界观设定
CREATE TABLE world_settings (
    id          TEXT PRIMARY KEY,
    project_id  TEXT NOT NULL,
    title       TEXT NOT NULL,
    content     TEXT NOT NULL DEFAULT '',
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at  TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
);
CREATE INDEX idx_worldsettings_project ON world_settings(project_id);

-- 5. 素材
CREATE TABLE materials (
    id          TEXT PRIMARY KEY,
    project_id  TEXT NOT NULL,
    name        TEXT NOT NULL,
    content     TEXT NOT NULL DEFAULT '',
    file_type   TEXT NOT NULL DEFAULT '',
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
);
CREATE INDEX idx_materials_project ON materials(project_id);

-- 6. 提示词模板
CREATE TABLE templates (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    category    TEXT NOT NULL DEFAULT '',
    content     TEXT NOT NULL DEFAULT '',
    is_system   INTEGER NOT NULL DEFAULT 0,   -- 1=系统内置(不可删改), 0=用户自定义
    created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

-- 7. 模型配置
CREATE TABLE models (
    id             TEXT PRIMARY KEY,
    name           TEXT NOT NULL UNIQUE,
    vendor         TEXT NOT NULL DEFAULT '',
    api_endpoint   TEXT NOT NULL DEFAULT '',
    api_key        TEXT NOT NULL DEFAULT '',
    status         TEXT NOT NULL DEFAULT 'active',
    daily_limit    INTEGER NOT NULL DEFAULT 0,
    is_custom      INTEGER NOT NULL DEFAULT 0,
    context_limit  INTEGER NOT NULL DEFAULT 4096,
    support_stream INTEGER NOT NULL DEFAULT 1,
    is_default     INTEGER NOT NULL DEFAULT 0,
    description    TEXT NOT NULL DEFAULT '',
    temperature    REAL NOT NULL DEFAULT 0.7,
    top_p          REAL NOT NULL DEFAULT 0.9,
    created_at     TEXT NOT NULL DEFAULT (datetime('now'))
);

-- 8. 角色-模型绑定（优先级）
CREATE TABLE role_models (
    id       TEXT PRIMARY KEY,
    role     TEXT NOT NULL,                   -- thinker / worker / verifier / helper
    model_id TEXT NOT NULL,
    priority INTEGER NOT NULL DEFAULT 0,      -- 0=主模型，数字越大越靠后
    UNIQUE(role, model_id),
    FOREIGN KEY (model_id) REFERENCES models(id) ON DELETE CASCADE
);
CREATE INDEX idx_role_models_role ON role_models(role, priority);

-- 9. 模型调用日志
CREATE TABLE generation_logs (
    id                 TEXT PRIMARY KEY,
    project_id         TEXT NOT NULL DEFAULT '',
    role               TEXT NOT NULL DEFAULT '',
    model_name         TEXT NOT NULL DEFAULT '',
    prompt_tokens      INTEGER NOT NULL DEFAULT 0,
    completion_tokens  INTEGER NOT NULL DEFAULT 0,
    duration_ms        INTEGER NOT NULL DEFAULT 0,
    status             TEXT NOT NULL DEFAULT 'ok', -- ok / partial / error
    error_msg          TEXT NOT NULL DEFAULT '',
    created_at         TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX idx_logs_created ON generation_logs(created_at);
CREATE INDEX idx_logs_model ON generation_logs(model_name, created_at);

-- 10. 全局配置（限额阈值等，后台可改无需重启）
CREATE TABLE configs (
    id          TEXT PRIMARY KEY,
    key         TEXT NOT NULL UNIQUE,
    value       TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT ''
);

-- 11. 每日用量统计（配额/降级依据）
CREATE TABLE usage_daily (
    day         TEXT NOT NULL,                -- YYYY-MM-DD
    model_name  TEXT NOT NULL,
    calls       INTEGER NOT NULL DEFAULT 0,
    tokens      INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (day, model_name)
);

-- 12. 卷（章节层级管理）
CREATE TABLE volumes (
    id          TEXT PRIMARY KEY,
    project_id  TEXT NOT NULL,
    title       TEXT NOT NULL DEFAULT '',
    sort_order  INTEGER NOT NULL DEFAULT 0,
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at  TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
);
CREATE INDEX idx_volumes_project ON volumes(project_id, sort_order);

-- 13. 章节
CREATE TABLE chapters (
    id          TEXT PRIMARY KEY,
    project_id  TEXT NOT NULL,
    volume_id   TEXT NOT NULL DEFAULT '',
    title       TEXT NOT NULL DEFAULT '',
    content     TEXT NOT NULL DEFAULT '',
    word_count  INTEGER NOT NULL DEFAULT 0,
    sort_order  INTEGER NOT NULL DEFAULT 0,
    tags        TEXT NOT NULL DEFAULT '',
    synopsis    TEXT NOT NULL DEFAULT '',
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at  TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
);
CREATE INDEX idx_chapters_project ON chapters(project_id, sort_order);

-- 14. 章节版本
CREATE TABLE chapter_versions (
    id          TEXT PRIMARY KEY,
    chapter_id  TEXT NOT NULL,
    title       TEXT NOT NULL DEFAULT '',
    content     TEXT NOT NULL DEFAULT '',
    version     INTEGER NOT NULL DEFAULT 1,
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (chapter_id) REFERENCES chapters(id) ON DELETE CASCADE
);
CREATE INDEX idx_chapter_versions ON chapter_versions(chapter_id);
