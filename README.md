# AI 写作助手

本地优先的**小说创作管理工具**（纯作家辅助）。数据全部保存在本地 SQLite，AI 只做助手（摘要、提取、分析），不写正文——正文由作者自己写，工具负责把设定、伏笔、人物、进度管得明明白白。

## 功能

**写作**
- 富文本编辑器（Tiptap）+ Markdown 模式，多套 UI 主题
- 卷 / 章节管理（拖拽排序、批量操作、版本历史、回收站）
- 全文搜索、字数统计（全书/本章）、自动保存与冲突保护

**世界构建**（世界观页 4-tab）
- 📖 设定：分类世界观条目，AI 自动从正文提取、丰富建议
- ⚔️ 势力：组织/势力档案（首领、成员、关系）
- 📍 地点：地点库（类型、描述、关联）
- 📅 时间线：剧情事件时间线（时间、关联章节、出场人物）

**人物**
- 人物卡（AI 自动提取 + 去重合并）
- 💞 关系库：手动维护人物关系 + SVG 关系图谱（另有 AI 分析式即时图谱）

**剧情管理**
- 章节大纲 / 场景卡（scene beats）
- 🔮 伏笔看板：手动标记 + AI 扫描全书识别伏笔，跟踪回收状态
- 素材库 / 文风样本 / 拆书导入（txt / md / epub / docx / html）

**阅读**
- 💬 批注 & 🎨 高亮：选中即标注，锚定基于纯文本偏移（编辑正文后自动重对齐），不污染正文
- 📍 阅读进度：自动记录上次读到哪一章

**项目管理工具**（纯数据规则，不调用生成）
- 一键取料包（人物卡/世界观/前情/摘要/伏笔拼成一段，方便贴给任意 AI 使用）
- 人物出场统计、意象/梗追踪、章末钩子检查、时间线视图、写作统计日历

> 说明：仓库内保留了一套历史遗留的多 Agent 生成流水线代码（Go），该能力已从界面移除，当前定位为纯辅助工具。

## 技术栈

- **后端**: Go 1.26 + chi v5 + Viper
- **数据库**: SQLite（`modernc.org/sqlite` 纯 Go 驱动，零配置）
- **前端**: 原生 JS + Tiptap 2（esm.sh CDN），静态资源 embed 进 server.exe
- **AI 适配**: OpenAI 兼容接口（DeepSeek / Kimi / GLM / MiniMax 等任意 OpenAI 兼容厂商）

## 快速开始

1. 配置 `configs/config.yaml` 填入模型 API Key（仅 AI 辅助功能需要，写作/管理功能不依赖 AI）
2. 双击 `run.bat`（或 `go run ./cmd/server`）
3. 浏览器访问 `http://localhost:8081`

启动时自动完成数据库迁移并备份快照到 `data/backups/`。

## 项目结构

```
├── cmd/server/               # 入口：装配各层、启动 HTTP
├── internal/
│   ├── domain/               # 领域逻辑（pipeline/roles 为历史生成流水线，已停用）
│   ├── infrastructure/
│   │   ├── database/         # SQLite 数据访问层（项目/章节/人物/世界观/势力/地点/时间线/关系/伏笔/批注/阅读进度…）
│   │   ├── config/           # Viper 配置
│   │   └── llm/              # 模型适配层（OpenAI 兼容）
│   └── interfaces/api/       # RESTful 接口
├── web/                      # 前端静态资源（editor/pages/tools 等模块化 JS）
├── configs/                  # 配置模板 + schema
└── data/                     # SQLite 数据文件（自动生成，不入库）
```

## 数据安全

- 全部数据存于本地 `data/ai-novel.db`，不依赖云端
- 每次启动自动备份一致性快照
- API 密钥仅在 `configs/config.yaml`（已 gitignore），仓库不含任何密钥

## 主要 API

| 方法 | 路径 | 说明 |
|------|------|------|
| GET/POST | `/api/projects` | 项目列表 / 新建 |
| GET/POST | `/api/chapters` | 章节列表 / 新建 |
| GET/POST | `/api/characters` | 人物卡 |
| GET/POST | `/api/worldsettings` | 世界观设定 |
| GET/POST | `/api/factions` `/api/locations` `/api/timeline` | 势力 / 地点 / 时间线事件 |
| GET/POST | `/api/relations` | 人物关系 |
| GET/POST | `/api/annotations` | 批注 / 高亮 |
| GET/POST | `/api/reading_progress` | 阅读进度 |
| GET/POST | `/api/foreshadows` | 伏笔 |
| POST | `/api/materials/upload` | 素材导入（txt/md/epub/docx） |
| POST | `/api/tools/execute` | 项目管理工具（取料包/统计/追踪等） |
