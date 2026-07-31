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

---

### ✅ P2 优化项（2026-07-30 完成）

#### P2-003 章节删除回收站
- **后端**：`chapters` 表增加 `is_deleted`(INT) + `deleted_at`(TEXT) 列，自动迁移；`DeleteChapter` 改为软删除；新增 `ListTrashChapters`、`RestoreChapter`、`PermanentDeleteChapter`、`PurgeOldTrash` 方法
- **API**：新增 `GET /api/chapters/trash`、`POST /api/chapters/{id}/restore`、`POST /api/chapters/{id}/permanent-delete`（需 `confirm: true`）
- **前端**：`Tools.showTrash` 优先调用后端 API，失败时回退 localStorage；新增永久删除二次确认；"合并删除"和"批量删除"提示"可在回收站恢复，保留7天"
- **自动清理**：启动时和每小时自动清理 7 天前的过期回收站章节

#### P2-007 多模型失效自动备用切换
- **改进**：`callRoleWithModel` 和 `callRoleStreamWithModel`（orchestrated 模式）中，当指定模型失败后自动遍历该角色的备用模型列表，而非直接报错
- **通知**：前端已有 `EventWarning` 降级通知 + toast 弹窗（`⚠️ 模型降级：<原因>`）

#### P2-006 长篇全书校验
- **增强**：`verifyFullBook` 逐章校验结果以折叠面板展示，按章节高亮通过/失败状态，每章内展开详细问题清单，顶部汇总通过数/问题数

#### P2-008 空白面板引导文案
- **增强**：侧边栏人物卡和世界观面板空状态展示引导卡片，分别列出"手动创建"/"AI 一键提取"/"工具面板提取"三种方式，带视觉边框区分
- **增强**：右侧面板context标签的人物卡和世界观面板也增加引导文案和视觉样式

#### 体验小改善
- 章节树头部工具栏增加 🗑 回收站快捷入口
- `verifyCrossChapter` 改成分批审计避免大文本截断
- `mergeChapters` 和 `splitChapter` 改为软删除而非物理删除

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

---

## 鈿?P0 鈥?Bug 淇 鍥?2026-07-31 鏂板 淇 澶?

### P0-011 鐐瑰嚮鍒氳竟/椤甸潰 UI 鏃跺脊閿欒 鈥滈〉闈㈣烦杞 凡琚嫤鎴?鈥 骞朵笖鏃犳硶鍒囨崲椤甸潰
- **鐜拌薄**锛氱敤鎴风偣鍑诲垱浣滈〉闈㈤《閮ㄦ爮/宸ュ叿鏍忕殑 UI 鏃讹紝蹇嶅嚭閿欒 鎻愮ず銆婇〉闈㈣烦杞 凡琚嫤鎴?锛堜晶閿 瀵艰嚧锛?锛屽苟涓旀祻瑙堝櫒鏃犳硶璺宠浆鍒板叾浠栭〉闈€?/li>
- **鏍瑰洜**锛歛pp.js 銆?10. 闃绘 姝㈤紶鏍囦晶閿 瀵艰嚧椤甸潰璺冲嚭鈥?鐨?popstate 闃绘嫤鍣ㄥ皢搴旂敤鑷韩鐨?hash 鍐呴儴瀵艰埅锛堝垱浣滈〉闈?椤靛潡鍒囨崲锛夎 鍒ゆ柇鎴愨€滄祻瑙堝櫒鍓嶈繘/鍚庨€€鈥濓細
  1. 杞崲 hash 鍚庤Е鍙?popstate 鈫?handle 鎵ц  history.replaceState('novel','','/') 灏?hash 鎶逛簡
  2. 骞朵笖寮瑰嚭閿欒 鎻愮ず銆婇〉闈㈣烦杞 宸茶嫤鎴?鈥?/li>
  3. 绌哄?hash 鍙堣Е鍙?hashchange 鈫?Router.navigate('editor') 鈫?椤甸潰琚 寮瑰洖缂栬緫鍣ㄩ〉闈?鈥?/li>
- **淇 澶?/strong>锛?
  1. router.js 銆?navigate() 璁剧疆 hash 鍓嶅厛鏍囪  window.__appInternalNav = true 锛堣〃绀哄簲鐢ㄨ嚜韬 瀵艰埅锛?/li>
  2. app.js popstate 闃绘嫤鍣?鍏堝垽鏂 璇ュ爜锛氬簲鐢ㄨ嚜韬 瀵艰埅鍒欐斁琛岋紙涓嶅脊鎻愮ず銆佷笉鎶逛笉 hash锛?鍙 鐪熷疄鍓嶈繘/鍚庨€€鎵嶆嫤鎴?/li>
  3. 鏂板  mousedown 闃绘嫤閿?3/4锛堝井杞 渚ч敭 XButton1/2锛夛紝鍦ㄨ嚜韬 闃舵 鐩存帴闃绘 姝㈠洖杞 瀵艰嚧鐨?popstate
- **瀹炴祴**锛?/strong>姝ｅ父鍒囨崲鍏?椤?/code> 甯︾晫閮藉湪鑻忔晥锛屾棤閿欒 鎻愮ず锛?hash 姝ｅ父淇濈暀锛涚湡瀹炴祻瑙堝櫒鍚庨€€浠嶈 姝ｅ父闃绘嫤+鎻愮ず锛屼笉澶辫拷鎶ょ綒鍐呭

---

## 🔴 P0 Bug 修复记录（2026-07-31 新增修复）

### P0-011 点击编辑器那一栏的 UI 弹错误「页面跳转已被拦截」且无法切换页面
- **现象**：用户点击创作页顶部栏/工具栏/侧栏导航的 UI 时，弹出错误提示「页面跳转已被拦截（侧键导致），请勿使用浏览器前进/后退」，并且浏览器无法跳转到其他页面（点完又弹回编辑器页）。
- **根因**：app.js「10. 阻止鼠标侧键导致页面跳出」的 `popstate` 拦截器把**应用自身的 hash 内部导航**（侧栏/页面切换）误判为「浏览器前进/后退」：
  1. 设置 hash 后触发 `popstate` → 拦截器执行 `history.replaceState('novel','','/')` 把 hash 抹掉，并弹出错误提示
  2. 空 hash 又触发 `hashchange` → `Router.navigate('editor')` → 页面被弹回编辑器页
  3. 结果：每次点击导航都是「提示错误 + 跳转被弹回」
- **修复**：
  1. `web/js/router.js` 的 `navigate()`：设置 hash 前先标记 `window.__appInternalNav = true`（表示应用自身导航）
  2. `web/js/app.js` 的 popstate 拦截器：先判断该标记——应用自身导航则放行（不弹提示、不抹 hash）；只有真实浏览器前进/后退才拦截
  3. 新增 `mousedown` 拦截鼠标侧键（button 3/4，即 XButton1/2），在源头阻止侧键回退/前进触发的 popstate
- **实测**：6 个侧栏页面（编辑器/章节大纲/人物卡/世界观/AI工具箱/仪表盘）真实鼠标点击全部正常切换、无错误提示、URL hash 正常保留；真实浏览器后退仍正常拦截+提示（防丢稿保护不丢失）；编辑器工具栏按钮（B/I/U/H2/H3/预览/专注/保存版本）全部正常无报错。


---

## 🔴 P0 Bug 修复记录（2026-07-31 晚 · 用户视角审查轮）

### P0-012 Token 消耗明细面板一直出现不消失
- **现象**：顶栏 Token 数字下方"各角色 Token 消耗"面板常驻显示，永不消失（设计意图是"悬停查看"）。
- **根因**：`web/js/usage.js` 的 `render()` 里 `roleBD.style.display = ''` 强制显示，且全站没有绑定任何 hover/click 显隐逻辑。
- **修复**：默认隐藏；`Usage.bindBreakdownHover()`（只绑定一次）——鼠标悬停显示、移开隐藏、点击显示、点击外部隐藏；内容在 render 时预填。

### P0-013 仪表盘统计全是错的（字段名与后端不匹配）
- **现象**：全书 7625 字但"0卷/0章"、柱状图永远"暂无写作数据"、今日 API 调用/Token 恒为 0。
- **根因**：`pages-dashboard.js` 读的是 `stats.chapter_count`/`stats.volume_count`/`stats.chapter_word_counts`/`usage.call_count`/`usage.token_count`，但后端实际返回 `total_chapters`/`volumes[]`（无 chapter_word_counts），usage 返回 `today:{calls,tokens}`。
- **修复**：`renderStats` 改读 `total_chapters` + `volumes.filter(v=>v.volume_id).length`（"未分类"是伪卷不计入）+ `usage.today.calls/tokens`；`renderChart` 在无 `chapter_word_counts` 时回退用 `Store.state.chapters`。
- **实测**：仪表盘显示"0卷 / 1章"、调用 25、Token 72,996、柱状图 1 根，与大纲页数据一致。

### P0-014 其他小修
- 重复 `id="previewBtn"`（HTML 非法，工具栏预览按钮永不高亮）→ 工具栏改 `previewBtn2`，`togglePreview` 同步两个按钮高亮。
- 编辑器灵感提示"✕ 关闭自动提示"点了不关闭（反而重置 30 秒计时器）→ 新增 `Editor.closeIdleHints()`：真正禁用 + localStorage 持久化 + 恢复右侧面板原内容。
- 导出 PDF 被浏览器弹窗拦截时 `w` 为 null 直接抛 TypeError 无提示 → 加 null 检查 + 错误 toast。


### P0-015 右侧 AI 助手面板整块不可见（重大布局 bug）
- **现象**：右侧面板（流水线进度/大纲/审核结果/AI 总结）在桌面端几乎完全不可见——tabs 右缘被裁 32px，正文区域被挤到屏幕外（right-body 只有 28px 宽、位于 x=1732，视口右缘 1699）。
- **根因**：`body.desktop .right-panel { display:flex; }` 没有配套 `flex-direction:column`，flex 默认 row 方向，导致 `.right-tabs`（图标栏）和 `.right-body`（内容）横排：tabs 占 344px 不收缩，body 被压成 28px 且溢出面板右缘（body overflow-x:hidden 裁掉）。
- **影响**：生成时的流水线进度面板、大纲、审稿结果用户一直看不到；这解释了此前"续写看不到进度"的反馈。
- **修复**：`web/css/style.css` `body.desktop .right-panel` 补 `flex-direction:column`。
- **实测**：面板内 tabs/body/pipeCard 全部回到视口内（1388-1699），MiMo 视觉模型截图识别到右侧"构思大纲→动笔写作→品质审稿"流水线内容（修复前识别不到）。

### P0-016 选中项目的标题被章节列表挤没
- **现象**：选中项目后侧栏卡片标题只剩 14px（"那年夏天的风铃"几乎不可见），因为 `.novel-expand`（章节列表）是 `.novel-item` 这个 flex row 的子元素，展开时和标题横排抢宽度，把 `.novel-info` 压到 0。
- **修复**：`.novel-item` 加 `flex-wrap:wrap`，`.novel-expand` 加 `flex-basis:100%`，章节列表换行到标题下方整行显示。
- **实测**：选中卡片标题 123px 完整显示，展开区在标题下方（expandTop > titleTop），其余长标题正常省略号。


### P0-017 续写/生成超长自动分章（新功能，替代"超长裁剪丢弃"）
- **原行为**：生成文本超过目标字数 30% 时，按句子边界裁剪，超出部分直接丢弃（sse.js done 分支），用户长续写内容会丢失。
- **新行为**：自动追加模式下超目标 30% → 按目标字数 1.1 倍为一段、句子边界切分，当前章保留第一段，其余自动创建"第N章"新章节并落库，toast 提示拆分结果。
- **实现**：`web/js/sse.js` 新增 `SSE.splitIntoChapters(fullText, boundCh, tw)`（同步写第一段 + Promise 链逐章 createChapter + renderTree/updateMeta），done 分支超长时调用（autoAppend=false 时保留旧裁剪逻辑）。
- **实测**：构造 3000 字文本 target=1000 → 自动拆 3 章（1088/1086/826 字，句子边界），章节落库、树刷新、toast 正常；测试后已还原原章节内容。

### 8 种创作模式全量真实生成测试（2026-07-31 晚，临时测试项目）
- 测试方法：每个模式用"写一段夏夜月亮描写"（目标100字）真实跑通 SSE 全链路，统计事件流/耗时/正文长度，全部收到 done 且无 error：
  1. auto 智能协同：131字 / 30s（detectPipeline 自动选流水线）
  2. draft 快速草稿：80字 / 7s（Worker 直出，单次调用）
  3. light 轻量快捷：175字 / 10s（Helper 单次）
  4. manual 手动自选：132字 / 3s（直调指定模型，流式）
  5. art 文艺创作：135字 / 18s（Thinker→Worker 轻审）
  6. strict 严谨模式：148字 / 52s（Thinker→Worker→Verifier 多迭代）
  7. orchestrated 指派Agent：143字 / 10s（多角色指定模型，有 warning 降级事件但正常 done）
  8. collab 协同闭环：124字 / ~150s（Thinker→Worker→Verifier→重规划→重写 完整闭环）
- 结论：8 种模式全部可用；耗时差异明显（draft/light/manual 秒级，collab/strict 分钟级）。


### P0-018 自动分章重构：按后端分段标记拆章（替代纯字数切分）
- **调研**：网上查证 AI 小说生成项目（AI_NovelGenerator / AI-Novel-Generator / AI_NovelCraft），主流做法是"生成时按章节组织"，而非"事后按字数切"。本系统后端 `workerWrite` 在目标 >4000 字时本就分段生成（每段 ≤3500 字、模型按情节连续生成），但拼接时丢失了段边界信息。
- **改造**：
  1. 后端 `types.go` 新增 `ChapterBreakMarker = "

[=====AI-NOVEL-CHAPTER-BREAK=====]

"`；`execute.go workerWrite` 分段拼接时在段间插入该标记（prevSegs 传给下一段模型的参考文本不含标记，保持干净）。
  2. 前端 `sse.js`：done 时检测到标记即触发拆章；`splitIntoChapters` 优先按标记拆（清理标记与首尾空行），无标记才回退句子边界切分。
- **端到端实测**：临时项目 target=5000 → 后端"Worker 撰写第 1/2 段、第 2/2 段" → 自动拆成 第1章 6724字 + 第2章 13846字，章节内容无标记残留、情节衔接自然（第2章开头"第二天中午…"）。测试后已删除临时项目、正式项目数据完好。

### P0-019 前端内容丢失链路排查（重点）
- **修复 1（终止/断网丢内容）**：`SSE.stop()` 只 abort，`finish()` 不保存——用户点"■ 终止"或网络中断时，已流式生成的内容只存在内存/编辑器 DOM（流式写入不触发 autosave），刷新即丢。修复：`finish()` 兜底保存——streamText 非空且与章节内容不同时，立即 `API.updateChapter` 写入绑定章节并 toast。正常 done 路径幂等（autoAppend 已清 streamText；非 autoAppend 时 done 已写入）。
- **修复 2（done 清空章节）**：done 分支 `boundCh.content = ev.final_text || ''`——若 final_text 为空会把章节已有内容清空。改为仅当 `ev.final_text` 非空才写入。
- **确认无丢失**：error 事件分支已有保存逻辑（此前修复）；`_flushStreamBuf` 切章保护只丢 DOM 显示、`streamText` 全量保留，最终由 done/error/finish 统一持久化。

### 看图 skill 换 MiniMax M3（2026-07-31 晚）
- 小米 MiMo 对 UI 审查类任务不可靠（4 次测试：3 次答非所问/空输出，1 次部分可用）。
- 用户提供 MiniMax key（sk-api-…，已写入 `~/.config/opencode/opencode.jsonc` provider.minimax，国内站 https://api.minimaxi.com/v1）。
- `vision.ps1` 支持 `-Provider`/`-Model` 参数，默认 minimax/MiniMax-M3；修了 PS 变量大小写 bug（$provider 与 $Provider 同名覆盖）。
- 实测：MiniMax-VL-01 模型名不存在；M3/M2.1/M2/abab6.5s 可用；**M3 支持图片输入且能力远超 MiMo**——准确读出顶栏数值、项目名、仪表盘全部统计，能发现疑点（"右侧面板遮挡"经 DOM 验证为误判，但读数据能力可靠）。
- 注意：`powershell -File` 传中文参数会乱码（PS5.1 GBK），视觉审查 prompt 用英文更稳。


### P0-020 全角色接入 deepseek-v4-flash + 字数失控/超长耗时根治（2026-07-31 晚）
- **用户反馈**：5000 字目标生成 2 万字、耗时 15+ 分钟不正常；要求所有 agent 接入 deepseek-v4-flash（key 与 deepseek-v4-pro 相同，sk-94dd…）。
- **字数失控根因**（两层）：
  1. `verifyAndRevise` 微调让 Worker 重写全文且无字数约束 → 每轮 Verifier 挑毛病→全文重写→模型习惯性扩写→滚到 2 万字；多轮全文重写 = 耗时爆炸
  2. `workerWrite` 分段 prompt 只有"约 3500 字"没有硬上限，模型单段可写 6600+
- **修复**：
  1. `buildReviseUserPrompt` 加【字数硬约束】：修正后全文必须在目标 80%~110% 之间，超了必须删减（修复膨胀源头）
  2. `buildWorkerUserPrompt` 分段加"严禁超过 segmentSize+500 字"
  3. `execute.go` 新增 `trimToSentenceBoundary`：段级超长（>4300）按句子裁剪；`verifyAndRevise` 入口正文超目标 130% 先裁剪再校验
- **实测**：目标 5000 → 第1章 3260 + 第2章 2199 = 5459 字（109%），483 秒完成（vs 之前 2 万字/15+ 分钟），自动分章正常、章节衔接自然、无标记残留
- **空响应失败根治**：deepseek-v4-flash/pro 是推理模型，偶发"返回空响应"（思考占满 max_tokens 或 API 抖动）导致整链失败（thinker 空响应即失败）。修复：
  1. `openai.go` NewOpenAICompatible 增加 maxTokens 参数（不再硬编码 4096，改用 DB 配置 8192）；Generate 空响应自动重试 1 次
  2. `dispatcher.go callRoleStream` 流正常结束但正文为空 → 记为错误并降级到备用模型（原来当成功返回空文本）
  3. DB role_models 每角色加 deepseek-chat 备用（priority 1，官方模型名更稳），空响应重试仍失败自动降级
- **模型切换**：DB models 表新增 deepseek-v4-flash（DeepSeek/官方 endpoint/真实 key），4 角色主模型全部指向它；configs/config.yaml 同步
- **验证细节**：generation_logs 查询必须 ORDER BY created_at（id 是 UUID 字符串排序不可靠，曾误判"日志没写入"）


### P0-021 生成提速 3 倍 + 空响应根治（thinking 关闭）+ 全模型盘点（2026-07-31 晚）
- **用户反馈**：8 分钟还是长；项目不止 4 个 agent；给了多个 API key；免费网页 AI 没测过
- **提速根因**：deepseek-v4-flash/pro 是推理模型，**默认开启思考模式（thinking）**，思考内容占满输出预算 → thinker 93s（out 到 8191 截断）、worker 段超写截断（96s）、verifier 空响应超时（138s 失败）
- **核心修复**：`openai.go` NewOpenAICompatible 对 deepseek-v4-flash/v4-pro 加 `ExtraFields: {"thinking": {"type": "disabled"}}`（eino openai 组件支持 ExtraFields 透传）；DB `max_iterations` 3→2
- **实测对比**（目标 5000 字，auto 模式）：
  - 修复前：483s（thinker 93 + worker 34/96 + verifier 138 失败）
  - 修复后：**165s**（thinker 45 + worker 26/39 + verifier 4.7 通过），字数 5447（109%），自动分章正常
  - verifier 从 138s 空响应失败 → 4.7s 通过：**空响应根因就是 thinking 占满 max_tokens**，顺带根治
- **模型盘点**（角色=4 个 thinker/worker/verifier/helper，模型=9 个可绑定）：
  - DeepSeek×3：deepseek-chat（备用）、deepseek-v4-pro、deepseek-v4-flash（4 角色主模型）
  - Kimi×2：moonshot-v1-32k、kimi-k3（可用，未绑定角色）
  - MiniMax-M3：**新接入 models 表**（用户新 key，api.minimaxi.com，多模态，未绑定角色，可在前端模型管理选择）
  - 网页 AI×2：QT_WebAI_Test（kimi web）、Test_WA_New（kimi-free）——**已测，Cookie 均失效**（"Cookie 已失效或格式错误"），需用户重新提供有效 Kimi 网页 Cookie
- **遗留**：网页 AI 需有效 Cookie 才能用；MiniMax-M3 可绑定到角色或手动模式选用


### P0-022 深度思考开关 + 进度条重做 + 免费网页AI扩展（2026-07-31 晚）
- **用户反馈**：别默认关 thinking（质量差），做成用户可选；token 上限别设；免费网页 AI 多加（豆包/DeepSeek）；进度条看不懂；自动抓 cookie 功能要用起来
- **thinking 开关**（默认开=质量优先）：
  - 后端：GenerateRequest 加 `disable_thinking`；ModelAdapter 接口加 `SetThinking(bool)`；OpenAICompatibleAdapter 双实例（thinking 开/关各一个 eino chatModel，关的带 ExtraFields thinking disabled，懒创建）；dispatcher 的 callRole/callRoleStream/callRoleWithModel/callRoleStreamWithModel 全部加 disableThinking 参数（18 个调用点脚本插入，execute.go 有备份 .bak_thinking_*）
  - 前端：生成区顶栏加「🧠 深度思考」checkbox（默认开，Composer.onThinkingChange 持久化），payload 传 disable_thinking
  - 实测：800 字目标，thinking 开 177s vs 关 99s（快 45%）
- **进度条重做**（sse.js）：
  - 通俗阶段文案：⏳准备中 → 📋规划中 → 📋规划故事框架中(12%) → ✍️正在写作（第x/y段）(20-75%) → 🔍审稿中(78%) → 🔄修改中(88%) → ✅审稿通过(92%) → ➕补写中
  - 新增计时器：显示"已用 X分X秒 · 预计还需约 X分X秒"（按阶段权重估算剩余）
  - 双维度进度：阶段权重为主，写作阶段按字数在 20%~75% 内微调；完成自动隐藏
- **token 上限**：configs per_request_token_limit 32000 → 0（不限制）
- **免费网页 AI 扩展**：models 表新增「豆包免费版」(doubao-free) 和「DeepSeek免费版」(deepseek-free) 模板（web 类型，cookie 待填）；模型管理面板「免费网页AI」标签加「🪄 自动获取」按钮（调用既有 /api/webai/auto-cookie 流程：rod 开浏览器→用户登录→轮询抓取→自动保存模型），原自动抓取功能前端 UI 缺失已补上


### P0-023 角色分级思考策略 + verifier 超时根治（2026-07-31 晚）
- **用户反馈**：需要思考的 agent 开思考、干脏活累活的不开；网页 AI 3000 字 20 秒，为什么 API 流水线这么慢；问要不要加"脏活快手"agent；要求测免费网页 AI
- **思考策略最终版**（dispatcher 4 个 call 函数统一）：`ad.SetThinking(!disableThinking && role == llm.RoleThinker)` —— **仅 thinker（规划）开思考**（大纲质量最关键）；worker/helper（写作/轻活）不思考（快）；verifier（审稿）不思考
- **verifier 超时根治**：实测 verifier 开思考审 5000 字全文 > 3 分钟超时（每次白等 180s，占 42% 耗时）→ 不思考后 **180s 超时 → 6.8s 通过**
- **实测对比**（5000 字标准模式）：
  - 全开 thinking + iter3（旧）：483s（verifier 超时浪费）
  - thinker 思考 + worker/verifier 不思考（新默认）：**165s**，thinker 68s / worker 39+22s / verifier 6.8s，字数 5448（109%），自动分章正常
  - 结论：新策略与"全关"同速，但 thinker 保思考 → 大纲质量更高
- **prompt 精简**：thinker 加【输出要求】禁止开场白/解释；worker 加【输出要求】直接输出正文、禁止思考过程/标题/多余说明（减少"假时间"）
- **网页 AI 实测**：现有 Kimi 两个 web 模型 Cookie 已失效（"Cookie 已失效或格式错误"），豆包/DeepSeek 模板 cookie 为空——需用户用「🪪 自动获取」重新登录抓取
- **"加 agent 干脏活"结论**：不加新角色。worker/helper 不思考已实现"快写手"；draft 模式（Worker 直出）和 light 模式（Helper 单次）就是脏活专用通道，秒级到 1 分钟


### P0-024 推荐思考配置 + 全开模式优化 + Cookie 抓取修复（2026-07-31 晚）
- **用户需求**：别默认全开思考；默认=推荐配置（几个动脑几个不动脑）；优化 thinking 全开模式；worker 不动脑；自动弹出的登录页未登录就消失
- **推荐默认配置**（前端默认 + 后端 thinkingEnabled 缺省值一致）：**仅 thinker（规划师）开思考**，worker/verifier/helper 关。UI 勾选框默认同步：规划✓ 写作✗ 审稿✗ 轻活✗；顶部「🧠 思考·全开」= 一键全开/全关
- **为什么 worker 不用动脑**：写作是执行/生成任务，思考链开销大收益小；实测 worker 不思考 600 字 6-9s 写完，开思考要 30s+。质量杠杆在规划（大纲）与审稿（检查）
- **实测数据（600 字 standard 流水线）**：
  - 推荐配置（仅 thinker 思考）：thinker 28-83s（API 波动）+ worker ~9s + **verifier 3.7s** ≈ 35-95s 完成，单轮通过
  - verifier 开思考：审 700 字要 42-155s（ct 5000-14000）→ 全开模式慢的元凶
- **全开模式优化**（仍可选，勾选「思考·全开」）：
  - openai.go 新增 budget()：思考开启时输出预算自动放宽到 max(16384, maxTokens)，防止"推理占满 8192 预算→正文截断/空响应"（此前 verifier 138s 失败空响应的根因）
  - Stream 路径补上 WithMaxTokens（此前流式未传 max_tokens）
  - execute.go：orchestrated/collab 的 verifier 输入也统一 truncateForReview(8000 字)，避免长文审查上下文爆炸
- **Cookie 自动抓取登录页秒关根因**：hasSessionCookie 只数 Cookie 数量（>=3 即算登录成功），而 DeepSeek 打开即种 HWWAFSESID/ab 等风控 cookie → 秒判成功→关浏览器
  - 修复：垃圾 cookie 过滤（风控/统计类：csrf/waf/sensors/hwwafsesid/ab/cna 等）+ 登录态判定（cookie 名含 session/token/sid/auth/login/uid/member/user 且值>=8，或>=2 个长值 cookie，或整串>=200 字符）
  - 前端弹窗实时状态：显示已等待时间 + 已检测到 N 个 Cookie；抓取成功保存模型名不再重复「免费版」后缀
- **排查插曲**：一度误判"verifier 关思考仍慢 117s"，实为测试脚本 role_thinking 里 verifier 误留 true；debug 打印 map 证实后端忠实生效（thinker=true worker=false verifier=true 按 payload 执行）——后端逻辑本就正确


### P0-025 审稿清单执行核对 + 硬性输出格式（2026-07-31 22:00-22:20 轮）
- **用户需求**：检查 thinker 的要求 worker 是否全部执行了；若 verifier 能核对清单执行情况则换该配置
- **实测发现**：verifier 不思考时**会敷衍**——真实提示词+故意违反 4 条清单的正文，1.3s 直接输出【校验通过】（完全不核对）
- **解法：硬性输出格式**（user prompt 尾部强制）：①先输出【清单核对】逐条「条目N：已执行/未执行——证据(引用正文原文)」②再输出【缺陷清单】③禁止只输出【校验通过】四字
- **实测对比**（同一份违反 4 条的正文 + 真实系统提示词）：
  - thinking off + 无硬格式：1.3s，【校验通过】（敷衍）
  - thinking off + 硬格式：**10.1s，6 条全核对，精准揪出 4 条违反+字数不足**，还给出「青石板→碎砖地、橘猫→麻雀」级修改建议
  - thinking on + 硬格式：12.7s，同样逐条核对（思考未明显加分）
- **结论**：verifier 不思考 + 硬格式 = 快（10-19s）+ 可靠核对。VerifierPrompt 加「0. 规划执行核对（最高优先）」；extractIssues 跳过【】/条目N 核对行，只取真缺陷
- **完整流水线实测**（600 字 standard，推荐配置）：thinker 23-76s（API 波动）+ worker 6-10s + verifier 15-19s 硬核对 → 一轮过 51.9s / 发现缺陷 3 轮微调 128s
- **附带修复**：微调后字数膨胀裁剪（>130% 裁到 110%），防「微调越写越长」（600→2038 的 340% 膨胀已控）
- **已知小瑕疵**：SSE 累计显示字数 = worker 首稿全量，交付 = 裁剪后文本，前端显示可能偏大（记录待修）


### P0-026 学习 InkOS 多Agent思路 + 去AI味功能（2026-07-31 22:20-23:00 轮）
- **用户需求**：学习「同类型产品」文件夹（inkos-master）的多 agent 调度思路；给每个 agent 加严格提示词；实现「去AI味」功能
- **InkOS 调度思路学习**：每个 agent = 职责边界（禁止做什么）+ 硬规约（具体数字规则）+ 输出契约（返回格式）；流程 planner→writer（含动笔前自检）→reviewer→reviser→polisher（只动文字层，结构问题用 [polisher-note] 上交）→post-write-validator。核心文件：writing-methodology.ts（去AI味正反例+六步人物心理）、polisher.ts（润色提示词）、ai-tells.ts（规则检测）
- **提示词强化**（roles/prompts.go）：WorkerPrompt 加 10 条「去AI味写作规约」（情绪外化/五感/动词驱动/句长多样/禁AI标记词(仿佛宛如不禁显然)/禁"不是…而是"、破折号限量/"了"字控制/段落40-120字/对话辨识度/禁"众人齐声"/删无效描写/口语化转折）；VerifierPrompt 加「6. AI味检测」审查项
- **规则检测器移植**（ai_tells.go，纯规则无LLM）：段落等长（变异系数<0.15）、套话密度（似乎/可能/或许>3次千字）、转折词重复（≥3）、AI标记词密度、列表式结构（≥3句同开头）、"不是…而是"句式、破折号滥用、"了"字堆砌 → /api/ai-tells
- **去AI味润色**：PolishSystemPrompt（移植 InkOS polisher，6雷点+硬规约+输出契约±15%）→ /api/ai-polish（worker 角色，上限12000字）
- **前端**：编辑器工具栏「✨去AI味」按钮 → 先检测（弹报告）→ 润色替换全文（可撤销，undoContent 快照）
- **实测**：AI味文本（套话24次/千字、然而×3、了字42次/千字）→ 检测 4 项全中；润色 164→160 字（±15%内），"他感到非常愤怒"→"他很愤怒，但脸上没露出来"、"然而"全消、叙述者总结句改写
- **完整流水线实测**（600字推荐配置）：worker 输出明显更人味（动词驱动/五感），verifier 更严格（3轮微调打磨），总 99.5s
- **已知问题（延续 P0-025）**：SSE 累计字数=各轮全量，交付=裁剪后，前端显示偏大；verifier 严格+字数硬约束下微调轮数偏多，字数仍可能膨胀（兜底裁剪已生效）


### P0-027 移植 InkOS 审查三件套：评分制 + 净改进停止 + 快照回退（2026-07-31 22:37-23:00 轮）
- **用户批准**：学习 InkOS 循环控制机制（比提示词层更精华）
- **改动**：
  1. VerifierPrompt + buildVerifierUserPrompt：硬格式新增「最后一行输出【评分】N/100，85 分及格」
  2. verifyAndRevise 重写：parseReviewScore 解析评分（含旧格式 fallback）；评分≥85 或含"校验通过"即通过；**净改进停止**（微调后评分提升 <3 分即停）；**快照回退**（每轮记录 内容+评分+长度合规，循环结束挑评分最高、同分优先长度合规的版本）；微调未产出新内容即停
- **实测对比（600 字 standard）**：
  - 推荐配置：58.7s，1 轮微调后「校验通过（88分）」（之前 3 轮空转 99.5s）
  - 全开模式：**202.2s、1 轮通过（88分）、588 字（98% 达标）**——之前 340s、3 轮微调、1366 字（228% 超写）。快 40% + 字数根治
- **结论**：评分制让 verifier 精准放行（88 分直接过，不再吹毛求疵），快照回退根治"微调越改越长"；净改进停止防空转


### P0-028 前端 UI 体检 + P0 改进（2026-07-31 22:54-23:10 轮）
- **用户要求**：用识图 skill 看前端，对比 InkOS 前端和网上主流设计，找可改进点
- **体检方法**：CDP 截图 + MiniMax-M3 视觉分析（image-vision skill，-Provider minimax）。自家编辑器页 13 条问题；对比 InkOS studio（米白+酒红暖色、衬线字体、大留白、单一 CTA）
- **P0 五项已实施**（用户确认全做）：
  1. 顶栏精简：「今日调用/Token」进度条 meter 隐藏，改紧凑小字（调用 151/500 · Token 735k/2M）；usage.js meter 引用加空保护
  2. 工具栏分组：版本组与章节组之间、文档组（导入/导出/AI总结/净化）前加 tool-divider
  3. 模型选择折叠：「指派Agent模型」三个下拉折叠成「⚙️ 模型配置 ▾」按钮，点击展开（toggleOrchPanel）
  4. 目标字数标签：生成区数字输入框前加「目标字数」说明
  5. 模式提示截断修复：orchestrated 文案缩短 + modeHint 加 ellipsis 防溢出
- **重要教训**：前端静态文件是 **go:embed 进 server.exe** 的（webstatic.FS，无本地回退），**改前端必须重新编译部署才生效**——第一次改完没编译，截图全是旧页面，白跑一轮验证
- 已验证：重新编译部署后刷新，5 项全部生效；折叠点击展开 3 个下拉正常
- P1 待办（未做）：橙色主色收敛、项目头像底色统一、副本标签、空状态引导；P2：暖色纸感主题、衬线标题


### P0-029 新主题「纸墨书香」（InkOS 风格，用户可自选切换）（2026-07-31 23:01-23:15 轮）
- **用户需求**：P1/P2 的视觉改进弄成一个新模式，让用户自己调换
- **实现**：新增主题 paper-ink「纸墨书香」📜（浅色暖调）：
  - themes.css 追加 [data-theme="paper-ink"] 全套变量：暖米白纸感背景(#f3eddf)、酒红主色(#8c2f39)、衬线字体(Noto Serif SC/宋体)、编辑区稿纸细横格线、侧栏浅纸面板、顶栏纸带
  - themes.js 注册表加 paper-ink（主题菜单 + 外观面板自动出现，与 dark/light/ink-study/gold/focus 并列）
  - index.html 首屏防闪烁白名单加 paper-ink
- **验证**：MiniMax 视觉确认暖米白+酒红符合 InkOS 风格，可读性良好；已切回默认 dark
- **插曲**：第一次改 themes.js 后磁盘文件未更新（PowerShell 解析错误吞掉了写入命令），服务端一直返回旧版——重新执行补丁+重新编译解决
- 遗留小项（vision 提示）："已保存/字数"数字在浅色主题下的强调色显示待微调（不影响使用）

### P0-030 空状态引导卡片 + 顶栏用量降噪（2026-07-31 23:13-23:25 轮）
- **用户要求**：前端体检后"开始吧"→ 实施改进
- **1. 空状态引导卡片**（新用户第一印象，最高优先）：
  - index.html 编辑区新增 `.empty-guide` 卡片：徽章「📖 AI Novel Studio」+ 标题「开始你的第一部小说」+ 三步引导（新建项目→写下创作需求→点击生成）+ 两个按钮（＋新建项目 / 📚查看模板）+ 底部提示「已有项目？点击左侧项目名直接进入编辑」
  - style.css 追加引导卡片样式（居中卡片、编号圆点、与深色主题协调）
  - editor.js 新增 `Editor.updateEmptyGuide()`：无当前章节且无正文 → 显示，否则隐藏
  - 调用点：chapters.js selectChapter（选中→隐藏）、editor.js setContent（有内容→隐藏）、project.js 清空时（无项目→显示）
  - 验证：未选项目时显示引导；点击「测试-角色开关」自动打开第1章后引导自动隐藏 ✅
- **2. 顶栏用量降噪**：
  - 调用/Token 数字加 `.quota-mini` 样式（字号10px、透明度.65、悬停变亮）——不占视觉焦点但可读
  - 修复 usage.js 残留 bug：meter 元素删除后 `tm.querySelector('i')` 无空保护 + `tp`/`cp` 未定义（会抛 ReferenceError），补全 `var cp/tp` 计算 + 空保护
  - 验证：usage 正常渲染（qCalls=151），无控制台报错 ✅
- 已重新编译部署（go:embed），刷新即生效

### P0-031 主题切换全检 + 浅色主题顶栏可读性修复（2026-07-31 23:29-23:45 轮）
- **用户要求**：各种模式都看了吗（实际指**主题切换**）——用眼睛看 6 个主题
- **检查方法**：CDP 切换 6 主题（dark/light/ink-study/gold/focus/paper-ink）+ DOM 计算颜色对比度（WCAG）
- **结论：主题系统本身无 bug**——之前疑似"下拉框颜色不随主题"是测试方法错误（直接改 localStorage 绕过 Store，Store 才是主题持久化源；且切换后 700ms 内读取会读到上一主题残留变量）
- **真实问题 1 个 + 修复**：light/paper-ink/ink-study 三个浅色主题顶栏文档标题用浅橙色（--accent 渐变文字），在浅底上对比度 2.34-4.83（WCAG AA 需 4.5）
  - themes.css 追加修复：浅色主题 .doc-title 用 var(--text) 深色文字（去掉渐变）
  - 修复后对比度：light 15.14 / paper-ink 11.97 / ink-study 8.82 / dark 6.77（深色保留橙色渐变效果）
- **教训**：①主题持久化走 Store.set/get('theme')，不是 localStorage 直读；②主题切换后要等 CSS 变量重算（≥2s）再读取颜色，否则读到残留值
- 已重新编译部署

### P0-032 工具栏收纳「⋯更多」+ 模式选择器分组（2026-07-31 23:45-23:55 轮）
- **用户要求**：改（采纳"工具栏收纳 + 模式精简"建议）
- **1. 工具栏收纳**：
  - index.html：tool-right 里 4 个低频按钮（导入/导出/AI总结/净化）收进「⋯ 更多」下拉菜单（moreMenu）
  - editor.js 新增 Editor.toggleMoreMenu(ev)：点击展开/收起，点击外部自动关闭
  - style.css 追加 .more-menu/.mm-item 样式（面板下拉、悬停高亮）
  - 验证：点击展开显示 4 项（导入文档/导出文档/AI总结/净化），点击外部关闭 ✅
- **2. 模式选择器分组**（8 模式全保留，前端分组）：
  - 常用组：智能协同（推荐）/ 快速草稿 / 指派Agent模型 / 手动自选模型
  - 进阶组：严谨模式 / 文艺创作 / 协同闭环 / 轻量快捷
  - 后端逻辑零改动，只改前端 optgroup 展示
  - 验证：2 组 optgroup + 8 个 option ✅
- 已重新编译部署

### P0-033 全主题视觉审查（MiMo）+ 空章节引导修复（2026-07-31 23:50-24:00 轮）
- **用户要求**：继续用识图 skill 把所有主题看了，对比同类产品给改进/保留/精简建议
- **6 主题视觉分析**（小米 MiMo，英文 prompt 效果稳定；中文/复杂 prompt 会泛化或返回空）：
  - dark 经典深色：深黑+亮橙，7/10
  - light 经典浅色：白+橙，8/10
  - ink-study 墨韵书斋：深棕+暖米（文学感），7/10（工具栏稍密）
  - gold 鎏金：黑+金，分区清晰，高对比
  - focus 沉浸专注：深蓝灰，8/10 专业
  - paper-ink 纸墨书香：暖纸白+酒红，**9/10 最高**（舒适间距）
- **发现并修复**：选中**空章节**时引导卡不显示（原逻辑只在"无章节"时显示）——updateEmptyGuide 判断改为 `!ch || !hasText`（无章节或章节无内容都显示）
  - 验证：创建空章节→选中→引导卡显示 ✅；测试章节已清理
- 已重新编译部署
- **MiMo 使用经验**：prompt 用英文短句（"List 3 short observations... 50 words max"）回答稳定；中文长 prompt 容易泛化成教程或返回空

### P0-034 弹窗取消/关闭按钮失效修复（2026-07-31 23:56-24:05 轮）
- **用户报告**：有些界面的取消键点了没反应，不会退回
- **根因**：ui.js `modal()` 的按钮处理逻辑——只有 `a.onClick` 或 `a.id === 'cancel'` 才执行；**id 为 'close' 等非 cancel 且无 onClick 的按钮（如 ModelSettings.showQuickKey 的「关闭」）既不执行逻辑也不关闭弹窗**，点击无反应
- **修复**：ui.js modal 按钮逻辑改为「有 onClick 执行 onClick，否则默认 ov.remove() 关闭弹窗」
- **安全性确认**：所有 ok/确认按钮都有 onClick（不受影响）；mobile.js 的 id 是移动端面板标签非 modal actions（不受影响）
- **验证**：6 个弹窗全部实测通过——showQuickKey(close)、新建项目(cancel)、prompt、confirm、网页AI新增、模型新增，点击后弹窗均正常关闭
- 已重新编译部署
- **附**：调试浏览器（edge-debug-profile）因用户关闭原浏览器导致 CDP 断连，已重新启动带 --remote-debugging-port=18800 的调试实例

### P0-035 引导界面加退出键（2026-08-01 00:00-00:05 轮）
- **用户要求**：引导界面（空状态引导卡）应该弄个退出键
- **实现**：
  - index.html：引导卡右上角加 ✕ 按钮（.eg-close）
  - editor.js：新增 Editor.dismissGuide()（点击关闭引导 + sessionStorage 记住「已关闭」）+ guideDismissed() 查询
  - updateEmptyGuide 增加条件：用户关闭过引导则不再自动弹出（本次会话内）
  - style.css：.eg-close 圆形按钮样式，悬停变 accent 色
- **验证**：退出键点击→引导关闭+记忆保存 ✅；已关闭后空状态不再自动弹出 ✅
- 已重新编译部署

### P0-036 项目归档功能（2026-08-01 00:05-00:15 轮）
- **用户要求**：改（采纳"项目测试数据归档/隐藏"建议）
- **实现**（纯前端，localStorage 记录归档 ID，不动后端）：
  - project.js：getArchived/isArchived/archive/unarchive/renderArchived/toggleArchived
  - 右键菜单 ctxMenu 加「📦 归档（隐藏）」项（复制项目后、删除前）
  - renderList 过滤已归档项目；loadAll 时渲染归档区
  - index.html 项目列表底部加 #archivedBox 容器
  - style.css：.arch-head/.arch-item/.arch-name 样式
  - 归档当前项目时自动清空编辑器 + 显示空状态引导
- **验证**：归档「测试-角色开关」→ 列表消失 + 底部「📦 已归档（1）」+ 恢复按钮；恢复 → 项目回列表 ✅
- **坑**：Python 写 JS onclick 引号转义两次出错（node --check 报 SyntaxError），最终按行号重建行 + chr 转义解决
- 已重新编译部署

### P0-037 RAG 长篇记忆·按需注入（2026-08-01 00:14-00:30 轮）
- **用户要求**：RAG 长篇记忆，按需注入（源于 AutoGen Memory/RAG 文章启发）
- **实现**（context.go，零向量库依赖，实体关键词粗检索）：
  1. buildContext 在 smart 分层基础上追加【RAG相关记忆】段
  2. ragRetrieve：提取实体（书名号《》+人物卡名字+需求高频词）→ 扫描全书早前章节（当前章前 2 章之前）→ 命中计分排序 → 取前 3 段，每段截实体前后 60 字
  3. ragExtractTerms / snippetAround / minInt 辅助函数
- **验证**（暗恋这件难过的小事 9 章）：
  - 有 RAG（smart）：写到第 8 章提"林云/运动会"，正确注入第 4/6 章细节——"惊鸿替林云跑完八百米，冲线时脚踝扭伤"，3.8s/288字
  - 无 RAG（current）：细节全编造（"跑最后一棒"与原文冲突），2.7s/182字
  - **结论：RAG 防"设定冲突式编造"，是长篇连贯性的关键**
- **坑**：ListChapters 返回 ChapterWithVolume 非 Chapter；itoa 已在 ai_tells.go 声明（去重）；strconv 未使用；测试脚本 401（urllib 需带 UA，curl 正常）
- 已重新编译部署（server.exe PID 11868）

### P0-038 完整向量库 RAG（2026-08-01 00:18-00:40 轮）
- **用户要求**：做个向量库，把 RAG 功能全部实现
- **嵌入模型调研**：MiniMax embe-01 返回 2013 invalid params（key 无权限/参数不对）；小米 MiMo 无 embedding 端点；DeepSeek 无 embedding → **决定纯 Go 本地向量化，零外部依赖**
- **实现**：
  - `internal/infrastructure/rag/vector.go`：中文 2-gram + 单字特征、hash 到 65536 维稀疏向量、子线性归一化（类 BM25 饱和）、余弦相似度、JSON 序列化
  - `internal/infrastructure/rag/service.go`：分块（400 字/块、60 字重叠、按段落）、IndexChapters/IndexChapter 增量、Search（topK=3、相似度>0.05 过滤）、BuildContextText
  - `internal/infrastructure/database/rag.go`：rag_chunks 表 CRUD（id/project_id/chapter_id/chapter_no/title/text/vector）
  - db.go schemaDDL 加 rag_chunks 表 + 索引
  - dispatcher.go：Dispatcher.rag 字段 + NewDispatcher 注入
  - context.go：buildContext 懒建索引（无索引时全量建）+ 向量检索注入【RAG相关记忆（向量检索）】+ 原实体检索兜底【实体匹配】
- **验证**（Go 测试程序直调 rag 包）：
  - 索引：261 块（9 章），懒建正常
  - 语义检索：「物理课代表」→ 全命中第4章《林云的物理课代表》；「笔记本 借 同桌」→ 第6章"成为同桌的第二天借笔记本"；「运动会 跑步」→ 第1章"从运动会开始讲"——真语义匹配
  - 端到端生成：写第 8 章提到运动会/林云，正确注入早期细节
- **坑**：Python 测试脚本用字符串 key 而 Go 用 hash key（结果全 0 误判"检索失败"）；最终用 Go 测试程序验证
- 已重新编译部署

### P0-039 RAG 收尾：增量索引 + 前端可视化（2026-08-01 00:23-00:45 轮）
- **用户要求**：都做了吧，调用一下前端skill（加载了 anthropics frontend-design：克制、单一用途、用户视角命名）
- **① 章节保存增量索引**：
  - server.go：Server 加 rag 字段，NewServer 注入 rag.NewService(store)
  - chapters.go：UpdateChapter/CreateChapter 成功后 go 协程 IndexChapter 重建该章向量块；DeleteChapter 后 DeleteRAGChunksByChapter 清理
  - 坑：正则匹配 writeOK 块时误把钩子加进 GetChapter，精确文本替换移除
- **② 检索结果前端可视化**：
  - api/rag.go：POST /api/rag/preview（懒建索引 + Search top5 + score 计算），注册路由
  - index.html：生成区加「🧠 相关记忆」按钮 + #ragPreview 容器
  - composer.js：previewRAG() 调 API 渲染（章节/标题/相关度/片段/✕关闭）；style.css .rag-preview 样式
  - 坑：onclick 嵌套单引号 SyntaxError，转义修复
- **验证**：
  - API：需求"林云借笔记本/运动会/周然"→ 5 条全命中第6章笔记本传字情节
  - 前端：点击按钮 → 面板展开显示 5 条相关记忆（100%相关度）+ ✕关闭 ✅
- 已重新编译部署

### P0-040 前端三问题一次性修复（无障碍+生成区降噪+模式反馈）（2026-08-01 00:33-00:50 轮）
- **用户要求**：调用前端skill和识图skill，把三个问题一次性改完
- **① 无障碍增强**（app.js enhanceA11y）：
  - 纯图标按钮（无可见文字）用 title 兜底 aria-label
  - 295 个带 onclick 的 div/span 模拟按钮：补 role="button" + tabindex="0" + Enter/Space 键盘触发
  - MutationObserver 监听动态内容（弹窗/列表渲染后自动增强）
  - 修正：首次误判 2 字中文按钮（恢复/扩写等）为图标，改为「去除 emoji 后无可见文字才算图标」
  - 验证：修正标准下缺失 aria-label 且无 title 的按钮 = 0；295 div 全部 role=button ✅
- **② 生成区降噪**（index.html + composer.js）：
  - 主行只留：输入框 + 大纲 + ✨生成 + ⚙️选项
  - 批量建章/模板/目标字数/相关记忆收进「⚙️ 选项」折叠行（toggleGenOptions）
  - 验证：点击展开 flex / 再点收起 none ✅
- **③ 模式切换反馈**（composer.js onModeChange）：
  - 切换模式时 toast「已切换：快速草稿：跳过 Thinker+Verifier…」
  - 验证：切换 draft 后 toast 正常弹出 ✅
- 已重新编译部署

### P0-041 前端卡顿性能优化（2026-08-01 00:37-00:55 轮）
- **用户反馈**：前端有点卡 → 调用前端skill（performance-optimization：先测量再优化）+ 实测定位
- **测量**（CDP Performance API）：
  - 页面加载 445ms、无长任务、renderTree 1ms、字数统计 16ms/100次、save HTTP 4ms、Tiptap 72 节点——静态性能全优
  - **定位到卡顿源：SSE 流式生成时的 O(n²) 操作**
- **根因**：
  1. `appendStream` 每个 token 到达都调 `ChapterUI.renderTree()` 全量重建章节树
  2. `Array.from(streamText)` 每次对累计全量文本转字符数组
  3. 每次 token 调 `updateWordCount()`（Tiptap getText 全量遍历）+ `refreshPreview()`
- **修复**（节流）：
  - editor.js appendStream：renderTree 改 500ms 节流；updateWordCount/refreshPreview 换节流版
  - sse.js token case：字数进度条改 300ms 节流 + 用 length 代替 Array.from
  - 坑：宽松正则替换截断 break 语句致 sse.js 语法错误，删残留块修复
- **验证**：100 次 appendStream 同步开销 ~30ms（每次 0.3ms）；生成 569字/7s/401 token 事件正常
- 已重新编译部署

### P0-042 上线前修复：3 个 P2 + Eino 日志（2026-08-01 00:43-01:00 轮）
- **依据**：最新测试报告（2026-08-01）结论"满足上线标准，建议修 3 个 P2 后发布"
- **Q1 双击新建项目叠弹窗**：ui.js modal 加单例守卫（打开前清空 modalRoot）→ 验证 3 连击仅 1 弹窗 ✅
- **Q2 未选项目人物卡静默失败**：pages-characters.js + pages-world.js save() 加 toast「请先在左侧选择项目」→ 验证 toast 弹出 ✅
- **Q3 模型面板首次打开卡加载中**：mobile.js openModelPanel 在 loadAll 完成后检查占位符并强制 ModelSettings.render 再复制 → 验证内容正常渲染 ✅
- **P3-3 Eino 启动误导日志**：根因=startEinoProcess 以嵌套相对路径 ai-novel-main/server.exe 自启动（cwd 已是根目录），cmd.Start() 必然失败报错；根治=禁用 Eino 子进程（主线已含完整功能）→ 验证重启无 Eino 日志 ✅
- 已重新编译部署

### P0-043 工具箱可用性实测（2026-08-01 00:47-00:55 轮）
- **用户问**：工具箱能用吗
- **实测**（浏览器 CDP + 真实调用）：
  - 39 个工具函数全部存在，0 死链（cleanAIFiller 磁盘上已是 Editor.cleanAIFiller，非 Tools.cleanAIFiller——旧版已修）
  - 校对 proofread（/api/tools/execute）：真实 AI 精确揪出"总的来说"缺逗号 ✅
  - 字数统计 count：返回 10 正确 ✅
  - AI 味检测 aiTells：规则检测正常 ✅
- **结论：工具箱完全可用**（30+ 工具五分类）
- 测试脚本教训：API.proofread 不存在，正确端点是 API.post('/api/tools/execute', {tool: 'proofread'})

### P0-044 上线包 + v1.2 项全部落地（2026-08-01 00:51-01:05 轮）
- **用户要求**：都做了吧（一键启动包 + P3-4 + 文生图封面 + N4 + 增量索引）
- **① 一键启动包**：
  - 启动.bat（GBK 编码防乱码）：检查 server.exe → 防重复启动（端口检测直接开浏览器）→ 启动 + 等待就绪（20s 轮询）→ 自动开浏览器
  - 使用说明.md：三步开始（启动/配Key/创作）+ 常用功能表 + 高级 + FAQ
- **② P3-4 文案修复**：showQuickKey 不再死盯 deepseek-v4-pro，改为查找任意有 key 的模型；未设置时提示「请在下方面板配置」
- **③ 文生图封面**：确认已实现（covers.go HandleGenerateCover 用 Pollinations.ai 免费 API，前端🎨生成封面按钮）
- **④ N4 判定细化**：detectPipeline art 触发词补充 韵味/意境/美感/画面感/细腻/优美/含蓄/留白/张力/慢镜头/细节描写/环境描写/心理描写/文风/文采
- **⑤ 增量索引恢复**：发现 P0-039 的 Update/Create 钩子在后续 fix 中被误删（只剩 Delete），已恢复 IndexChapter×2 + Delete×1
- 全部编译通过 + 部署

### P0-045 v1.2 发布：git 提交+上传 GitHub / 启动模拟 / 使用说明HTML（2026-08-01 00:55-01:15 轮）
- **任务一：v1.2 保存到 git + 上传 GitHub**：
  - 发现仓库结构问题：git 同时跟踪根目录（真实代码）和旧 ai-novel-main/ 子目录（7月28日过时副本 97 文件）
  - 用户确认后：git rm -r --cached ai-novel-main 移除旧跟踪；更新 .gitignore（排除第三方参考目录/备份/测试脚本/报告/zip）
  - 提交 v1.2（多Agent+RAG+前端优化+上线修复），推送 origin master（94458ef..ab375fb）✅
  - 网络坑：github.com 直连超时但 api.github.com 通，重试后推送成功
- **任务二：模拟一键启动**：
  - 实测发现启动.bat 中文乱码+命令解析错误——根因：LF 换行（Windows 批处理必须 CRLF）+ chcp 65001 与 GBK 内容冲突
  - 修复：CRLF+GBK 编码、删 chcp 行 → 模拟双击验证：全新启动✅ 防重复启动✅ 中文显示✅ 服务6秒就绪✅
- **任务三：使用说明 HTML 版**：
  - md2html.py 极简 markdown 转换器 → 使用说明.html（自包含单文件，纸墨书香风格样式）
  - 结构验证：doctype/表格/列表/代码块全齐 ✅
- 全部推送 GitHub（5b4cd09），本地与远程同步
