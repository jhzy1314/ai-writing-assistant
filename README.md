# AI辅助写作助手 后端服务

基于 **多模型角色调度架构** 的 AI 写作协同系统后端。通过「职能角色 + 调度中枢 + 标准流水线」实现大纲规划、正文创作、全文校验、轻量处理的多模型协同，并内置成本管控、限流降级、流式输出。

> 前端已单独开发，本仓库仅实现后端业务逻辑、API 接口、Agent 调度、模型适配层、数据持久化。双击 `run.bat` 即可启动。

## 技术栈

- **语言**: Go 1.26
- **Agent/模型框架**: [CloudWeGo Eino](https://github.com/cloudwego/eino)（ChatModel / 流式能力作为模型适配层底座）
- **数据库**: SQLite（`modernc.org/sqlite` 纯 Go 驱动，零配置，`run.bat` 双击即用）
- **路由**: chi v5
- **配置**: Viper（YAML，API 密钥由配置文件管理，禁止硬编码）

## 核心架构（四层分离）

```
┌─ 接口层  internal/interfaces/api    RESTful + SSE 流式输出
├─ 调度引擎层 internal/domain/pipeline  调度中枢 Agent + 4 标准流水线
├─ 模型适配层 internal/infrastructure/llm  统一抽象 + DeepSeek/Kimi 适配器 + 角色注册中心
├─ 数据访问层 internal/infrastructure/database  SQLite 仓库（DDL 见 configs/schema.sql）
└─ 角色层   internal/domain/roles     4 职能角色 + 硬编码 Prompt 系统
```

### 模型角色池（模型与角色解耦，可插拔）

| 角色 | 职责 | 系统提示词 |
|------|------|-----------|
| **Thinker** 规划师 | 大纲、需求拆解、剧情脉络、结构化框架 | `roles.ThinkerPrompt` |
| **Worker** 文笔创作者 | 正文撰写、续写、润色、长文本叙事 | `roles.WorkerPrompt` |
| **Verifier** 校验官 | 全文审查、OOC/世界观/剧情漏洞检测、修改方案 | `roles.VerifierPrompt` |
| **Helper** 轻助手 | 缩写/扩写/摘要/改写，禁止长篇正文 | `roles.HelperPrompt` |

每个角色绑定**主模型 + 多级备用模型**（`role_models` 表，按 priority 排序），后台可增删切换，无需改代码。

### 调度中枢 Agent（`pipeline.Dispatcher`）

接收请求 → 自动识别任务类型 → 匹配流水线 → 拆分任务 → 有序调用子 Agent → 管控迭代循环 → 异常降级 → 汇总终稿。**禁止直接生成正文**，只做调度与流程控制。

### 4 套标准流水线（自动匹配）

| 流水线 | 触发条件 | 执行链路 |
|--------|---------|---------|
| **标准通用**（默认） | 小说/故事创作无标注 | Thinker 大纲 → Worker 撰写 → Verifier 校验 → 缺陷回传 Worker 微调（最大 3 轮） |
| **严谨模式** | 严谨/正式/学术/公文 | Thinker 独立初稿 → Worker 轻度润色 → Verifier 高标准逻辑校验 |
| **文艺创作** | 文笔/氛围/文学性 | Thinker 极简框架 → Worker 高度自由创作 → Verifier 宽松审查（仅拦重大 BUG） |
| **轻量化快速** | 文本<500字 / 局部改写 | 直接调用 Helper，不启动多轮串联 |

## 项目结构

```
ai-novel-main/
├── cmd/server/main.go              # 入口：装配各层、启动 HTTP
├── configs/
│   ├── config.yaml                 # 服务/SQLite/限额/模型池/角色绑定（含 API Key）
│   └── schema.sql                  # 数据库 DDL（交付参考，运行时自动迁移）
├── internal/
│   ├── domain/
│   │   ├── roles/                  # 硬编码 Prompt + 4 角色 Agent
│   │   │   ├── prompts.go          # 规格第五章全部提示词
│   │   │   └── agent.go            # RoleAgent（组装系统提示词 + 调用适配器）
│   │   └── pipeline/               # 调度中枢 + 流水线
│   │       ├── types.go            # 请求/事件/上下文类型
│   │       ├── dispatcher.go       # Dispatcher.Run + callRole/callRoleStream（降级+日志）
│   │       ├── execute.go          # 4 流水线执行 + 微调迭代循环
│   │       ├── context.go          # 上下文组装 + 任务自动判定
│   │       └── prompt_build.go     # 各子任务用户提示词构造
│   ├── infrastructure/
│   │   ├── config/config.go        # Viper 加载 config.yaml
│   │   ├── database/               # SQLite 数据访问层（store + 各实体仓库）
│   │   ├── llm/                    # 模型适配层
│   │   │   ├── adapter.go          # ModelAdapter 统一接口 + token 估算
│   │   │   ├── openai.go           # OpenAI 兼容适配器（DeepSeek/Kimi 复用）
│   │   │   └── registry.go         # 角色→有序模型列表 注册中心（带缓存）
│   │   └── quota/limiter.go        # 配额/限流/并发/降级预算
│   └── interfaces/api/             # RESTful + SSE 接口
│       ├── server.go               # 路由注册 + 通用助手
│       ├── generate.go             # POST /api/generate (SSE) + /api/verify
│       ├── projects.go             # 项目 + 版本
│       ├── resources.go            # 人物卡 + 世界观 + 素材上传(docx/txt)
│       ├── templates.go            # 模板 CRUD
│       ├── models.go               # 模型 + 角色绑定
│       └── stats.go                # 日志/用量/配置
├── web/                            # 前端静态资源（embed，由前端项目单独维护）
├── data/ai-novel.db                # SQLite 数据文件（自动生成）
├── run.bat / run.ps1               # 双击启动
└── go.mod
```

## 快速开始

1. **配置**：编辑 `configs/config.yaml`，填入模型 API Key（DeepSeek + Kimi 已预置示例）。
2. **启动**：双击 `run.bat`，或 `go run ./cmd/server`。
3. **访问**：`http://localhost:8081`

> 后台调整模型/角色绑定/限额：通过 `/api/models`、`/api/roles/{role}/models`、`/api/configs` 接口，或直接改 `config.yaml` 后重启（启动时幂等种子入库）。

## API 接口（RESTful）

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/generate` | 创作请求（SSE 流式：阶段信息 + 最终文本） |
| POST | `/api/verify` | 逻辑自检（问题清单 + 修改建议） |
| GET/POST | `/api/projects` | 项目列表 / 新建 |
| GET/PUT/DELETE | `/api/projects/{id}` | 项目详情 / 更新 / 删除 |
| GET | `/api/projects/{id}/versions` | 版本列表 |
| POST | `/api/versions` | 保存新版本（version 自增） |
| GET | `/api/versions/{id}` | 获取版本内容 |
| GET/POST | `/api/characters` | 人物卡（`?project_id=`） |
| PUT/DELETE | `/api/characters/{id}` | 改 / 删 |
| GET/POST | `/api/worldsettings` | 世界观 |
| PUT/DELETE | `/api/worldsettings/{id}` | 改 / 删 |
| POST | `/api/materials/upload` | 素材上传解析（txt/md/docx） |
| GET | `/api/materials` | 素材列表 |
| DELETE | `/api/materials/{id}` | 删除素材 |
| GET/POST | `/api/templates` | 模板（区分系统内置/自定义） |
| PUT/DELETE | `/api/templates/{id}` | 改 / 删（系统内置不可改删） |
| GET/POST | `/api/models` | 模型列表（api_key 脱敏） / 新增 |
| PUT/DELETE | `/api/models/{id}` | 改 / 删 |
| GET | `/api/roles/{role}/models` | 角色绑定的模型优先级 |
| PUT | `/api/roles/{role}/models` | 配置角色绑定（`{model_ids:[...]}`） |
| GET | `/api/logs` | 模型调用日志 |
| GET | `/api/usage` | 当日/近 7 日用量统计 + 限额阈值 |
| GET/PUT | `/api/configs` `/api/configs/{key}` | 后台配置（限额参数） |

### `/api/generate` 请求体

```json
{
  "project_id": "项目ID（可空，用于自动加载设定/前文）",
  "user_demand": "用户原始需求",
  "selected_text": "编辑器选中文字（无则空）",
  "world_setting": "世界观（空则按 project_id 从库加载）",
  "character_setting": "人物卡（空则从库加载）",
  "history_content": "前文（空则取最新版本）",
  "material_text": "素材文本",
  "target_word": 1000,
  "run_mode": "auto|strict|art|light|manual",
  "model_name": "手动模式指定模型名"
}
```

### SSE 事件类型（`data: {json}\n\n`）

`estimate`(token预估) → `plan`(流水线) → `stage`(阶段进度) → `token`(正文增量) → `warning`(降级/缺陷) → `done`(终稿)。微调重写前会发 `token` 事件且 `reset:true`，前端应清空已渲染文本。

## 调度规则与异常降级（已实现）

- **任务自动判定**：文本<500字/局部改写→轻量；严谨/学术→严谨；文笔/氛围→文艺；其余→标准。
- **上下文注入**：携带世界观/人物卡/前文/素材时，自动注入所有子任务（请求体显式传入优先，缺失则按 `project_id` 从库加载）。
- **分段执行**：目标>4000字自动分段（每段≤3500字），Thinker 产出分段框架，Worker 逐段撰写。
- **手动模式**：跳过流水线，直接调用指定模型。
- **降级**：子 Agent 失败（超时/429/报错）→ 自动切换该角色下一级备用模型；全部失败→友好中文提示。
- **迭代上限**：校验仍有缺陷时最大迭代 3 轮（可后台改 `max_iterations`），超限终止并标注现存问题。

## 调用限制与成本控制（`configs` 表，后台可改）

- 全局每日调用次数 / token 上限（达到返回「今日调用额度已用完」）
- 单请求 token 上限、轻量模式输入 500 字上限
- 单 IP 每分钟限流、并发上限（信号量）
- 单模型每日用量预算与预警比例（用于降级兜底）
- 每次生成前预估 token，所有调用记录持久化（模型/入参出参 token/耗时/状态）

## v1.0 已实现 ✅

- 4 角色 + 硬编码 Prompt 系统（规格第五章全部提示词）
- 调度中枢 + 4 标准流水线（标准/严谨/文艺/轻量）+ 手动模式
- 模型适配层：统一接口 + DeepSeek & Kimi（OpenAI 兼容）+ 角色注册中心（缓存）
- SSE 流式输出（阶段信息 + 正文增量 + 终稿）
- 备用模型降级 + 迭代上限 + 友好错误提示
- SQLite 持久化 + 全部 11 张表 DDL + 自动迁移
- 全部 API 接口 + 素材 docx/txt 解析
- 成本管控：配额/限流/并发/用量统计/调用日志/后台可配置阈值

## 后续迭代扩展点 🔜

- 自定义拖拽工作流（预留：流水线可在 `pipeline` 包新增，调度中枢已参数化）
- A/B 双稿对比（预留：`done` 事件可扩展多终稿字段）
- 向量 RAG 记忆系统（已移除旧实现，二期可重接入）
- 多用户/计费体系（当前按规格不做）
- 更多厂商适配器（实现 `llm.ModelAdapter` 接口即可，如 Claude/Gemini 原生协议）

## 关于模型配置

`config.yaml` 预置了 DeepSeek（`deepseek-chat`）与 Kimi（`moonshot-v1-32k`）。若某模型 Key/名称不可用（如 404），系统会**自动降级到该角色的备用模型**并推送 `warning` 事件，不影响生成。可在 `config.yaml` 或通过 `/api/models` 修改为你的 Key 支持的模型名。
