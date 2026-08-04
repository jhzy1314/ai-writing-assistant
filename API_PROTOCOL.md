# AI辅助写作助手 标准化接口协议文档

> 基于 `internal/interfaces/api` 实际实现整理。协议版本：v1.0

---

## 一、通用约定

### 1.1 基础信息

| 项 | 值 |
|---|---|
| 协议 | HTTP/1.1 |
| 默认端口 | `8081` |
| 字符集 | UTF-8 |
| 请求体格式 | `application/json`（除素材上传为 `multipart/form-data`） |
| 响应体格式 | `application/json; charset=utf-8` |
| CORS | `Access-Control-Allow-Origin: *` |
| 鉴权 | 当前版本无鉴权（单机本地部署） |

### 1.2 统一响应结构

**成功（业务数据包络）**

```json
{ "items": [ ... ] }          // 列表型
{ "item":  { ... } }          // 单实体型
{ "<key>": <value>, ... }     // 复合型（如 usage）
```

**失败**

```json
{ "error": "<友好中文错误信息>" }
```

HTTP 状态码语义：

| 状态码 | 含义 |
|---|---|
| `200 OK` | 请求成功，返回业务数据 |
| `201 Created` | 资源创建成功，返回 `{item}` |
| `204 No Content` | 删除成功，无响应体 |
| `400 Bad Request` | 参数缺失/非法/解析失败 |
| `403 Forbidden` | 系统内置资源不可改删 |
| `404 Not Found` | 资源不存在 |
| `429 Too Many Requests` | 限流/配额耗尽 |
| `500 Internal Server Error` | 服务端异常 |
| `503 Service Unavailable` | 无可用模型/全部模型失败 |

### 1.3 通用请求限制

| 项 | 默认值 | 配置键 |
|---|---|---|
| 请求体最大 | 10 MB | — |
| 全局每日调用上限 | 500 | `daily_call_limit` |
| 全局每日 Token 上限 | 2,000,000 | `daily_token_limit` |
| 单请求 Token 上限 | 8,000 | `per_request_token_limit` |
| 单 IP 每分钟限流 | 20 | `rate_limit_per_minute` |
| 并发上限 | 5 | `max_concurrent` |
| 微调迭代上限 | 3 | `max_iterations` |
| 轻量模式输入字符上限 | 500 | `light_input_char_limit` |

---

## 二、API 路由总览

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/api/generate` | 创作请求（SSE 流式） |
| POST | `/api/verify` | 独立逻辑自检 |
| GET | `/api/projects` | 项目列表 |
| POST | `/api/projects` | 新建项目 |
| GET | `/api/projects/{id}` | 项目详情（附最新版本+资源计数） |
| PUT | `/api/projects/{id}` | 更新项目 |
| DELETE | `/api/projects/{id}` | 删除项目（级联） |
| GET | `/api/projects/{id}/versions` | 项目版本列表 |
| POST | `/api/versions` | 保存新版本 |
| GET | `/api/versions/{id}` | 获取版本内容 |
| GET | `/api/characters` | 人物卡列表（`?project_id=`） |
| POST | `/api/characters` | 新建人物卡 |
| PUT | `/api/characters/{id}` | 更新人物卡 |
| DELETE | `/api/characters/{id}` | 删除人物卡 |
| GET | `/api/worldsettings` | 世界观列表（`?project_id=`） |
| POST | `/api/worldsettings` | 新建世界观 |
| PUT | `/api/worldsettings/{id}` | 更新世界观 |
| DELETE | `/api/worldsettings/{id}` | 删除世界观 |
| POST | `/api/materials/upload` | 素材上传解析（multipart） |
| GET | `/api/materials` | 素材列表（`?project_id=`） |
| DELETE | `/api/materials/{id}` | 删除素材 |
| GET | `/api/templates` | 模板列表（`?category=`） |
| POST | `/api/templates` | 新建模板 |
| PUT | `/api/templates/{id}` | 更新模板（系统内置不可改） |
| DELETE | `/api/templates/{id}` | 删除模板（系统内置不可删） |
| GET | `/api/models` | 模型列表（api_key 脱敏） |
| POST | `/api/models` | 新建模型 |
| PUT | `/api/models/{id}` | 更新模型 |
| DELETE | `/api/models/{id}` | 删除模型 |
| GET | `/api/roles/{role}/models` | 角色绑定模型优先级 |
| PUT | `/api/roles/{role}/models` | 配置角色绑定 |
| GET | `/api/logs` | 模型调用日志（`?project_id=&limit=&offset=`） |
| GET | `/api/usage` | 当日/近7日用量+限额阈值 |
| GET | `/api/configs` | 后台配置列表 |
| PUT | `/api/configs/{key}` | 修改单个配置 |

---

## 三、核心接口定义

### 3.1 创作生成 `POST /api/generate`（SSE 流式）

**请求体**

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `project_id` | string | 否 | 项目 ID，用于自动加载设定/前文 |
| `user_demand` | string | 二选一 | 用户创作需求 |
| `selected_text` | string | 二选一 | 编辑器选中文字，无则空串 |
| `world_setting` | string | 否 | 世界观文本，空则按 project_id 加载 |
| `character_setting` | string | 否 | 人物卡文本，空则按 project_id 加载 |
| `history_content` | string | 否 | 前文，空则取最新版本 |
| `material_text` | string | 否 | 素材文本 |
| `target_word` | int | 否 | 目标字数 |
| `run_mode` | string | 否 | 运行模式，默认 `auto` |
| `model_name` | string | 否 | 手动模式指定模型名 |

> 校验：`user_demand` 与 `selected_text` 至少一个非空。

**响应**：`Content-Type: text/event-stream`，逐条推送 SSE 事件（见第五章）。客户端断开连接即取消生成。

### 3.2 逻辑自检 `POST /api/verify`

**请求体**

```json
{ "content": "<待校验正文>", "world_setting": "", "character_setting": "" }
```

**响应 `200`**

```json
{
  "passed": true,
  "review": "<校验官原始输出>",
  "issues": ["问题1", "问题2"],
  "suggestions": "<修改建议全文>",
  "model": "<实际调用的模型名>"
}
```

### 3.3 项目

**`POST /api/projects`**

```json
// 请求
{ "name": "风起西陵", "type": "玄幻" }
// 响应 201
{ "item": { "id":"...", "name":"...", "type":"...", "created_at":"...", "updated_at":"..." } }
```

**`GET /api/projects/{id}` 响应**

```json
{
  "item": { "id":"...", "name":"...", "type":"...", "created_at":"...", "updated_at":"..." },
  "latest_version": { "id":"...", "project_id":"...", "title":"...", "content":"...", "version":3, "created_at":"..." } | null,
  "character_count": 2,
  "worldsetting_count": 1,
  "material_count": 0
}
```

**`PUT /api/projects/{id}`** — `name`、`type` 为可选指针字段（`null` 表示不更新）

**`DELETE /api/projects/{id}`** — `204`，级联删除子表

### 3.4 稿件版本

**`POST /api/versions`**

```json
// 请求
{ "project_id":"...", "title":"初稿", "content":"..." }
// 响应 201 — version 自动递增
{ "item": { "id":"...", "project_id":"...", "title":"...", "content":"...", "version":1, "created_at":"..." } }
```

**`GET /api/projects/{id}/versions`** — 按 version 倒序

```json
{ "items": [ { "id":"...", "version":3, "title":"...", "content":"...", "created_at":"..." }, ... ] }
```

### 3.5 人物卡

**`POST /api/characters`**

```json
{ "project_id":"...", "name":"林风", "description":"...", "avatar_url":"" }
```

**`PUT /api/characters/{id}`** — `name` / `description` / `avatar_url` 均为可选指针字段

**实体结构**

```json
{ "id":"", "project_id":"", "name":"", "description":"", "avatar_url":"", "created_at":"", "updated_at":"" }
```

### 3.6 世界观

**`POST /api/worldsettings`**

```json
{ "project_id":"...", "title":"九州大陆", "content":"..." }
```

**`PUT /api/worldsettings/{id}`** — `title` / `content` 可选指针字段

**实体结构**

```json
{ "id":"", "project_id":"", "title":"", "content":"", "created_at":"", "updated_at":"" }
```

### 3.7 素材

**`POST /api/materials/upload`**（`multipart/form-data`）

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `project_id` | string | 是 | 项目 ID |
| `file` | file | 是 | 支持 `.txt` / `.md` / `.docx` |
| `name` | string | 否 | 留空使用文件名 |

**响应 `201`**

```json
{ "item": { "id":"", "project_id":"", "name":"", "content":"<解析后纯文本>", "file_type":"txt", "created_at":"" }, "text": "<解析后纯文本>" }
```

### 3.8 模板

**`GET /api/templates?category=`** — 系统内置置顶，其次按创建时间倒序

**`POST /api/templates`**

```json
{ "name":"...", "category":"...", "content":"..." }
```

> `is_system` 恒为 `false`（系统内置仅由启动时种子写入）。

**`PUT /api/templates/{id}`** — 仅 `is_system=0` 可改；`name`/`category`/`content` 可选指针

**`DELETE /api/templates/{id}`** — 仅 `is_system=0` 可删；系统内置返回 `403`

**实体结构**

```json
{ "id":"", "name":"", "category":"", "content":"", "is_system":true, "created_at":"" }
```

### 3.9 模型配置

**`GET /api/models`** — `api_key` 脱敏（前3后4，中间 `****`）

**实体结构**

```json
{ "id":"", "name":"", "vendor":"", "api_endpoint":"", "api_key":"sk-****abcd", "status":"active", "daily_limit":0, "created_at":"" }
```

`status` 枚举：`active` / `disabled`

**`POST /api/models`** — `name`、`api_key` 必填

### 3.10 角色绑定

**`GET /api/roles/{role}/models`** — `role` ∈ `{thinker, worker, verifier, helper}`

```json
{
  "item": {
    "role":"worker",
    "models":[ {"model_id":"...", "name":"...", "vendor":"...", "priority":0} ]
  }
}
```

**`PUT /api/roles/{role}/models`**

```json
{ "model_ids": ["id1", "id2"] }
```

> 覆盖式设置，`priority` 按 `model_ids` 数组顺序递增；空数组返回 `400`。

### 3.11 用量统计 `GET /api/usage`

```json
{
  "today": { "calls": 12, "tokens": 34567 },
  "today_by_model": [ {"day":"2026-07-28","model_name":"deepseek-chat","calls":10,"tokens":30000} ],
  "week_by_model":  [ ... ],
  "limits": {
    "daily_call_limit": 500,
    "daily_token_limit": 2000000,
    "per_request_token_limit": 8000,
    "rate_limit_per_minute": 20,
    "max_concurrent": 5,
    "max_iterations": 3
  }
}
```

### 3.12 调用日志 `GET /api/logs`

参数：`?project_id=&limit=&offset=`

```json
{
  "items": [{
    "id":"", "project_id":"", "role":"worker", "model_name":"...",
    "prompt_tokens":0, "completion_tokens":0, "duration_ms":0,
    "status":"ok", "error_msg":"", "created_at":""
  }]
}
```

`status` 枚举：`ok` / `partial`（流中断但已有部分输出）/ `error`

### 3.13 后台配置

**`GET /api/configs`** → `{ "items": [{ "id":"", "key":"", "value":"", "description":"" }] }`

**`PUT /api/configs/{key}`**

```json
{ "value":"1000", "description":"单请求token上限" }
```

---

## 四、run_mode 枚举

| 值 | 模式名 | 触发流水线 | 执行链路 | model_name |
|---|---|---|---|---|
| `auto` | 智能协同（默认） | 自动判定 | 由调度中枢根据任务判定（文本<500字/局部改写→light；严谨/学术→strict；文笔/氛围→art；其余→standard） | 空 |
| `strict` | 严谨模式 | `strict` | Thinker 独立初稿 → Worker 轻度润色 → Verifier 高标准校验 | 空 |
| `art` | 文艺创作模式 | `art` | Thinker 极简框架 → Worker 高度自由创作 → Verifier 宽松审查 | 空 |
| `light` | 轻量化快速模式 | `light` | 直接调用 Helper，不启动多轮串联 | 空 |
| `manual` | 手动自选模型模式 | `manual` | 跳过流水线，直接调用 `model_name` 指定模型 | **必填** |

---

## 五、SSE 事件协议

### 5.1 传输格式

- `Content-Type: text/event-stream`
- `Cache-Control: no-cache`
- `Connection: keep-alive`
- 每条事件：`data: <JSON>\n\n`
- 客户端断开连接即触发 `ctx.Done()`，调度中枢终止生成

### 5.2 事件类型枚举

| type | 含义 | 时机 |
|---|---|---|
| `estimate` | Token 预估 | 请求受理后、流水线启动前 |
| `plan` | 流水线计划 | 解析出流水线后 |
| `stage` | 阶段进度 | 每个子任务开始时 |
| `token` | 正文增量分片 | 流式生成过程中（可多次） |
| `warning` | 降级/缺陷提示 | 模型降级、校验发现缺陷、迭代微调 |
| `error` | 错误 | 流程终止 |
| `done` | 完成 | 终稿产出 |

### 5.3 事件数据结构（`ProgressEvent`）

```json
{
  "type": "stage",
  "pipeline": "standard",
  "stage": "创作者 Worker 撰写正文中",
  "role": "worker",
  "model": "deepseek-chat",
  "text": "",
  "iteration": 1,
  "issues": [],
  "final_text": "",
  "tokens": 0,
  "degraded": false,
  "reset": false
}
```

| 字段 | 类型 | 出现于事件 | 说明 |
|---|---|---|---|
| `type` | string | 全部 | 事件类型枚举 |
| `pipeline` | string | plan, done | 流水线名 `standard`/`strict`/`art`/`light`/`manual` |
| `stage` | string | plan, stage, warning | 阶段描述文本 |
| `role` | string | stage, token | `thinker`/`worker`/`verifier`/`helper`/`manual` |
| `model` | string | stage, token | 当前调用的模型名 |
| `text` | string | token, warning, error | 增量文本 / 警告消息 / 错误消息 |
| `iteration` | int | stage, warning | 迭代轮次（微调循环） |
| `issues` | string[] | warning | 校验问题清单 |
| `final_text` | string | done | 终稿全文 |
| `tokens` | int | estimate | 预估 Token 数 |
| `degraded` | bool | warning | 是否发生降级 |
| `reset` | bool | token | `true` 表示清空已渲染文本（微调重写前通知前端重置） |

### 5.4 各事件字段取值

**`estimate`**

```json
{ "type":"estimate", "tokens":4200 }
```

**`plan`**

```json
{ "type":"plan", "pipeline":"standard", "stage":"标准通用创作流水线" }
```

**`stage`**（示例：3 阶段流水线）

```json
{ "type":"stage", "stage":"规划师 Thinker 正在搭建大纲", "role":"thinker" }
{ "type":"stage", "stage":"创作者 Worker 撰写正文中", "role":"worker" }
{ "type":"stage", "stage":"校验官 Verifier 第 1 轮审查", "role":"verifier", "iteration":1 }
{ "type":"stage", "stage":"创作者 Worker 根据修改意见微调中", "role":"worker", "iteration":1 }
```

**`token`**

```json
{ "type":"token", "text":"风起", "role":"worker", "model":"deepseek-chat" }
```

带 `reset`（微调重写前）：

```json
{ "type":"token", "text":"", "role":"worker", "reset":true }
```

> 前端收到 `reset:true` 应清空已渲染的流式文本，重新追加后续分片。

**`warning`**

```json
{ "type":"warning", "stage":"发现缺陷，回传创作者微调（第 1/3 轮）", "role":"verifier", "iteration":1, "issues":["人设OOC：...","时间线矛盾：..."], "degraded":false }
{ "type":"warning", "text":"主模型异常，已自动降级到备用模型", "degraded":true }
{ "type":"warning", "stage":"已达最大迭代轮次(3)，交付当前稿件并标注现存问题", "issues":["校验未完全通过，可能仍存在次要缺陷，请人工复核"] }
```

**`error`**

```json
{ "type":"error", "text":"「规划师Thinker」角色的全部模型调用均失败，请稍后重试或在后台检查模型配置" }
```

> 错误信息为友好中文，禁止暴露原始 HTTP 状态码/堆栈。

**`done`**

```json
{ "type":"done", "pipeline":"standard", "final_text":"<终稿全文>" }
```

### 5.5 标准流水线事件序列（示例）

```
estimate → plan → stage(thinker) → stage(worker) → token* →
stage(verifier) → [warning(issues) → token(reset) → stage(worker) → token*]* → done
```

`*` 表示可重复；方括号内为微调迭代循环（最多 `max_iterations` 轮）。

---

## 六、错误信息格式

### 6.1 HTTP 错误响应

统一为：

```json
{ "error": "<友好中文消息>" }
```

### 6.2 SSE 错误事件

```json
{ "type":"error", "text":"<友好中文消息>" }
```

### 6.3 常见错误场景

| 场景 | HTTP | 消息示例 |
|---|---|---|
| 请求体 JSON 解析失败 | 400 | `请求体解析失败: <detail>` |
| 必填字段缺失 | 400 | `name 不能为空` / `project_id 与 name 必填` / `缺少 project_id 查询参数` |
| `user_demand` 与 `selected_text` 同时为空 | 400 | `user_demand 与 selected_text 至少需提供一个` |
| 资源不存在 | 404 | `项目不存在` / `版本不存在` / `人物不存在` |
| 系统模板改删 | 403 | `系统内置模板不可删除` |
| 角色非法 | 400 | `非法角色，可选: thinker/worker/verifier/helper` |
| 限流/配额耗尽 | 429 | `今日调用额度已用完` / `请求过于频繁` |
| 无可用模型 | 503 | `「worker」无可用模型：<detail>` / `校验官全部模型调用失败，请稍后重试` |
| 模型全部失败（SSE） | — | `「<角色中文名>」角色的全部模型调用均失败，请稍后重试或在后台检查模型配置` |

### 6.4 友好错误转换规则

调度中枢对底层错误统一封装为 `friendlyErr`：仅保留角色中文名 + 通用提示，**不暴露原始 API 错误码、模型厂商返回体、堆栈信息**。降级时通过 `warning` 事件提示，不中断流程。

---

## 七、数据库核心实体

> 数据库：SQLite，DDL 见 `configs/schema.sql`，运行时由 `database.Open` 自动迁移（`CREATE TABLE IF NOT EXISTS`，幂等）。

### 7.1 实体关系

```
projects (1) ──< documents (N)          稿件版本
projects (1) ──< characters (N)         人物卡
projects (1) ──< world_settings (N)     世界观
projects (1) ──< materials (N)          素材
projects (1) ──< generation_logs (N)    调用日志
models (1) ──< role_models (N)          角色-模型绑定（优先级）
templates                              提示词模板（全局，无项目归属）
configs                                后台配置键值对（全局）
usage_daily                            每日用量统计（按模型）
```

外键均带 `ON DELETE CASCADE`（项目删除时级联清理子表）。

### 7.2 实体定义

#### `projects`

| 字段 | 类型 | 约束 | 说明 |
|---|---|---|---|
| `id` | TEXT | PK | UUID |
| `name` | TEXT | NOT NULL | 项目名 |
| `type` | TEXT | DEFAULT '' | 类型 |
| `created_at` | TEXT | DEFAULT datetime('now') | |
| `updated_at` | TEXT | DEFAULT datetime('now') | |

#### `documents`（稿件版本快照）

| 字段 | 类型 | 约束 | 说明 |
|---|---|---|---|
| `id` | TEXT | PK | |
| `project_id` | TEXT | NOT NULL, FK→projects | |
| `title` | TEXT | DEFAULT '' | 版本标题/备注 |
| `content` | TEXT | DEFAULT '' | 全文 |
| `version` | INTEGER | DEFAULT 1 | 版本号，新建时取 `MAX(version)+1` |
| `created_at` | TEXT | DEFAULT datetime('now') | |

索引：`idx_documents_project(project_id)`，按 version 倒序查询。

#### `characters`

| 字段 | 类型 | 约束 | 说明 |
|---|---|---|---|
| `id` | TEXT | PK | |
| `project_id` | TEXT | NOT NULL, FK | |
| `name` | TEXT | NOT NULL | 姓名 |
| `description` | TEXT | DEFAULT '' | 描述（前端可结构化打包） |
| `avatar_url` | TEXT | DEFAULT '' | 头像 |
| `created_at` | TEXT | | |
| `updated_at` | TEXT | | |

#### `world_settings`

| 字段 | 类型 | 约束 | 说明 |
|---|---|---|---|
| `id` | TEXT | PK | |
| `project_id` | TEXT | NOT NULL, FK | |
| `title` | TEXT | NOT NULL | 标题 |
| `content` | TEXT | DEFAULT '' | 内容 |
| `created_at` | TEXT | | |
| `updated_at` | TEXT | | |

#### `materials`

| 字段 | 类型 | 约束 | 说明 |
|---|---|---|---|
| `id` | TEXT | PK | |
| `project_id` | TEXT | NOT NULL, FK | |
| `name` | TEXT | NOT NULL | 文件名/自定义名 |
| `content` | TEXT | DEFAULT '' | 解析后纯文本 |
| `file_type` | TEXT | DEFAULT '' | txt / md / docx |
| `created_at` | TEXT | | |

#### `templates`

| 字段 | 类型 | 约束 | 说明 |
|---|---|---|---|
| `id` | TEXT | PK | |
| `name` | TEXT | NOT NULL | 模板名（系统内置按 name 去重幂等种子） |
| `category` | TEXT | DEFAULT '' | 分类 |
| `content` | TEXT | DEFAULT '' | 模板内容 |
| `is_system` | INTEGER | DEFAULT 0 | 1=系统内置（不可改删），0=用户自定义 |
| `created_at` | TEXT | | |

#### `models`

| 字段 | 类型 | 约束 | 说明 |
|---|---|---|---|
| `id` | TEXT | PK | |
| `name` | TEXT | NOT NULL, UNIQUE | 模型名（调度层按 name 查找） |
| `vendor` | TEXT | DEFAULT '' | 厂商 |
| `api_endpoint` | TEXT | DEFAULT '' | 接口地址 |
| `api_key` | TEXT | DEFAULT '' | 密钥（列表接口脱敏） |
| `status` | TEXT | DEFAULT 'active' | `active` / `disabled` |
| `daily_limit` | INTEGER | DEFAULT 0 | 单模型日上限，0=不限 |
| `created_at` | TEXT | | |

#### `role_models`（角色-模型绑定）

| 字段 | 类型 | 约束 | 说明 |
|---|---|---|---|
| `id` | TEXT | PK | |
| `role` | TEXT | NOT NULL | `thinker`/`worker`/`verifier`/`helper` |
| `model_id` | TEXT | NOT NULL, FK→models | |
| `priority` | INTEGER | DEFAULT 0 | 0=主模型，数字越大越靠后 |

约束：`UNIQUE(role, model_id)`。索引：`idx_role_models_role(role, priority)`。

#### `generation_logs`

| 字段 | 类型 | 约束 | 说明 |
|---|---|---|---|
| `id` | TEXT | PK | |
| `project_id` | TEXT | DEFAULT '' | |
| `role` | TEXT | DEFAULT '' | |
| `model_name` | TEXT | DEFAULT '' | |
| `prompt_tokens` | INTEGER | DEFAULT 0 | |
| `completion_tokens` | INTEGER | DEFAULT 0 | |
| `duration_ms` | INTEGER | DEFAULT 0 | |
| `status` | TEXT | DEFAULT 'ok' | `ok`/`partial`/`error` |
| `error_msg` | TEXT | DEFAULT '' | |
| `created_at` | TEXT | | |

索引：`idx_logs_created(created_at)`、`idx_logs_model(model_name, created_at)`。

#### `configs`（后台配置键值对）

| 字段 | 类型 | 约束 |
|---|---|---|
| `id` | TEXT | PK |
| `key` | TEXT | NOT NULL, UNIQUE |
| `value` | TEXT | DEFAULT '' |
| `description` | TEXT | DEFAULT '' |

#### `usage_daily`（每日用量统计）

| 字段 | 类型 | 约束 | 说明 |
|---|---|---|---|
| `day` | TEXT | NOT NULL | `YYYY-MM-DD` |
| `model_name` | TEXT | NOT NULL | |
| `calls` | INTEGER | DEFAULT 0 | |
| `tokens` | INTEGER | DEFAULT 0 | |

主键：`PRIMARY KEY(day, model_name)`。配额/降级依据。

---

## 八、调度角色定义（协议补充）

| 角色 | 标识 | 职责 | 系统提示词 |
|---|---|---|---|
| Thinker 规划师 | `thinker` | 大纲、需求拆解、剧情脉络、结构化框架 | `roles.ThinkerPrompt` |
| Worker 文笔创作者 | `worker` | 正文撰写、续写、润色、长文本叙事 | `roles.WorkerPrompt` |
| Verifier 校验官 | `verifier` | 全文审查、OOC/世界观/剧情漏洞检测、修改方案 | `roles.VerifierPrompt` |
| Helper 轻助手 | `helper` | 缩写/扩写/摘要/改写，禁止长篇正文 | `roles.HelperPrompt` |

每个角色绑定主模型 + 多级备用模型（`role_models` 表按 `priority` 排序）。子 Agent 失败（超时/429/报错）时自动切换下一级备用模型，全部失败则推送 `error` 事件并终止。

---

## 九、章节层级管理 API（volumes / chapters / chapter_versions）

### 9.1 卷（Volumes）

**`GET /api/projects/{id}/volumes`** — 卷列表

```json
{ "items": [{ "id":"", "project_id":"", "title":"第一卷", "sort_order":1, "created_at":"", "updated_at":"" }] }
```

**`POST /api/volumes`** — 新建卷
```json
// 请求
{ "project_id":"", "title":"第一卷", "sort_order":0 }
// 响应 201
{ "item": {...} }
```

**`PUT /api/volumes/{id}`** — 更新卷（`title` / `sort_order` 可选指针）

**`DELETE /api/volumes/{id}`** — 删除卷（204）

**`POST /api/volumes/reorder`** — 卷排序
```json
{ "items": [{ "id":"", "sort_order":1 }] }
```

### 9.2 章节（Chapters）

**`GET /api/chapters?project_id=&volume_id=`** — 章节列表
```json
{ "items": [{
  "id":"", "project_id":"", "volume_id":"", "title":"第一章",
  "content":"", "word_count":0, "sort_order":1,
  "tags":"", "synopsis":"", "volume_title":"",
  "created_at":"", "updated_at":""
}]}
```

**`POST /api/chapters`** — 新建章节
```json
{ "project_id":"", "volume_id":"", "title":"", "content":"" }
```

**`GET /api/chapters/{id}`** — 章节详情

**`PUT /api/chapters/{id}`** — 更新（`title` / `content` / `volume_id` / `tags` / `synopsis` 可选指针）

**`DELETE /api/chapters/{id}`** — 删除（204）

**`POST /api/chapters/{id}/copy`** — 复制章节 → 201

**`POST /api/chapters/reorder`** — 章节排序
```json
{ "items": [{ "id":"", "sort_order":1 }] }
```

### 9.3 章节操作

**`POST /api/chapters/merge`** — 合并章节（同卷连续）
```json
{ "chapter_ids": ["id1","id2"], "title":"合并后标题" }
```

**`POST /api/chapters/{id}/split`** — 拆分章节（光标位置）
```json
{ "cursor_pos": 500 }
```

**`POST /api/chapters/import`** — 导入章节（覆盖式）
```json
{ "project_id":"", "volumes":[], "chapters":[{...}] }
```

**`GET /api/chapters/export?project_id=`** — 导出全部章节 JSON

**`POST /api/chapters/split`** — 智能分割文档为章节
```json
{ "project_id":"", "content":"", "split_by":"auto" }
// split_by: "auto" | "## " | "### " | "第.*[章回节卷]" | "custom分隔符"
```

### 9.4 章节版本（Chapter Versions）

**`GET /api/chapters/{id}/versions`** — 章节版本列表
```json
{ "items": [{ "id":"", "chapter_id":"", "title":"", "content":"", "version":1, "created_at":"" }] }
```

**`POST /api/chapters/{id}/versions`** — 保存章节版本
```json
{ "title":"", "content":"" }
```

**`GET /api/chapters/versions/{id}`** — 获取版本内容

**`GET /api/projects/{id}/stats`** — 项目统计
```json
{ "item": { "total_chapters":10, "total_words":50000, "total_chars":80000, "volumes":[...] } }
```

### 9.5 GenerateRequest 扩展字段

| 字段 | 类型 | 说明 |
|------|------|------|
| `chapter_id` | string | 关联章节ID |
| `model_config_id` | string | 自定义模型配置ID |
| `cursor_position` | int | 光标位置（续写起点）|
| `no_rewrite` | bool | 禁止改写已有前文 |
| `context_scope` | string | `current`/`withSummary` |
| `previous_summaries` | string | 前面章节摘要文本 |
