# AI辅助写作助手 标准化接口协议文档

> 版本：v1.0 ｜ Base URL：`http://localhost:8081` ｜ 编码：UTF-8
> 本文档定义前后端对接的全部 API 路由、请求/响应结构、SSE 事件协议、错误格式与核心数据实体。前端按本协议对接，禁止依赖未定义字段。

---

## 0. 通用约定

### 0.1 请求头
| Header | 值 | 说明 |
|---|---|---|
| `Content-Type` | `application/json; charset=utf-8` | 除素材上传外所有接口 |
| `Accept` | `text/event-stream` | 仅 `/api/generate` |
| `Content-Type` | `multipart/form-data` | 仅 `/api/materials/upload` |

### 0.2 统一响应包装
- **单对象**：`{ "item": <T> }`
- **列表**：`{ "items": [ <T>, ... ] }`
- **操作确认**：`{ "status": "ok" }`
- **错误**：见 §5

### 0.3 ID 与时间
- 所有主键 `id` 为 **UUID 字符串**（`text`），由后端生成。
- 时间字段（`created_at` / `updated_at`）为 RFC3339 字符串，如 `2026-07-28T23:03:20+08:00`。

### 0.4 HTTP 状态码
| 码 | 含义 |
|---|---|
| 200 | 成功（GET/PUT） |
| 201 | 创建成功（POST） |
| 204 | 删除成功（无响应体） |
| 400 | 请求参数错误 |
| 403 | 禁止操作（如删除系统内置模板） |
| 404 | 资源不存在 |
| 429 | 限流/额度用尽 |
| 500 | 服务器内部错误 |
| 503 | 模型不可用 |

---

## 1. API 路由总表

| # | 方法 | 路径 | 说明 | 鉴权 |
|---|---|---|---|---|
| 1 | POST | `/api/generate` | 创作生成（SSE 流式） | — |
| 2 | POST | `/api/verify` | 独立逻辑自检 | — |
| 3 | GET | `/api/projects` | 项目列表 | — |
| 4 | POST | `/api/projects` | 新建项目 | — |
| 5 | GET | `/api/projects/{id}` | 项目详情（含资源计数+最新版本） | — |
| 6 | PUT | `/api/projects/{id}` | 更新项目 | — |
| 7 | DELETE | `/api/projects/{id}` | 删除项目（级联） | — |
| 8 | GET | `/api/projects/{id}/versions` | 项目版本列表 | — |
| 9 | POST | `/api/versions` | 保存新版本（version 自增） | — |
| 10 | GET | `/api/versions/{id}` | 获取版本内容 | — |
| 11 | GET | `/api/characters?project_id=` | 人物卡列表 | — |
| 12 | POST | `/api/characters` | 新建人物卡 | — |
| 13 | PUT | `/api/characters/{id}` | 更新人物卡 | — |
| 14 | DELETE | `/api/characters/{id}` | 删除人物卡 | — |
| 15 | GET | `/api/worldsettings?project_id=` | 世界观列表 | — |
| 16 | POST | `/api/worldsettings` | 新建世界观 | — |
| 17 | PUT | `/api/worldsettings/{id}` | 更新世界观 | — |
| 18 | DELETE | `/api/worldsettings/{id}` | 删除世界观 | — |
| 19 | POST | `/api/materials/upload` | 素材上传解析（txt/md/docx） | — |
| 20 | GET | `/api/materials?project_id=` | 素材列表 | — |
| 21 | DELETE | `/api/materials/{id}` | 删除素材 | — |
| 22 | GET | `/api/templates?category=` | 模板列表（系统+自定义） | — |
| 23 | POST | `/api/templates` | 新建自定义模板 | — |
| 24 | PUT | `/api/templates/{id}` | 更新模板（系统内置不可改） | — |
| 25 | DELETE | `/api/templates/{id}` | 删除模板（系统内置不可删） | — |
| 26 | GET | `/api/models` | 模型列表（api_key 脱敏） | — |
| 27 | POST | `/api/models` | 新增模型 | — |
| 28 | PUT | `/api/models/{id}` | 更新模型配置 | — |
| 29 | DELETE | `/api/models/{id}` | 删除模型 | — |
| 30 | GET | `/api/roles/{role}/models` | 角色绑定的模型优先级 | — |
| 31 | PUT | `/api/roles/{role}/models` | 配置角色模型绑定 | — |
| 32 | GET | `/api/logs?project_id=&limit=&offset=` | 调用日志 | — |
| 33 | GET | `/api/usage` | 用量统计+限额阈值 | — |
| 34 | GET | `/api/configs` | 全部后台配置 | — |
| 35 | PUT | `/api/configs/{key}` | 修改单个配置 | — |

---

## 2. 创作生成接口（SSE）

### 2.1 `POST /api/generate`

**请求体** `GenerateRequest`：

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `project_id` | string | 否 | 项目 ID；填写则后端自动加载缺失的设定/前文 |
| `user_demand` | string | 是* | 用户创作需求（与 `selected_text` 至少有一个非空） |
| `selected_text` | string | 否 | 编辑器选中文字，无选中为空串 |
| `world_setting` | string | 否 | 世界观文本（空则按 project_id 从库加载） |
| `character_setting` | string | 否 | 人物卡文本（空则从库加载） |
| `history_content` | string | 否 | 前文内容（空则取项目最新版本） |
| `material_text` | string | 否 | 参考素材文本 |
| `target_word` | int | 否 | 目标字数；>4000 后端自动分段 |
| `run_mode` | enum | 否 | 运行模式，见 §3，缺省 `auto` |
| `model_name` | string | 否 | 仅 `manual` 模式必填，其余为空 |

**响应**：`Content-Type: text/event-stream`，SSE 协议见 §4。

---

## 3. `run_mode` 枚举

| 值 | 名称 | 触发流水线 | 说明 |
|---|---|---|---|
| `auto` | 智能协同模式（默认） | 后端自动判定 | 按文本长度/关键词自动选择下列流水线 |
| `strict` | 严谨模式 | 严谨流水线 | Thinker 独立初稿 → Worker 轻度润色 → Verifier 高标准校验 |
| `art` | 文艺创作模式 | 文艺流水线 | Thinker 极简框架 → Worker 高度自由创作 → Verifier 宽松审查 |
| `light` | 轻量化快速模式 | 轻量流水线 | 直接调用 Helper，不启动多轮串联；输入强制 ≤500 字 |
| `manual` | 手动自选模型 | 手动直调 | 跳过流水线，直接调用 `model_name` 指定模型 |

**后端自动判定规则**（`run_mode=auto` 时）：
1. `selected_text` 非空且 <500 字 → `light`
2. 需求含「严谨/正式/学术/公文/论文/报告/纪实」→ `strict`
3. 需求含「文笔/氛围/文学性/散文/诗意」→ `art`
4. 其余 → 标准（Thinker→Worker→Verifier）

**角色枚举** `role`（用于 `/api/roles/{role}/models` 路径参数与 SSE 事件）：

| 值 | 角色 | 图标建议 |
|---|---|---|
| `thinker` | Thinker 规划师 | 🧭 |
| `worker` | Worker 文笔创作者 | ✍️ |
| `verifier` | Verifier 校验官 | 🔍 |
| `helper` | Helper 轻助手 | ⚡ |

---

## 4. SSE 事件协议

### 4.1 传输格式
每条事件为一行：`data: <JSON>\n\n`（标准 Server-Sent Events）。
前端通过 `fetch` + `ReadableStream` 读取，按 `\n` 分割，取 `data: ` 前缀行 JSON.parse。

### 4.2 事件统一结构 `ProgressEvent`

| 字段 | 类型 | 出现场景 | 说明 |
|---|---|---|---|
| `type` | string | 必有 | 事件类型，见 4.3 |
| `pipeline` | string | plan/done | 流水线名：`standard`/`strict`/`art`/`light`/`manual` |
| `stage` | string | stage/warning | 阶段描述文本 |
| `role` | string | stage/token | 当前角色（`thinker`/`worker`/`verifier`/`helper`/`manual`） |
| `model` | string | stage | 当前使用的模型名 |
| `text` | string | token/warning/error | 增量正文片段 或 提示消息 |
| `iteration` | int | stage/warning | 微调迭代轮次（从 1 起） |
| `issues` | string[] | warning | 校验问题清单 |
| `final_text` | string | done | 最终完整稿件 |
| `tokens` | int | estimate | 预估 token 消耗 |
| `degraded` | bool | warning | 是否发生模型降级 |
| `reset` | bool | token | `true` 表示前端需先清空已渲染正文再追加后续 token（微调重写前触发） |

### 4.3 事件类型 `type` 枚举

| type | 说明 | 前端处理 |
|---|---|---|
| `estimate` | 生成前 token 预估 | 读取 `tokens`，可与用户确认后发起 |
| `plan` | 流水线计划 | 读取 `pipeline`/`stage`，初始化进度卡片 |
| `stage` | 阶段进度 | 更新进度条文本、角色图标；按 `role` 渲染【n/3】 |
| `token` | 正文增量分片 | 追加 `text` 到编辑器；若 `reset=true` 先清空 |
| `warning` | 降级/校验缺陷提示 | 展示 `text` 或 `issues` 列表 |
| `error` | 错误 | 友好弹窗展示 `text`，终止生成 |
| `done` | 生成完成 | 进度条 100%，用 `final_text` 落盘，保存版本 |

### 4.4 典型事件序列（标准流水线）

```
estimate  → {type:"estimate", tokens:282}
plan      → {type:"plan", pipeline:"standard", stage:"标准通用创作流水线"}
stage     → {type:"stage", role:"thinker", stage:"规划师 Thinker 正在搭建大纲"}
stage     → {type:"stage", role:"thinker", model:"deepseek-chat", stage:"大纲已完成"}
stage     → {type:"stage", role:"worker", stage:"创作者 Worker 撰写正文中"}
token     → {type:"token", role:"worker", text:"林动走进"}      (多次)
token     → {type:"token", role:"worker", text:"山洞..."}
stage     → {type:"stage", role:"verifier", iteration:1, stage:"校验官 Verifier 第 1 轮审查"}
warning   → {type:"warning", degraded:true, text:"校验官主模型异常，已降级到备用模型"}
[若校验未通过：]
token     → {type:"token", role:"worker", reset:true, text:""}  (清空信号)
stage     → {type:"stage", role:"worker", iteration:1, stage:"创作者 Worker 根据修改意见微调中"}
token     → {type:"token", role:"worker", text:"..."}           (重写正文)
done      → {type:"done", pipeline:"standard", final_text:"<完整终稿>"}
```

### 4.5 中断
客户端断开连接（`AbortController.abort()`）即取消生成，后端释放资源。

---

## 5. 错误信息格式

### 5.1 HTTP 错误响应体
所有 4xx/5xx 返回 JSON：
```json
{ "error": "友好中文提示信息" }
```
> 后端**不返回原始 API 错误码/堆栈**；模型调用失败经降级后仍全部失败时返回友好提示，如「校验官角色的全部模型调用均失败，请稍后重试或在后台检查模型配置」。

### 5.2 SSE 内错误
`error` 事件：`{ "type": "error", "text": "友好中文提示" }`，前端弹窗展示后关闭流。

### 5.3 常见错误场景
| 场景 | HTTP | error 文案示例 |
|---|---|---|
| 缺少必填字段 | 400 | `project_id 与 name 必填` |
| 资源不存在 | 404 | `项目不存在` |
| 删除系统模板 | 403 | `系统内置模板不可删除` |
| 限流 | 429 | `请求过于频繁，请稍后再试` |
| 并发满 | 429 | `当前并发请求过多，请稍后再试` |
| 额度用尽 | 429 | `今日调用额度已用完，请明日再试` |
| 模型全失败 | 503 | `「校验官Verifier」角色的全部模型调用均失败...` |

---

## 6. 核心数据实体

### 6.1 Project（项目）
```json
{
  "id": "ad69cc71-e44b-4741-aa90-2ac7ae14b109",
  "name": "测试小说",
  "type": "玄幻",
  "created_at": "2026-07-28T23:03:20+08:00",
  "updated_at": "2026-07-28T23:03:35+08:00"
}
```
`GET /api/projects/{id}` 额外返回：
```json
{
  "item": { /* Project */ },
  "latest_version": { /* Version|null */ },
  "character_count": 1,
  "worldsetting_count": 1,
  "material_count": 0
}
```

### 6.2 Version（稿件版本，documents 表）
```json
{
  "id": "b1483d4f-...",
  "project_id": "ad69cc71-...",
  "title": "初稿",
  "content": "正文内容...",
  "version": 1,
  "created_at": "2026-07-28T23:03:35+08:00"
}
```
- `POST /api/versions` 请求体：`{ "project_id", "title", "content" }`，`version` 后端自增。

### 6.3 Character（人物卡）
```json
{
  "id": "3401e5a7-...",
  "project_id": "ad69cc71-...",
  "name": "林动",
  "description": "少年，性格坚毅",
  "avatar_url": "",
  "created_at": "...",
  "updated_at": "..."
}
```
- `POST`/`PUT` 请求体：`{ "project_id", "name", "description", "avatar_url" }`（PUT 为可选字段指针）。
- 列表/创建需 `project_id`（列表用查询参数，创建用请求体）。

> 前端人物卡编辑器字段（姓名/外貌/性格/背景/行为底线/备注）建议拼接为 `description` 单字段存储，或扩展后端字段。当前后端实体为 `name` + `description` + `avatar_url`。

### 6.4 WorldSetting（世界观设定）
```json
{
  "id": "ff516a3e-...",
  "project_id": "ad69cc71-...",
  "title": "武学体系",
  "content": "凝气境、造化境",
  "created_at": "...",
  "updated_at": "..."
}
```
- `POST`/`PUT` 请求体：`{ "project_id", "title", "content" }`。

### 6.5 Material（素材文档）
```json
{
  "id": "...",
  "project_id": "ad69cc71-...",
  "name": "参考资料.txt",
  "content": "解析后的纯文本...",
  "file_type": "txt",
  "created_at": "..."
}
```
- 上传 `POST /api/materials/upload`（`multipart/form-data`）字段：`project_id`（必填）、`name`（可选）、`file`（文件）。支持 `.txt`/`.md`/`.docx`，后端解析为纯文本。
- 响应：`{ "item": <Material>, "text": "解析文本" }`。

### 6.6 Template（提示词模板）
```json
{
  "id": "7929592e-...",
  "name": "标准小说续写",
  "category": "novel",
  "content": "模板正文...",
  "is_system": true,
  "created_at": "..."
}
```
- `is_system=true` 为系统内置（后端种子），**不可修改/删除**（PUT/DELETE 返回 403/404）。
- `POST` 请求体：`{ "name", "category", "content" }`（自动 `is_system=false`）。
- 列表支持 `?category=` 过滤，按 `is_system DESC, created_at DESC` 排序。

### 6.7 Model（模型配置）
```json
{
  "id": "deepseek-chat",
  "name": "deepseek-chat",
  "vendor": "DeepSeek",
  "api_endpoint": "https://api.deepseek.com",
  "api_key": "sk-****e744",
  "status": "active",
  "daily_limit": 0,
  "created_at": "..."
}
```
- `api_key` 在列表/详情接口中**脱敏**（仅前3后4），创建/更新时传明文。
- `status`：`active` / `disabled`。`daily_limit=0` 表示不限。
- `POST` 请求体：`{ "name", "vendor", "api_endpoint", "api_key", "status", "daily_limit" }`。
- `PUT` 全字段可选（指针更新）。

### 6.8 RoleModels（角色-模型绑定）
`GET /api/roles/{role}/models` 响应：
```json
{
  "item": {
    "role": "worker",
    "models": [
      { "model_id": "deepseek-chat", "name": "deepseek-chat", "vendor": "DeepSeek", "priority": 0 },
      { "model_id": "moonshot-v1-32k", "name": "moonshot-v1-32k", "vendor": "Kimi", "priority": 1 }
    ]
  }
}
```
`PUT /api/roles/{role}/models` 请求体：
```json
{ "model_ids": ["deepseek-chat", "moonshot-v1-32k"] }
```
- `priority` 按数组顺序递增（0=主模型，数字越大越靠后=备用）。
- 覆盖式更新，`role` 取值：`thinker`/`worker`/`verifier`/`helper`。

### 6.9 GenerationLog（调用日志）
```json
{
  "id": "...",
  "project_id": "ad69cc71-...",
  "role": "verifier",
  "model_name": "deepseek-chat",
  "prompt_tokens": 839,
  "completion_tokens": 128,
  "duration_ms": 2582,
  "status": "ok",
  "error_msg": "",
  "created_at": "..."
}
```
- `status`：`ok` / `partial`（中途断流接受部分）/ `error`。
- `GET /api/logs?project_id=&limit=&offset=`（limit 默认 50，上限 200）。

### 6.10 Config（后台配置/限额）
```json
{ "key": "daily_call_limit", "value": "500", "description": "全局每日总调用次数上限" }
```
`PUT /api/configs/{key}` 请求体：`{ "value", "description" }`。

**限额键值表**：

| key | 默认 | 说明 |
|---|---|---|
| `daily_call_limit` | 500 | 全局每日总调用次数上限 |
| `daily_token_limit` | 2000000 | 全局每日总 token 上限 |
| `per_request_token_limit` | 8000 | 单次请求 token 上限 |
| `light_input_char_limit` | 500 | 轻量模式输入字符上限 |
| `max_iterations` | 3 | 流水线最大迭代轮次 |
| `rate_limit_per_minute` | 20 | 单 IP 每分钟请求数 |
| `max_concurrent` | 5 | 并发请求数上限 |
| `warn_ratio` | 0.8 | 单模型当日消耗预警比例 |

---

## 7. 运维统计接口

### 7.1 `GET /api/usage`
响应：
```json
{
  "today": { "calls": 5, "tokens": 4059 },
  "today_by_model": [ { "model_name": "deepseek-chat", "calls": 5, "tokens": 4059 } ],
  "week_by_model":  [ { "model_name": "deepseek-chat", "calls": 5, "tokens": 4059 } ],
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
> 前端顶部额度进度条用 `today.calls / limits.daily_call_limit` 与 `today.tokens / limits.daily_token_limit`。

### 7.2 独立逻辑自检 `POST /api/verify`
请求体：
```json
{
  "content": "待校验正文",
  "world_setting": "可选",
  "character_setting": "可选"
}
```
响应：
```json
{
  "passed": false,
  "review": "【校验未通过】\n1. ...",
  "issues": ["问题1", "问题2"],
  "suggestions": "修改建议全文",
  "model": "deepseek-chat"
}
```
- `passed=true` 表示校验通过（review 含「校验通过」）。
- 该接口同样消耗配额并记录日志。

---

## 8. 数据库核心实体关系（ER 概览）

```
projects  1───*  documents (versions)
projects  1───*  characters
projects  1───*  world_settings
projects  1───*  materials

models  1───*  role_models  (role: thinker/worker/verifier/helper)
templates        (独立，is_system 区分)
generation_logs  (按 project_id 关联，可空)
configs          (key-value)
usage_daily      (day + model_name 复合主键)
```

**完整 DDL** 见 `configs/schema.sql`（11 张表：projects / documents / characters / world_settings / materials / templates / models / role_models / generation_logs / configs / usage_daily）。

---

## 9. 前端对接清单（Checklist）

- [ ] 请求模块统一封装 `fetchJSON`/`fetchSSE`，基础 URL 可配置
- [ ] `POST /api/generate` 用 `fetch` + `ReadableStream` 解析 `data: ` 行，按 `type` 分发
- [ ] `token` 事件追加正文，`reset=true` 先清空，`done` 用 `final_text` 落盘 + 调 `POST /api/versions`
- [ ] `stage` 事件按 `role` 渲染【n/3】进度（thinker→worker→verifier）
- [ ] `warning.degraded=true` 时提示「已降级到备用模型」
- [ ] `error` 事件友好弹窗，禁显原始码
- [ ] 模式选择器 5 选项映射 `run_mode`，`manual` 额外传 `model_name`（`GET /api/models` 拉取）
- [ ] 右侧勾选人物卡/世界观/素材后，拼接文本进 `world_setting`/`character_setting`/`material_text`
- [ ] 编辑器全文进 `history_content`，选中文字进 `selected_text`
- [ ] 顶部额度条读 `GET /api/usage`，达限置灰生成按钮
- [ ] 轻量模式前端校验：`selected_text` >500 字提示切换模式
- [ ] `POST /api/verify` 结果展示问题清单（`issues`）
- [ ] 模板面板读 `GET /api/templates`，点击填充指令框
- [ ] `Ctrl+Enter` 发送、`Ctrl+S` 保存、`Esc` 终止（AbortController）
