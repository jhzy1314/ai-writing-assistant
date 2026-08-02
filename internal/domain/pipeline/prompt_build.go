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
func buildThinkerUserPrompt(req GenerateRequest, bundle ContextBundle, pl PipelineName) string {
	var b strings.Builder
	b.WriteString("【用户创作需求】\n")
	b.WriteString(req.UserDemand)
	b.WriteString("\n\n")
	if bundle.HasContext() {
		b.WriteString(bundle.AssembledText())
		b.WriteString("\n")
	}
	// 人设保持约束：规划剧情时人物行为必须基于人物卡设定
	if strings.TrimSpace(bundle.CharacterSetting) != "" {
		b.WriteString("【人设铁律】上方【人物卡】是角色设定基准。规划剧情/动机/对话时，所有角色的行为必须符合其人物卡设定（性格、背景、关系、说话方式）。人物可随剧情合理成长，但成长线要有铺垫。\n\n")
	}
	// 世界观+大纲约束：规划剧情时世界规则与既有大纲是基准，演化需自洽
	if strings.TrimSpace(bundle.WorldSetting) != "" {
		b.WriteString("【世界观铁律】上方【世界观设定】是世界规则基准。规划剧情时不得违背世界规则/力量体系/势力格局；如需演化（新势力、规则变化），要在剧情中给出合理契机与铺垫。\n\n")
	}
	if strings.TrimSpace(req.UserDemand) != "" {
		b.WriteString("【大纲约束】用户的需求/大纲见上方【用户创作需求】。规划创作框架时应遵循其主线方向，可细化补充但不得推翻骨架与结局方向。\n\n")
	}
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
	// 人设+世界观+大纲保持约束：人物卡已注入时，强制遵守人设并允许合理成长；世界观/大纲同样约束
	if strings.TrimSpace(bundle.CharacterSetting) != "" {
		b.WriteString("【人设铁律】必须严格遵守上方【人物卡】中的人物设定：外貌、性格、说话方式、背景、人际关系等核心特质不得偏离。人物可以随剧情事件合理成长变化（如性格渐变、关系发展），但必须有前文铺垫，禁止突然的性情大变或行为崩坏。若剧情需要人设发展，要在行文中自然过渡并保持人物可识别性。\n\n")
	}
	if strings.TrimSpace(bundle.WorldSetting) != "" {
		b.WriteString("【世界观铁律】必须遵守上方【世界观设定】中的世界规则、力量体系、势力格局、地理设定。世界观可随故事发展合理演化（如新势力崛起、规则被打破），但必须逻辑自洽、有前文铺垫，禁止前后矛盾或随意推翻既有设定。\n\n")
	}
	if strings.TrimSpace(outline) != "" {
		b.WriteString("【大纲约束】上方【创作框架】是规划师定的大纲。应按大纲推进剧情，但可随写作自然合理调整细节（如对话走向、过渡方式）；重大偏离（改变结局、砍掉关键节点）需在行文中自然承接，不得生硬跳跃。\n\n")
	}
	if req.SelectedText != "" {
		b.WriteString("【编辑器选中文字】\n")
		b.WriteString(req.SelectedText)
		b.WriteString("\n\n")
	}
	b.WriteString("【创作框架（来自规划师）】\n")
	b.WriteString(outline)
	b.WriteString("\n\n")
	if req.MaterialText != "" {
		b.WriteString("【参考文风素材-严格学习以下文本的叙事视角、句式、对话节奏、文笔风格，在续写中保持一致】\n")
		b.WriteString(req.MaterialText)
		b.WriteString("\n\n")
	}
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
	b.WriteString("【输出要求】直接输出正文本身：不要任何开场白、标题、思考过程、解释或多余说明，从故事正文第一句开始写。\n")
	writeWebRef(&b, req)
	b.WriteString("请撰写正文：")
	return b.String()
}

// buildReviseUserPrompt 构造微调（校验后回传）的用户提示词
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

// buildVerifierUserPrompt 构造校验官的用户提示词
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
