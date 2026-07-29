# AI Novel Studio 开发任务书

> 基于 2026-07-29 完整功能测试的发现，按优先级排列。

---

## 🔴 P0 — 立即修复的 BUG

### 1. 项目类型选择不生效
- **现象**：新建项目选择"校园"后，列表仍显示"未分类"
- **定位**：`web/js/project.js` 的 `showCreate()` 中，select 元素的值通过 `document.getElementById(idn + '_type').value` 获取并传给 `API.createProject(name, type)`，后端 `internal/interfaces/api/projects.go` 的 `HandleCreateProject` 正确接收并存储 `req.Type`
- **根因分析**：数据库表 `projects` 的 `type` 列可能存在写入问题，或者前端模态框关闭前 select 值未正确同步。请在 `create()` 函数中添加 `console.log(name, type)` 调试，确认 type 值是否为空字符串
- **修复方向**：确认 UI.modal 动态创建的 select 元素在执行 onClick 时值是否仍可访问。如果模态在回调前销毁了 DOM，需要先保存值再调用 create

### 2. "追加到末尾"导致内容重复
- **现象**：生成完成后点"📎 追加到末尾"，编辑器中的流式已追加内容被再次追加，导致全文重复
- **根因**：`web/js/sse.js` 的 `done` 事件处理中，`autoAppend !== false` (默认开启) 时已经将流式文本添加到编辑器。但 `genActions` 对话框仍然显示"追加到末尾"按钮（`editor.js:454`），点击后会再次 append `SSE.streamText`
- **修复**：当 `autoAppend === true` 时，`genActions` 应隐藏"追加到末尾"和"插入光标处"按钮，仅显示"↩ 撤销"和"✕ 丢弃"，因为文本已自动在编辑器中

---

## 🟡 P1 — 重要问题

### 3. 字数控制不精准
- **现象**：设定 3000 字，实际产出 5545~6455 字（1.85~2.15 倍）
- **定位**：`internal/domain/roles/prompts.go` — Worker 和 Verifier 的 prompt 中需要更严格的字数约束
- **修复**：
  - Worker prompt 中加入：`【硬性约束】全文字数严格控制在 ${target_word} 字 ±10%，超过范围视为不合格`
  - Verifier 检查项加入字数校验，超标时标记 warning 并要求 Worker 重写

### 4. 续写默认上下文不对
- **现象**：点击"续写"创建新章后，上下文范围默认"仅当前章节"（空章节），导致 AI 不知道前文内容
- **修复**：`web/js/composer.js` 或 `web/js/store.js` 中，续写触发时自动将 `contextScope` 设为 `"withSummary"`（前面章节摘要），并在 `buildPayload()` 中确保 `previous_summaries` 有内容

### 5. 项目标题栏计数滞后
- **现象**：续写完成后标题栏仍显示"第9章 · 0字"，需手动切换才能刷新
- **定位**：`web/js/sse.js` 的 `done` 处理中调用 `ProjectUI.updateMeta()` 但可能 DOM 未及时更新
- **修复**：在 `done` 的 `autoAppend` 分支中增加对 `docMeta` 的直接刷新，使用 `requestAnimationFrame` 确保渲染

### 6. Kimi API 密钥超额 (429)
- **现象**：`kimi-k3` 返回 `reached organization TPD rate limit`
- **修复**：更新 `configs/config.yaml` 中的 Kimi API key，或从模型列表中移除；在 `internal/infrastructure/llm/registry.go` 中对 429 错误增加更快的降级响应（当前已有降级机制，但日志应更明确）

---

## 🔵 P2 — 功能完善

### 7. 人物/世界观/素材面板落地
- **现状**：API 已完整支持 CRUD（`api.js:48-62`），后端接口齐全（`resources.go`, `characters`, `worldsettings`, `materials` 表），侧栏右侧面板有入口（"设定资源" Tab），但前端面板空白——没有编辑表单
- **需要做**：
  - 在 `web/js/resources.js` 中实现人物卡表单（名称、性别、年龄、性格、外貌、背景、关系图）
  - 实现世界观设定表单（时代背景、规则体系、地理、势力等）
  - 实现素材管理面板（图片上传/标签/关联章节）
  - 人物数量和世界观计数自然就有值了

### 8. 智能校对（错别字/标点/的地得）
- **API 已存在**：`POST /api/verify` 独立接口已在 `internal/interfaces/api/generate.go` 实现
- **需要做**：前端增加"校对"按钮，调用 `/api/verify` 后以 diff 视图展示问题清单和修改建议；用户可逐条接受/忽略
- **提示词优化**：`buildVerifyStandalonePrompt` 中增加中文常见错误检测规则

### 9. 写作统计面板
- 日写作字数趋势图（调用 API 获取每日统计）
- 各章长度分布柱状图
- 当前项目总字数、今日字数、连续写作天数
- 可复用已有的 `store.js` 中的章节数据和 `API.getUsage`

### 10. 大纲可编辑
- Thinker 产出大纲目前在右侧面板只读展示
- **需要做**：编辑按钮（已存在 `e46`）点击后切换为 editable textarea，允许手动调整章节结构、增删情节节点，保存后覆盖当前大纲

---

## ⚪ P3 — 体验优化

### 11. 封面图降级
- **现象**：多个项目封面 404（`data/covers/` 下文件不匹配项目名）
- **修复**：`project.js` 的 `renderList()` 中已有 `onerror` 降级为文字首字，确认生效。同时生成封面自动触发或提供默认占位图

### 12. 批量导出
- 侧栏已有批量模式 UI（`ChapterUI.batchMode`），`batchExportTXT/MD` 已存在按钮
- 需要实现 `ChapterUI.batchExportTXT()` 和 `batchExportMD()` 的导出逻辑

### 13. 搜索正文内容
- 当前搜索 (`e1`) 仅搜项目名
- 需要增加全文搜索（后端新增 `GET /api/search?q=xxx&project_id=xxx`，前端增加搜索结果面板）

### 14. "生成完成"对话框自动关闭
- `genActions` 在 `autoAppend===true` 时不应显示或应在操作后自动隐藏

### 15. 写作目标设定
- 顶部栏增加"今日目标"输入框，写入 localStorage
- 达到目标时弹 toast 庆祝

---

## 📁 关键文件速查

| 模块 | 文件 |
|------|------|
| 前端生成流程 | `web/js/composer.js`, `web/js/sse.js` |
| 前端编辑器 | `web/js/editor.js` |
| 前端项目管理 | `web/js/project.js`, `web/js/chapters.js` |
| 前端资源面板 | `web/js/resources.js`, `web/js/rightpanel.js` |
| 后端 API 路由 | `internal/interfaces/api/server.go` |
| 后端生成接口 | `internal/interfaces/api/generate.go` |
| 后端项目接口 | `internal/interfaces/api/projects.go` |
| 后段资源接口 | `internal/interfaces/api/resources.go` |
| 流水线执行 | `internal/domain/pipeline/execute.go` |
| 角色 Prompt | `internal/domain/roles/prompts.go` |
| 数据库 | `internal/infrastructure/database/` |
| 模型管理 | `internal/infrastructure/llm/` |
| 配置 | `configs/config.yaml` |

---

## 🎯 建议开发顺序

1. P0-1 + P0-2（Bug 修复，2-4h）
2. P1-3 + P1-4 + P1-5（字数控制 + 上下文 + 刷新，4-6h）
3. P2-7（人物/世界观面板，6-10h）— **用户感知最强的缺失功能**
4. P2-8（智能校对，4-6h）
5. P2-9 + P2-10（统计 + 大纲编辑，6-8h）
6. P3 各项目按需穿插
