package pipeline

import (
	"fmt"
	"strings"
)

// writeWebRef 追加联网参考信息块（仅当用户开启联网搜索且检索到内容时注入，附严格使用规则）
func writeWebRef(b *strings.Builder, req GenerateRequest) {
	if strings.TrimSpace(req.WebInfo) == "" {
		return
	}
	b.WriteString("【联网参考信息】（系统自动检索，内容可能不准确、过时或与创作无关）\n")
	b.WriteString(req.WebInfo)
	b.WriteString("\n\n【联网信息使用规则（必须严格遵守）】\n")
	b.WriteString("1. 仅作背景知识参考（如真实历史/地理/科学常识/职业细节/流行文化），用于增强细节真实感与准确性；\n")
	b.WriteString("2. 严禁大段复制或改写检索结果原文，严禁照搬他人作品或受版权保护的内容；\n")
	b.WriteString("3. 若检索信息与用户设定冲突，一律以用户设定为准；\n")
	b.WriteString("4. 不得声称自己实时搜索或编造来源，如需提及可用「据公开资料」表述；\n")
	b.WriteString("5. 与创作无关或无法核实的内容直接忽略，不要写入正文。\n\n")
}

// buildThinkerUserPrompt 构造规划师的用户提示词
// 注意：稳定上下文（人物卡/世界观，经 AssembledText）放在最前以命中提供商前缀缓存；
// 每次变化的【用户创作需求】置于其后（见 AssembledText 注释）。
func buildThinkerUserPrompt(req GenerateRequest, bundle ContextBundle, pl PipelineName) string {
	var b strings.Builder
	if bundle.HasContext() {
		b.WriteString(bundle.AssembledText())
		b.WriteString("\n")
	}
	b.WriteString("【用户创作需求】\n")
	b.WriteString(req.UserDemand)
	b.WriteString("\n\n")
	// 用户手填大纲：规划师必须忠实保留用户全部要点，仅在其基础上补充细节与结构（不得另起炉灶）
	if strings.TrimSpace(req.Outline) != "" {
		b.WriteString("【用户已提供的大纲（最高优先级，必须忠实保留）】\n")
		b.WriteString("以下是用户亲手填写的大纲。你必须：\n")
		b.WriteString("1. 完整保留用户大纲中的全部要点、剧情节点、人物与场景安排，不得删改、不得偏离；\n")
		b.WriteString("2. 仅在其基础上补充可执行的细节：每个节点的场景描写要点、角色行为动机、对话节奏、伏笔埋设；\n")
		b.WriteString("3. 若补充内容与用户大纲冲突，一律以用户大纲为准；\n")
		b.WriteString("4. 输出为可直接指导创作的框架，用户大纲的节点必须全部出现在框架中。\n\n")
		b.WriteString("【用户大纲】\n")
		b.WriteString(req.Outline)
		b.WriteString("\n\n")
	}
	// 人设/世界观/大纲约束已上移至 ThinkerPrompt 系统提示词（固定前缀，命中提供商缓存）
	if req.SelectedText != "" {
		b.WriteString("【编辑器选中文字】\n")
		b.WriteString(req.SelectedText)
		b.WriteString("\n\n")
	}
	// 目标字数与分段指令
	if req.TargetWord > 0 {
		b.WriteString(fmt.Sprintf("【目标字数】%d 字（请务必达到此字数，不可大幅缩减）\n", req.TargetWord))
		if needsSegmentation(req) {
			n := numSegments(req)
			b.WriteString(fmt.Sprintf("由于目标字数较大，需分 %d 段撰写（每段约 %d 字）。", n, segmentSize))
			b.WriteString("请在框架中明确标注「第1段」「第2段」…各自的核心内容与剧情节点，供创作者分段撰写。\n")
		}
		// 字数预估：每个剧情节点附预估字数
		b.WriteString("【字数预估要求】请在每个剧情节点后以「预估字数：XXX字」标注该节点建议的字数范围。\n")
		b.WriteString("在框架末尾以「全文建议字数汇总：XXX字」汇总所有节点建议字数的总和。\n")
	}
	// 剧情一致性硬性要求：规划师必须吃透前文，禁止重复/冲突
	b.WriteString("【剧情一致性硬性要求】\n")
	b.WriteString("1. 上方【历史前文】已交代过的人物关系、身份、事件、信息（谁认识谁、谁是什么关系、发生过什么），一律视为已知事实，严禁在框架中重复介绍、再次揭示或装作不知道；\n")
	b.WriteString("2. 每个剧情节点先对照【历史前文】检查：是否与已发生的事实冲突？是否重复已交代的信息？发现冲突或重复必须规避或修正；\n")
	b.WriteString("3. 本章的人物出场、事件推进必须建立在【历史前文】的既有进度之上，不得回退、重置或重新引入已解决的设定。\n\n")
	// 流水线差异化要求
	switch pl {
	case PipelineStrict:
		b.WriteString("【模式要求】严谨模式。请直接输出结构完整的初稿框架（含核心论点/事实/段落排布），逻辑与事实必须准确，由你独立完成主体内容框架。\n")
	case PipelineArt:
		b.WriteString("【模式要求】文艺创作模式。仅需产出极简框架与关键剧情节点，留给创作者充足自由度，不必细化到每段。\n")
	default:
		b.WriteString("【模式要求】标准创作。请输出完整章节大纲、关键剧情节点、角色行为动机。\n")
	}
	b.WriteString("【审稿清单要求】在创作框架末尾必须追加一段「【审稿清单】」：结合本篇具体内容列出 3-6 条审查要点，供校验官逐项核对（如：某角色行为必须符合其人设、某伏笔必须在结尾呼应、节奏不能拖沓、字数必须接近目标等）。要点要具体到本篇剧情，不要泛泛而谈。\n")
	b.WriteString("【输出要求】直接输出创作框架本身，不要任何开场白、解释或多余说明。\n")
	writeWebRef(&b, req)
	b.WriteString("\n请直接输出创作框架：")
	return b.String()
}

// buildWorkerUserPrompt 构造创作者的用户提示词
func buildWorkerUserPrompt(req GenerateRequest, bundle ContextBundle, outline string, segIdx, segTotal int, prevSegContent string) string {
	var b strings.Builder
	if bundle.HasContext() {
		b.WriteString(bundle.AssembledText())
		b.WriteString("\n")
	}
	// 人设/世界观无需重复"铁律"：设定本体已由 AssembledText 注入，Worker system 已要求保持人设设定
	// （2026-08-05 A/B 实测：约束堆叠压制文笔，精简后文笔/情节双达标）
	if strings.TrimSpace(outline) != "" {
		b.WriteString("【大纲提示】上方【创作框架】供剧情参考：按其推进主线，细节可随写作自然发挥，不必机械照搬每个节点。\n\n")
	}
	if req.SelectedText != "" {
		b.WriteString("【编辑器选中文字】\n")
		b.WriteString(req.SelectedText)
		b.WriteString("\n\n")
	}
	if strings.TrimSpace(outline) != "" {
		b.WriteString("【创作框架（来自规划师，剧情参考）】\n")
		b.WriteString(outline)
		b.WriteString("\n\n")
	}
	// 参考素材已由 bundle.AssembledText() 统一注入（含前端 material_text 与素材库融合），
	// 此处不再重复注入，避免同一份素材在 prompt 中出现两次
	if req.NoRewrite {
		b.WriteString("【重要约束：禁止改写已有前文】严格禁止修改、删除、压缩已有历史内容，仅允许在末尾追加续写新内容，保持与前文风格一致。\n\n")
	}
	// 分段撰写指令
	if segTotal > 1 {
		b.WriteString(fmt.Sprintf("【分段撰写】当前撰写 第%d/%d 段，约 %d 字（严禁超过 %d 字，宁短勿长）。", segIdx, segTotal, segmentSize, segmentSize+500))
		b.WriteString("严格依据框架中对应段落的内容撰写，保持与前文衔接、文风统一。仅返回本段正文。\n")
		if prevSegContent != "" {
			b.WriteString("【前段已写内容（请严格衔接到此内容之后继续写，不要重复也不要跳跃）】\n")
			b.WriteString(prevSegContent)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	} else if req.TargetWord > 0 {
		b.WriteString(fmt.Sprintf("【字数要求】约 %d 字，完成后需经审稿校验，请尽量接近目标。\n\n", req.TargetWord))
	}
	b.WriteString("【写作要点】1) 用具体动作、细节、对话展开场景，避免概括式一笔带过；2) 只写当前视角人物所知所见；3) 结尾收一条线，重要情节有铺垫。\n")
	// 叙事框架锚定：优先用自动提炼的"本书叙事特征"（最准），提炼失败时退回通用锚定
	if strings.TrimSpace(bundle.NarrativeHint) != "" {
		b.WriteString(bundle.NarrativeHint)
	} else if strings.TrimSpace(bundle.HistoryContent) != "" || strings.TrimSpace(bundle.MaterialText) != "" {
		b.WriteString("【叙事框架】必须延续上方【历史前文】的叙事结构与口吻：叙述视角与叙述者（如第一人称朋友转述/第三人称）、转述语气（幽默/克制/吐槽）、群像互动方式，一律与历史前文保持一致，不得改变叙事结构。\n")
	}
	b.WriteString("\n")
	b.WriteString("【输出要求】直接输出正文本身：不要任何开场白、标题、思考过程、解释或多余说明，从故事正文第一句开始写。\n")
	writeWebRef(&b, req)
	b.WriteString("请撰写正文：")
	return b.String()
}

// buildFreeUserPrompt 自由写作模式用户提示词（2026-08-05 落地）：
// 正面引导 + 分层拼装（文风样本 few-shot → 出场人物 → 世界观 → 最近前情 → 伏笔 → 素材 → 核心事件 → 字数）。
// 不写任何"禁止"类规则——自由写作要的是毛边感，禁令会把它压成安全范文。
func buildFreeUserPrompt(req GenerateRequest, bundle ContextBundle, styleText, charactersText, recentText, worldText, foreshadowText, materialText string) string {
	var b strings.Builder
	// 1. 写作基调（叙事特征卡动态生成，不写死）+ 正面引导
	if strings.TrimSpace(bundle.NarrativeHint) != "" {
		b.WriteString(bundle.NarrativeHint)
	}
	b.WriteString("自然一点，不用写太工整，可以有废话、有打岔、有说一半的话、有没意义的抬杠，像真人讲故事一样。\n\n")
	// 2. 文风样本（few-shot：自己写的章节，最高权重）
	if strings.TrimSpace(styleText) != "" {
		b.WriteString("【照着这个语气写】（严格模仿这几段的语气、句式、对话感觉，不要模仿内容）\n")
		b.WriteString(styleText)
		b.WriteString("\n\n")
	}
	// 3. 出场人物
	if strings.TrimSpace(charactersText) != "" {
		b.WriteString("【本章出场的人】\n")
		b.WriteString(charactersText)
		b.WriteString("\n\n")
	}
	// 4. 世界观
	if strings.TrimSpace(worldText) != "" {
		b.WriteString("【世界观设定】\n")
		b.WriteString(worldText)
		b.WriteString("\n\n")
	}
	// 5. 最近前情
	if strings.TrimSpace(recentText) != "" {
		b.WriteString("【最近发生的事】\n")
		b.WriteString(recentText)
		b.WriteString("\n\n")
	}
	// 6. 伏笔
	if strings.TrimSpace(foreshadowText) != "" {
		b.WriteString("【未回收伏笔（可自然承接，不必刻意）】\n")
		b.WriteString(foreshadowText)
		b.WriteString("\n\n")
	}
	// 7. 素材库片段
	if strings.TrimSpace(materialText) != "" {
		b.WriteString("【素材参考】\n")
		b.WriteString(materialText)
		b.WriteString("\n\n")
	}
	// 8. 本章核心事件（优先手填大纲，否则需求）
	events := strings.TrimSpace(req.Outline)
	if events == "" {
		events = strings.TrimSpace(req.UserDemand)
	}
	if events != "" {
		b.WriteString("【本章写这几件事】\n")
		b.WriteString(events)
		b.WriteString("\n\n")
	}
	if req.TargetWord > 0 {
		b.WriteString(fmt.Sprintf("【字数】约 %d 字。\n\n", req.TargetWord))
	}
	b.WriteString("开始写正文：")
	return b.String()
}

func buildReviseUserPrompt(req GenerateRequest, bundle ContextBundle, review, currentText string) string {
	var b strings.Builder
	if bundle.HasContext() {
		b.WriteString(bundle.AssembledText())
		b.WriteString("\n")
	}
	b.WriteString("【校验官修改意见】\n")
	b.WriteString(review)
	b.WriteString("\n\n【当前正文】\n")
	b.WriteString(currentText)
	b.WriteString("\n\n")
	if req.NoRewrite {
		b.WriteString("【重要约束：禁止改写已有前文】微调时不得修改已有文字，仅在文末根据修改意见追加补充内容。\n\n")
	}
	// 字数硬约束：修正后全文不得超过目标字数，已超则必须精简（修复“5000字目标生成2万字”的膨胀问题）
	if req.TargetWord > 0 {
		curLen := len([]rune(currentText))
		b.WriteString(fmt.Sprintf("【字数硬约束】当前正文 %d 字，目标 %d 字。修正后的完整正文总字数必须控制在目标字数的 80%%~110%% 之间（即 %d~%d 字）：若当前超出，必须删减冗余段落/描写以达标；若不足，可适度补充。禁止大幅扩写。\n\n", curLen, req.TargetWord, int(float64(req.TargetWord)*0.8), int(float64(req.TargetWord)*1.1)))
	}
	b.WriteString("请根据修改意见对正文进行微调（仅针对问题修改，保留可用内容与核心剧情），输出修改后的完整正文：")
	writeWebRef(&b, req)
	return b.String()
}

func buildVerifierUserPrompt(req GenerateRequest, bundle ContextBundle, content string, pl PipelineName, outline string) string {
	var b strings.Builder
	if bundle.HasContext() {
		b.WriteString(bundle.AssembledText())
		b.WriteString("\n")
	}
	// 人设审查约束：校验官必须对照人物卡核查人设一致性（允许合理成长但需前文铺垫）
	if strings.TrimSpace(bundle.CharacterSetting) != "" {
		b.WriteString("【人设审查】上方【人物卡】是角色设定基准。逐项核查正文中角色是否人设崩坏（外貌/性格/说话方式/关系突变且无前文铺垫）。允许人物随剧情合理成长，但成长必须有铺垫、可识别。发现人设偏差必须列为缺陷。\n\n")
	}
	// 世界观+大纲审查约束
	if strings.TrimSpace(bundle.WorldSetting) != "" {
		b.WriteString("【世界观审查】上方【世界观设定】是世界规则基准。核查正文是否违背世界规则/力量体系/势力设定；世界观演化必须有逻辑铺垫，前后矛盾列为缺陷。\n\n")
	}
	// 文风审查约束：有参考素材/前文时核查文风一致性（防"一章一个味道"；显式区分剧情需要与真实漂移，避免机械判定）
	// 文艺模式跳过：该模式不干涉文学表达与叙事节奏
	if pl != PipelineArt && (strings.TrimSpace(bundle.MaterialText) != "" || strings.TrimSpace(bundle.HistoryContent) != "") {
		b.WriteString("【文风审查】对照上方【参考素材】（若提供）与【历史前文】：核查正文的句式节奏、用词习惯、叙事视角、对话风格是否保持一致。区分「剧情需要的变化」与「真实文风漂移」：因场景/情绪/情节需要的合理变化不算缺陷；与参考素材明显偏离、或与前文出现断层（同一卷内文风突变）才列为缺陷（【次要文字优化建议】级别）。\n\n")
	}
	// 伏笔审查约束：有未回收伏笔提醒时核查推进/回收情况
	if strings.Contains(bundle.HistoryContent, "未回收伏笔提醒") {
		b.WriteString("【伏笔审查】上方【历史前文】含【未回收伏笔提醒】：核查本段是否推进或回收了其中至少一个伏笔。若本段情节明显涉及相关伏笔却完全无视，列为缺陷。\n\n")
	}
	if strings.TrimSpace(outline) != "" {
		b.WriteString("【大纲审查】上方【创作框架】是大纲基准。核查正文是否偏离大纲骨架（结局方向/关键节点）；合理细化不算偏离，但砍掉关键节点或改结局须标为缺陷。\n\n")
	}
	if strings.TrimSpace(outline) != "" {
		b.WriteString("【创作框架与审稿清单（来自规划师，请逐项核对）】\n")
		b.WriteString(truncateOutline(outline, 4000))
		b.WriteString("\n\n")
	}
	b.WriteString("【用户原始需求】\n")
	b.WriteString(req.UserDemand)
	b.WriteString("\n\n【待校验正文】\n")
	b.WriteString(content)
	b.WriteString("\n\n")
	// 字数校验项：校验官必须检查字数是否达标
	if req.TargetWord > 0 {
		b.WriteString(fmt.Sprintf("【字数校验】目标 %d 字，请统计待校验正文的实际字数。若超出目标 20%% 以上或不足目标 70%% 以上，必须标注为缺陷项。\n", req.TargetWord))
	}
	switch pl {
	case PipelineStrict:
		b.WriteString("【审查标准】严谨模式，高标准核查逻辑漏洞、事实错误、框架偏移。\n")
	case PipelineArt:
		b.WriteString("【审查标准】文艺模式，宽松审查，仅拦截重大人设崩坏、致命剧情矛盾，不干涉文学表达与叙事节奏。\n")
	default:
		b.WriteString("【审查标准】标准审查，逐项核查角色一致性、世界观冲突、剧情逻辑、文字质量、需求匹配。\n")
	}
	b.WriteString("\n【硬性输出格式】必须逐条输出，禁止省略：\n")
	writeWebRef(&b, req)
	b.WriteString("1. 先输出【清单核对】：若提供了审稿清单，对每一条单独输出一行「条目N：已执行/未执行 —— 证据（引用正文原文）」；无清单则按审查标准逐项输出结论；\n")
	b.WriteString("2. 再输出【缺陷清单】：有缺陷逐条列出（问题位置 + 类型 + 精准修改建议）；全部通过才输出【校验通过】；\n")
	b.WriteString("3. 禁止只输出【校验通过】四个字而不给出核对过程；\n")
	b.WriteString("4. 最后必须单独一行输出【评分】N/100（0-100 整数），85 分及以上才可通过。\n")
	b.WriteString("\n请输出校验结果：")
	return b.String()
}

// buildHelperUserPrompt 构造轻助手的用户提示词（输入强制限制 500 字）
func buildHelperUserPrompt(req GenerateRequest, bundle ContextBundle, lightLimit int) string {
	var b strings.Builder
	b.WriteString("【任务】\n")
	b.WriteString(req.UserDemand)
	b.WriteString("\n\n")
	// 轻量化模式输入上限强制限制
	selected := req.SelectedText
	if r := []rune(selected); len(r) > lightLimit {
		selected = string(r[:lightLimit])
	}
	if selected != "" {
		b.WriteString("【待处理文本】\n")
		b.WriteString(selected)
		b.WriteString("\n\n")
	}
	if bundle.HasContext() {
		// 轻量任务仅注入精简上下文
		b.WriteString("【相关设定】\n")
		b.WriteString(bundle.AssembledText())
		b.WriteString("\n")
	}
	writeWebRef(&b, req)
	b.WriteString("请直接返回处理结果：")
	return b.String()
}

// buildManualUserPrompt 构造手动模式的用户提示词
func buildManualUserPrompt(req GenerateRequest, bundle ContextBundle) string {
	var b strings.Builder
	if bundle.HasContext() {
		b.WriteString(bundle.AssembledText())
		b.WriteString("\n")
	}
	if req.SelectedText != "" {
		b.WriteString("【编辑器选中文字】\n")
		b.WriteString(req.SelectedText)
		b.WriteString("\n\n")
	}
	if req.TargetWord > 0 {
		b.WriteString(fmt.Sprintf("【字数要求】约 %d 字，请尽量接近。\n\n", req.TargetWord))
	}
	b.WriteString("【需求】\n")
	b.WriteString(req.UserDemand)
	b.WriteString("\n\n")
	writeWebRef(&b, req)
	b.WriteString("请生成内容：")
	return b.String()
}

// truncateOutline 截断创作框架（含审稿清单）到指定字数，防止校验官输入过长
func truncateOutline(s string, maxRunes int) string {
	r := []rune(s)
	if len(r) > maxRunes {
		return string(r[:maxRunes]) + "\n……（后文略）"
	}
	return s
}
