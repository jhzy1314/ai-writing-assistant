package pipeline

import (
	"fmt"
	"strings"
)

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
	b.WriteString("\n请直接输出创作框架：")
	return b.String()
}

// buildWorkerUserPrompt 构造创作者的用户提示词
func buildWorkerUserPrompt(req GenerateRequest, bundle ContextBundle, outline string, segIdx, segTotal int) string {
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
		b.WriteString(fmt.Sprintf("【分段撰写】当前撰写 第%d/%d 段，约 %d 字。", segIdx, segTotal, segmentSize))
		b.WriteString("严格依据框架中对应段落的内容撰写，保持与前文衔接、文风统一。仅返回本段正文。\n\n")
	} else if req.TargetWord > 0 {
		b.WriteString(fmt.Sprintf("【字数要求】约 %d 字。这是硬性要求，输出正文必须接近此字数，不可明显偏少。\n\n", req.TargetWord))
	}
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
	b.WriteString("请根据修改意见对正文进行微调（仅针对问题修改，保留可用内容与核心剧情），输出修改后的完整正文：")
	return b.String()
}

// buildVerifierUserPrompt 构造校验官的用户提示词
func buildVerifierUserPrompt(req GenerateRequest, bundle ContextBundle, content string, pl PipelineName) string {
	var b strings.Builder
	if bundle.HasContext() {
		b.WriteString(bundle.AssembledText())
		b.WriteString("\n")
	}
	b.WriteString("【用户原始需求】\n")
	b.WriteString(req.UserDemand)
	b.WriteString("\n\n【待校验正文】\n")
	b.WriteString(content)
	b.WriteString("\n\n")
	switch pl {
	case PipelineStrict:
		b.WriteString("【审查标准】严谨模式，高标准核查逻辑漏洞、事实错误、框架偏移。\n")
	case PipelineArt:
		b.WriteString("【审查标准】文艺模式，宽松审查，仅拦截重大人设崩坏、致命剧情矛盾，不干涉文学表达与叙事节奏。\n")
	default:
		b.WriteString("【审查标准】标准审查，逐项核查角色一致性、世界观冲突、剧情逻辑、文字质量、需求匹配。\n")
	}
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
		b.WriteString(fmt.Sprintf("【字数要求】约 %d 字。\n\n", req.TargetWord))
	}
	b.WriteString("【需求】\n")
	b.WriteString(req.UserDemand)
	b.WriteString("\n\n请生成内容：")
	return b.String()
}
