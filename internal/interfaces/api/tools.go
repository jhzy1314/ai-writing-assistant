package api

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/ai-novel/studio/internal/domain/roles"
	"github.com/ai-novel/studio/internal/infrastructure/database"
	"github.com/ai-novel/studio/internal/infrastructure/llm"
)

// HandleToolExecute POST /api/tools/execute
// 统一工具执行入口：接收 tool 名称 + 输入文本 + 可选参数，调用 Helper 角色执行并返回结果。
// 请求体：{ "tool": "clean|sort|extract|convert|count", "content": "...", "params": {} }
// 响应：  { "result": "...", "tool": "...", "model": "..." }
// HandleAnalyzeFile POST /api/tools/analyze-file
// 上传本地书籍文件（txt/md/epub/docx/html）→ 解析全文 → 异步拆书任务（全文分块通读，不截断）→ 返回 task_id
func (s *Server) HandleAnalyzeFile(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(64 << 20); err != nil { // 64MB 上限
		writeError(w, http.StatusBadRequest, "文件过大或格式错误: "+err.Error())
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "未找到文件字段")
		return
	}
	defer file.Close()
	modelName := r.FormValue("model") // 可选：指定拆书模型，空则用 helper 角色绑定
	// 上传的临时文件在请求结束后会被清理，先把内容读进内存再开后台任务
	data, err := io.ReadAll(file)
	if err != nil {
		writeError(w, http.StatusBadRequest, "读取上传文件失败: "+err.Error())
		return
	}

	taskID := analyzeTasks.start(func(t *analyzeTask) {
		t.set("解析文件…", 5)
		text, err := extractText(header.Filename, bytes.NewReader(data))
		if err != nil {
			t.finish(false, "", "", "解析文件失败: "+err.Error(), 0, 0)
			return
		}
		if strings.TrimSpace(text) == "" {
			t.finish(false, "", "", "未能从文件中提取到文本（PDF 暂不支持，请转成 txt/epub）", 0, 0)
			return
		}
		// 全文通读，不截断（固定 8 万字/块，不按 context_limit 配置限制）
		bookName := strings.TrimSuffix(filepath.Base(header.Filename), filepath.Ext(header.Filename))
		s.runBookAnalyze(t, text, modelName, bookName)
	})
	writeOK(w, map[string]interface{}{"task_id": taskID})
}

func (s *Server) HandleToolExecute(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Tool    string `json:"tool"`
		Content string `json:"content"`
		Params  struct {
			From        string `json:"from"`
			To          string `json:"to"`
			Instruction string `json:"instruction"`
		} `json:"params"`
		Model string `json:"model"` // 可选：指定工具使用的模型（如 summarize 用快速模型）
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "请求体解析失败: "+err.Error())
		return
	}
	if strings.TrimSpace(req.Content) == "" {
		writeError(w, http.StatusBadRequest, "content 不能为空")
		return
	}

	// 配额/限流校验
	guard, err := s.limiter.AllowRequest(r.Context(), clientIP(r))
	if err != nil {
		writeError(w, http.StatusTooManyRequests, err.Error())
		return
	}
	defer guard.Release()

	// 构建工具提示词
	userPrompt := buildToolPrompt(req.Tool, req.Content, req.Params.From, req.Params.To, req.Params.Instruction)
	if userPrompt == "" {
		writeError(w, http.StatusBadRequest, "不支持的工具类型: "+req.Tool)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), reqTimeout)
	defer cancel()

	// 调用 Helper 角色执行工具任务（summarize 等工具可指定快速模型；提取类工具关思考加速）
	result, modelName, err := s.callHelperTool(ctx, userPrompt, req.Model, req.Tool)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "工具执行失败: "+err.Error())
		return
	}

	writeOK(w, map[string]interface{}{
		"tool":   req.Tool,
		"result": result,
		"model":  modelName,
	})
}

// callHelperTool 调用 Helper 角色非流式生成，含备用模型降级；preferredModel 非空时优先使用该模型（失败再回退角色绑定）
// 提取/总结类工具（summarize/extract_*/count 等）自动关闭深度思考以提速
func (s *Server) callHelperTool(ctx context.Context, userPrompt, preferredModel, tool string) (string, string, error) {
	return s.callRoleTool(ctx, llm.RoleHelper, userPrompt, preferredModel, tool)
}

// callRoleTool 调用指定角色非流式生成，含备用模型降级；preferredModel 非空时优先使用该模型（失败再回退角色绑定）
func (s *Server) callRoleTool(ctx context.Context, role llm.Role, userPrompt, preferredModel, tool string) (string, string, error) {
	agent := roles.NewRoleAgent(role, "light")
	var adapters []llm.ModelAdapter
	// 指定模型优先（如拆书选免费模型），失败后回退角色绑定模型
	if preferredModel != "" {
		if a, err := s.registry.AdapterByName(ctx, preferredModel); err == nil {
			adapters = append(adapters, a)
		}
	}
	roleAdapters, roleErr := s.registry.AdaptersForRole(ctx, role)
	if roleErr != nil && len(adapters) == 0 {
		return "", "", fmt.Errorf("%s 无可用模型: %w", role, roleErr)
	}
	for _, ra := range roleAdapters {
		dup := false
		for _, ad := range adapters {
			if ad.Name() == ra.Name() {
				dup = true
				break
			}
		}
		if !dup {
			adapters = append(adapters, ra)
		}
	}
	// 提取/总结类工具关闭深度思考（简单任务不需要推理，提速明显）
	noThink := tool == "summarize" || tool == "extract_characters" || tool == "extract_worldsetting" ||
		tool == "count" || tool == "clean" || tool == "convert" ||
		tool == "book_analyze_pass" || tool == "book_analyze_merge" // 拆书分块特征提取/压缩：事实抽取，关思考提速
	if noThink {
		for _, ad := range adapters {
			if t, ok := ad.(interface{ SetThinking(bool) }); ok {
				t.SetThinking(false)
			}
		}
	}
	var lastErr error
	for _, ad := range adapters {
		start := time.Now()
		text, usage, gErr := s.generateWithRetry(ctx, agent, ad, userPrompt)
		durMs := time.Since(start).Milliseconds()
		if gErr == nil {
			_ = s.store.InsertLog(ctx, database.GenerationLog{
				Role: string(llm.RoleHelper), ModelName: ad.Name(),
				PromptTokens: usage.PromptTokens, CompletionTokens: usage.CompletionTokens,
				DurationMs: int(durMs), Status: "ok",
			})
			_ = s.store.IncrUsage(ctx, ad.Name(), 1, usage.Total())
			return text, ad.Name(), nil
		}
		_ = s.store.InsertLog(ctx, database.GenerationLog{
			Role: string(llm.RoleHelper), ModelName: ad.Name(),
			DurationMs: int(durMs), Status: "error", ErrorMsg: gErr.Error(),
		})
		lastErr = gErr
	}
	return "", "", fmt.Errorf("Helper 全部模型调用失败: %w", lastErr)
}

// generateWithRetry 单模型生成：429/503 等瞬时性错误（免费模型限流、服务繁忙）按退避重试，
// 避免瞬时抖动导致整次拆书/工具调用失败；其他错误立即返回交给模型降级。
func (s *Server) generateWithRetry(ctx context.Context, agent *roles.RoleAgent, ad llm.ModelAdapter, prompt string) (string, llm.Usage, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		text, usage, err := agent.Generate(ctx, ad, prompt)
		if err == nil {
			return text, usage, nil
		}
		lastErr = err
		if isRetryableError(err) && attempt < 2 {
			select {
			case <-time.After(time.Duration(2*(attempt+1)) * time.Second):
			case <-ctx.Done():
				return "", llm.Usage{}, ctx.Err()
			}
			continue
		}
		break
	}
	return "", llm.Usage{}, lastErr
}

// isRetryableError 429（限流/请求量过大）与 503（服务繁忙）属于瞬时故障，值得重试
func isRetryableError(err error) bool {
	s := err.Error()
	return strings.Contains(s, "429") || strings.Contains(s, "503") ||
		strings.Contains(s, "Too Many Requests") || strings.Contains(s, "Service Unavailable") ||
		strings.Contains(s, "busy") || strings.Contains(s, "rate limit") ||
		strings.Contains(s, "请求量过大") || strings.Contains(s, "限流") || strings.Contains(s, "繁忙")
}

// buildToolPrompt 按工具类型构建用户提示词
func buildToolPrompt(tool, content, from, to, instruction string) string {
	switch tool {
	case "clean":
		return fmt.Sprintf(`【工具：文本清洗】
请去除以下文本中的AI多余话术、代码围栏标记、解释性段落、多余开场白和结尾语，仅保留纯净正文。
若文本本身已是洁净正文，则原样返回。

待清洗文本：
%s`, content)

	case "convert":
		if from == "" {
			from = "markdown"
		}
		if to == "" {
			to = "纯文本"
		}
		return fmt.Sprintf(`【工具：格式转换】
将以下文本从 %s 格式转换为 %s 格式。
%s

待转换文本：
%s`, from, to, instruction, content)

	case "sort":
		return fmt.Sprintf(`【工具：章节排序】
以下是无序的章节列表，每章以"---"分隔，包含章节标题和正文片段。
请分析各章的剧情脉络、时间线、逻辑顺序，将章节按正确阅读顺序重新排列。
输出格式：
1. 排序后的章节标题序列（一行一个）
2. 简要排序理由

待排序章节：
%s`, content)

	case "extract":
		return fmt.Sprintf(`【工具：素材提取】
从以下文本中提取核心设定、人物信息、世界观要素、关键事件，输出结构化摘要。
%s

请按以下结构输出：
- 核心设定：
- 人物信息：
- 世界观要素：
- 关键事件：`, content)

	case "count":
		return fmt.Sprintf(`【工具：字数统计】
统计以下文本的精确字数（中文字符按1字计，英文单词按实际单词数计，标点不计入），只输出一个数字。

待统计文本：
%s`, content)

	case "outline":
		return fmt.Sprintf(`你是一个网文小说大纲策划师。请根据用户提供的创作需求（题材、核心设定、灵感等），生成一份专业、详细、可直接用于创作的小说大纲。

输出格式（严格按以下结构）：
【书名】
【题材】
【一句话梗概】
【核心设定】\n- 世界观：\n- 力量体系：\n- 核心矛盾：
【主要人物】\n- 主角：姓名/身份/性格/成长弧线\n- 配角：姓名/身份/作用
【主线剧情】\n1. 开篇（第1-3章）……\n2. 发展（中期）……\n3. 高潮……\n4. 结局……
【分卷规划】\n- 第一卷：标题/内容概要/目标字数\n- 第二卷：……
【卖点与爽点设计】

用户创作需求：
%s`, content)

	case "worldbuild":
		return fmt.Sprintf(`你是一个世界设定架构师。请根据用户提供的题材或灵感，生成一套完整、自洽、可直接用于创作的世界观设定。

输出格式（严格按以下结构）：
【世界名称】
【时代背景】
【地理设定】\n- 主要区域：\n- 特色地点：
【势力分布】\n- 势力一：名称/立场/目标\n- 势力二：……
【力量体系】（奇幻/仙侠类必填，都市类写“社会规则”）\n- 体系框架：\n- 等级划分：\n- 获取方式：
【文化风俗】\n【世界规则与限制】\n【与主角的关系】

用户题材/灵感：
%s`, content)

	case "namegen":
		return fmt.Sprintf(`你是一个小说人物起名专家。请根据用户提供的角色设定（性别、性格、时代/题材、身份等），生成 12 个贴合人设、不落俗套的名字，并标注每个名字的含义和适合的角色类型。

输出格式：
1. 名字（性别）—— 寓意：…… / 适合角色：……
2. ……
（共12个）

最后附一行【起名建议】：根据人设给出 2-3 条起名思路。

角色设定：
%s`, content)

	case "plotcheck":
		return fmt.Sprintf(`你是一个小说逻辑审稿专家。请仔细阅读以下小说正文（可能包含多章节），检查并指出：
1. 伏笔：埋下但未回收的伏笔
2. 逻辑矛盾：前后矛盾的情节、设定冲突、时间线错误
3. 人物一致性：人物性格/能力/记忆前后不一致的地方
4. 设定遗忘：前面出现过的设定在后面被遗忘

输出格式：
【伏笔追踪】\n- 第X章埋下的「xxx」伏笔，是否已回收？
【逻辑矛盾】\n- 位置/问题描述
【人物一致性】\n- 位置/问题描述
【修改建议】\n- 针对每个问题的具体修改建议

若未发现问题，请明确输出「未发现明显问题」。

小说正文：
%s`, content)

	case "roleplay":
		return fmt.Sprintf(`你是一个角色互动模拟器。用户会提供两个（或多个）角色的人物卡设定和一个互动场景。请模拟他们之间自然、符合人设的对话互动，展现出角色性格的碰撞与化学反应，帮助作者发现人物互动的张力。

要求：\n- 严格基于角色卡的性格、身份、说话习惯\n- 对话要体现潜台词（言外之意）\n- 在对话后附一段【互动洞察】：这段互动揭示了角色的哪些性格侧面，对剧情有什么启发

角色卡与场景：
%s`, content)

	case "branch":
		return fmt.Sprintf(`你是一个剧情分支推演器。用户会提供当前剧情的关键节点。请生成 4 个不同方向的剧情后续发展选项，帮助作者突破卡文。

输出格式：
【选项A】标题\n- 走向描述（200字内）：\n- 风险/收益：\n- 适合：
【选项B】标题\n……
【选项C】……
【选项D】……

最后附一行【创作启发】：这4个方向分别能带来什么戏剧效果。

当前剧情节点：
%s`, content)

	case "summarize":
		return fmt.Sprintf(`你是一个小说设定提取助手。请仔细阅读下面的小说正文，从中提取人物信息和世界观信息。

【提速与质量规则】
1. 只提取主要角色：出场多、有名字、对剧情重要的人物，最多 12 个。龙套/路人不要列。
2. 同名角色只输出一次，禁止重复（例如"天一"只出现一次，不要输出"天一天一"）。
3. 每个字段用 1-2 句话精炼描述，不要展开长篇。
3b. 每个角色至少输出 外貌、性格、背景 三个字段：正文没有明确描写时，根据上下文合理推断补齐（性格必须推断，这是人设核心）；外貌正文没提才可省略。
4. 不要用"未知"占位！正文中没有的信息，那一行就不写。只写正文里确实有的内容。
5. 无论题材都要提取世界观：现实题材也要提取 时代背景、主要地点（学校/城市/场景）、社会环境、核心设定（标志性物品、叙事结构、独特规则等）；不要因为没有玄幻设定就跳过世界观部分。
6. 时间线区分：当人物的身份、关系、背景在不同阶段发生变化时（例如"以前是陈思言的同桌，现在和惊鸿同桌"），**必须标注时间阶段**，用「现为…」「曾为…（后来…）」等方式分开说明，绝对禁止把不同阶段的身份/关系并列写成同时成立。
7. 关系只写当前正文里已经发生或明确提到的；正文尚未发生的关系不要写。
8. 世界观演化时间线：设定随剧情变化时同样标注阶段（「现为…」「曾为…」），与人物卡规则一致。
9. 只提取设定本身（时代/地点/规则/标志物），不要把具体剧情事件、人物互动写成世界观规则。

小说正文：
---
---
%s
---

请按以下格式输出：

【人物卡列表】
姓名：<角色名>
<如果文中提到了性别，加一行>性别：<男/女>
<如果文中提到了外貌特征，加一行>外貌：<描述>
<如果文中提到了性格，加一行>性格：<描述>
<如果文中提到了身份背景，加一行>背景：<描述，身份/关系有阶段变化时务必标注时间线，如「现为惊鸿的同桌；曾为陈思言的同桌」>
<如果有行为原则描写，加一行>行为底线：<描述>
<如果有与其他角色关系的描写，加一行>人际关系：<描述，同样注意时间线：区分「现在」与「以前」的关系>
---
（下一个角色，用 --- 分隔）

【世界观设定】（每条设定之间用 --- 分隔；按实际内容输出，字段没有就省略）
标题：<设定条目名，要具体有辨识度（如「镇海中学·高二3班」「红绳手链的由来」）；现实文用地点/时代/核心意象命名，不要只写「当前设定」>
<如果文中提到了时代/年代>时代背景：<描述>
<如果文中提到了地点>主要地点：<学校/城市/场景等，含环境氛围>
<如果有标志性物品/核心意象>核心设定：<这本书的标志性元素：特殊物品（如日记本、红绳）、叙事结构（如回忆视角、书信体）、独特规则等>
<如果有社会环境>社会环境：<时代氛围、校园风气、家庭背景等>
<如果有修炼体系描写>力量体系：<描述>（仅玄幻/仙侠/奇幻类）
<如果有势力描写>势力分布：<描述>
<如果有世界规则描写>世界规则：<描述>
<如果设定随剧情演化>演化标注：<用「现为…」「曾为…」标注时间线，如「现为高二3班；曾为高一2班」>
---
（下一个世界设定，用 --- 分隔）

请务必从正文中提取真实信息。例如给出了"林风是青云门弟子"、"师父张玄"、"红发男子来攻打"等事实，就要如实地列出。`, content)

	case "book_analyze":
		return fmt.Sprintf(`你是资深小说拆书编辑。用户提供一本书（或连续章节）的文本。你的任务：通读后，把这本书真正值得借鉴的东西拆出来——不是流水账，而是**精华提炼**。

输出四部分，严格按以下格式：

【标志性片段】（3-5 段，务必覆盖书中**不同类型/场景**的片段，如开篇氛围、高潮冲突、日常对话、情绪转折、场景白描等——同一本书里这些段落文风各不相同，都要挑出来）
1. <片段标题（如：开篇的黄昏教室 / 高潮的走廊对峙 / 日常的课间对话）>
来源：<第几章/大致位置>
片段正文：<原文摘录，150-300 字，保留原文文字与节奏>
风格要点：<这段为什么值得学：句式/节奏/视角/用词特征，1-2 句>

2. <…>
…

【主要角色】（列出本书**全部**主要角色与重要配角——群像/长篇小说通常 8-20 个，按重要性排序；只漏掉纯龙套路人）
姓名：<角色名>
定位：<主角/重要配角等>
性格：<1-2 句>
塑造手法：<作者如何写出这个人物：通过什么细节/行为/对话/反差，1-2 句，这是最值得借鉴的>

（每个角色重复以上四行，用 --- 分隔）

【世界观】
核心设定：<这本书最独特的世界观/设定核心，1-2 句>
构建手法：<作者如何让设定可信/有趣：从什么细节切入、如何层层展开，2-3 句>

【伏笔设计】
1. <伏笔内容> | 埋设位置：<第几章/场景> | 类型：<物件/对话/细节/意象> | 作用：<回收后如何影响剧情>
2. <…>
…

要求：
- 全部基于原文，不要编造；标志性片段必须原文摘录。
- 聚焦"值得借鉴的方法"，而不是剧情复述。

待分析书籍文本：
%s`, content)

	case "world_enhance":
		return fmt.Sprintf(`你是小说世界观设计顾问。用户提供当前已有的世界观设定（可能很少或较单薄）和作品风格参考。你的任务：找出世界观值得丰富的地方，给出具体可执行的丰富建议和**可直接保存**的预览样本。

要求：
1. 给出 3-5 个丰富方向，覆盖不同类型（核心意象深化、地点细节、时代氛围、社会背景、独特规则、配角生态、前史后史等），与现实文或作品的实际风格匹配，不要硬套玄幻/仙侠设定。
2. 每个方向包含：【理由】（为什么值得丰富、能带来什么，1-2 句）+【预览样本】（一段可直接保存为世界观条目的完整描述，100-200 字，文风与作品一致）。
3. 预览样本必须是可直接采用的内容：具体、有细节、可读性强，不要占位符、不要「例如」、不要省略号敷衍；段落以条目形式（标题行 + 描述），适合作为世界观卡片的「设定内容」。
4. 不要与用户已有世界观重复或雷同。

当前已有世界观：
---
%s
---

作品风格参考：
%s

输出格式（严格按此）：
【方向1】<方向标题>
【理由】<…>
【预览样本】<标题行>\n<描述文字>
【方向2】…
【方向3】…`, content, instruction)

	case "fieldgen":
		// 专业模式逐字段 AI 提示：instruction 传字段 key（bookname/genre/selling/hero/world/power/plot/volumes）
		field := instruction
		if field == "" {
			field = "bookname"
		}
		return buildFieldPrompt(field, content)

	case "proofread":
		return fmt.Sprintf(`【工具：文字校对】
你是专业中文校对助手。请仔细检查以下文本中的错别字、标点符号错误、的地得混用、用词不当等问题。

请按以下格式逐条列出发现的问题：
1. 位置：第X句（引用原文）
2. 问题类型：错别字/标点/的地得/用词
3. 建议修改：<具体修改建议>

校对规则：
- 「的地得」严格区分：名词前用「的」、动词前用「地」、补语前用「得」
- 标点符号：中文用全角，、。！“”『』；英文用半角.,!?
- 常见错别字：在/再、做/作、即/既、已/己、象/像/相
- 不要改动原文风格和剧情，仅纠正语言规范问题
		- 疑似歧义（如「在那做」可能为「在那里做」）也应列出并给出建议，不要因歧义而跳过。
- 如果没有发现问题，输出"未发现明显文字问题，梳理顺畅。"
- 使用精准的中文进行校对

待校对文本：
%s`, content)

	default:
		return ""
	}
}

// buildFieldPrompt 专业模式逐字段 AI 提示：严格限定只输出该字段内容
func buildFieldPrompt(field, content string) string {
	fieldNames := map[string]string{
		"bookname": "书名",
		"genre":    "题材",
		"selling":  "核心卖点",
		"hero":     "主角设定",
		"world":    "世界观/环境设定",
		"power":    "力量/等级体系",
		"plot":     "主线剧情概述",
		"volumes":  "分卷规划",
	}
	fieldName := fieldNames[field]
	if fieldName == "" {
		fieldName = field
	}
	lengthRule := "内容精炼，单行输出，不超过 100 字。"
	switch field {
	case "bookname":
		lengthRule = "书名控制在 2-12 个字，要有记忆点和网文感，避免生僻字堆砌，单行输出。"
	case "genre":
		lengthRule = "题材控制在 2-20 个字，用网文常见分类（校园青春/都市生活/都市异能/玄幻修仙/科幻末世/古代言情等），可写复合题材，单行输出。"
	case "selling":
		lengthRule = "核心卖点控制在 5-50 个字，说清最吸引人的 1-3 个点（青春成长、学霸逆袭、悬疑烧脑反转、爽点密度高等），单行输出。"
	case "hero":
		lengthRule = "主角设定控制在 20-120 个字：姓名（可给）、身份/职业（学生/职场人/修行者等均可）、核心特质、性格、潜在背景伏笔，单行输出。"
	case "world":
		lengthRule = "世界观控制在 20-150 个字：时代背景（校园/都市/异界等均可）、核心规则/设定、主要势力或格局，单行输出。"
	case "power":
		lengthRule = "力量体系控制在 20-100 个字：体系名称、等级划分（如 E→D→C→B→A→S）、获取方式；现代/校园题材没有超自然体系时，改为描述该作品独特的能力/特长/学业规则设定，单行输出。"
	case "plot":
		lengthRule = "主线剧情控制在 50-300 个字，按「开局→发展→高潮→结局」四段式概述，每段一行（最多 4 行）。"
	case "volumes":
		lengthRule = "分卷规划控制在 50-300 个字，每卷一行，格式：第X卷《卷名》章节范围：内容概要（最多 5 行）。"
	}
	return fmt.Sprintf(`你是一个网文创作辅助 AI。用户正在填写小说设定表单中的「%s」字段，请你只负责生成这一项内容。

【严格输出要求】
1. 只输出「%s」字段的内容本身，直接可用于填入表单——不要任何开场白、解释、前后缀、序号、标题、引号、星号或 Markdown 标记。
2. 严禁输出「好的」「以下是」「建议如下」等任何多余文字；严禁列出多个候选方案让用户挑选——只能输出唯一确定的最终内容。
3. 严禁输出与字段无关的信息（不要输出剧情大纲、不要输出其他字段的内容）。
4. 内容必须严格基于用户提供的创作需求；需求信息不足时，按网文常见套路合理补全，但不得与需求中已有的设定冲突。
5. 长度要求：%s
6. 全部用简体中文输出。

用户创作需求/上下文：
%s`, fieldName, fieldName, lengthRule, content)
}
