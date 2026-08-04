package api

import (
	"strings"
	"testing"
)

// TestSummarizePromptTimeline 契约回归：summarize prompt 必须包含时间线区分规则，
// 防止后续改 prompt 时丢掉「人物身份/关系分阶段标注」的能力（用户反馈：以前/现在身份混为一谈）
func TestSummarizePromptTimeline(t *testing.T) {
	p := buildToolPrompt("summarize", "正文占位", "", "", "")
	required := []string{
		"时间线区分",       // 规则 6
		"现为",           // 规则 6 示例
		"曾为",           // 规则 6 示例
		"不要写",          // 规则 7（正文尚未发生的关系不要写）
		"人际关系",        // 字段模板
		"【人物卡列表】",     // 输出格式
	}
	for _, s := range required {
		if !strings.Contains(p, s) {
			t.Errorf("summarize prompt 缺少关键内容 %q", s)
		}
	}
	// 背景行模板应提示时间线
	if !strings.Contains(p, "阶段变化时务必标注时间线") {
		t.Error("背景字段模板应提示时间线标注")
	}
	// 禁止性条款与关系行提示也应保留
	if !strings.Contains(p, "绝对禁止把不同阶段的身份/关系并列") {
		t.Error("规则 6 的禁止并列条款缺失")
	}
	if !strings.Contains(p, "人际关系：<描述，同样注意时间线") {
		t.Error("人际关系字段模板应提示时间线")
	}
}
