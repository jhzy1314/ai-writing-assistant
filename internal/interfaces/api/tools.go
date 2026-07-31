package api

import (
	"context"
	"fmt"
	"net/http"
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
func (s *Server) HandleToolExecute(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Tool    string `json:"tool"`
		Content string `json:"content"`
		Params  struct {
			From        string `json:"from"`
			To          string `json:"to"`
			Instruction string `json:"instruction"`
		} `json:"params"`
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

	// 调用 Helper 角色执行工具任务
	result, modelName, err := s.callHelperTool(ctx, userPrompt)
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

// callHelperTool 调用 Helper 角色非流式生成，含备用模型降级
func (s *Server) callHelperTool(ctx context.Context, userPrompt string) (string, string, error) {
	agent := roles.NewRoleAgent(llm.RoleHelper, "light")
	adapters, err := s.registry.AdaptersForRole(ctx, llm.RoleHelper)
	if err != nil {
		return "", "", fmt.Errorf("Helper 无可用模型: %w", err)
	}
	var lastErr error
	for _, ad := range adapters {
		start := time.Now()
		text, usage, gErr := agent.Generate(ctx, ad, userPrompt)
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
		return fmt.Sprintf(`你是一个小说设定提取助手。请仔细阅读下面的小说正文，从中提取所有能找到的人物信息和世界观信息。

最重要规则：不要用"未知"占位！如果正文中没有对应的信息，那一行就不要写。只写正文里确实有的内容。

小说正文：
---
%s
---

请按以下格式输出：

【人物卡列表】
姓名：<角色名>
<如果文中提到了性别，加一行>性别：<男/女>
<如果文中提到了外貌特征，加一行>外貌：<描述>
<如果文中提到了性格，加一行>性格：<描述>
<如果文中提到了身份背景，加一行>背景：<描述>
<如果有行为原则描写，加一行>行为底线：<描述>
<如果有与其他角色关系的描写，加一行>人际关系：<描述>
---
（下一个角色，用 --- 分隔）

【世界观设定】
标题：<从文中提取的世界观名称或直接写"当前设定">
<如果文中提到了时代>时代背景：<古代/现代/修仙时代等>
<如果有修炼体系描写>力量体系：<描述>
<如果有势力描写>势力分布：<描述>
<如果有地点描写>地理设定：<描述>
<如果有世界规则描写>世界规则：<描述>
---
（下一个世界设定，用 --- 分隔）

请务必从正文中提取真实信息。例如给出了"林风是青云门弟子"、"师父张玄"、"红发男子来攻打"等事实，就要如实地列出。`, content)

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
		lengthRule = "题材控制在 2-20 个字，用网文常见分类（都市异能/玄幻修仙/科幻末世/古代言情等），可写复合题材，单行输出。"
	case "selling":
		lengthRule = "核心卖点控制在 5-50 个字，说清最吸引人的 1-3 个点（废柴逆袭打脸、悬疑烧脑反转、爽点密度高等），单行输出。"
	case "hero":
		lengthRule = "主角设定控制在 20-120 个字：姓名（可给）、身份/职业、核心特质、性格、潜在背景伏笔，单行输出。"
	case "world":
		lengthRule = "世界观控制在 20-150 个字：时代背景、核心规则/设定、主要势力或格局，单行输出。"
	case "power":
		lengthRule = "力量体系控制在 20-100 个字：体系名称、等级划分（如 E→D→C→B→A→S）、获取方式，单行输出。"
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
