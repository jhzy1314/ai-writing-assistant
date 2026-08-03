package pipeline

import (
	"strings"
	"testing"
)

// TestVerifierStyleCheckPrompt 有参考素材/前文时，Verifier 提示词应包含【文风审查】约束
func TestVerifierStyleCheckPrompt(t *testing.T) {
	// 场景1：有素材（文风样本）+ 前文 → 应包含文风审查
	bundleWith := ContextBundle{
		CharacterSetting: "人物卡内容",
		MaterialText:     "【参考素材】龙族风格样本片段……",
		HistoryContent:   "前文内容……",
	}
	p1 := buildVerifierUserPrompt(GenerateRequest{}, bundleWith, "正文内容", PipelineStandard, "大纲")
	if !strings.Contains(p1, "【文风审查】") {
		t.Fatalf("有素材/前文时应包含【文风审查】，实际: %.120s", p1)
	}
	if !strings.Contains(p1, "真实文风漂移") {
		t.Fatalf("应包含「剧情需要的变化」与「真实文风漂移」的区分说明，避免机械判定")
	}

	// 场景2：无素材无前文 → 不应包含文风审查（避免空约束）
	bundleEmpty := ContextBundle{CharacterSetting: "人物卡内容"}
	p2 := buildVerifierUserPrompt(GenerateRequest{}, bundleEmpty, "正文内容", PipelineStandard, "大纲")
	if strings.Contains(p2, "【文风审查】") {
		t.Fatalf("无素材/前文时不应包含【文风审查】")
	}

	// 场景3：文艺模式 → 即使有素材/前文也应跳过（不干涉文学表达）
	p3 := buildVerifierUserPrompt(GenerateRequest{}, bundleWith, "正文内容", PipelineArt, "大纲")
	if strings.Contains(p3, "【文风审查】") {
		t.Fatalf("文艺模式不应包含【文风审查】")
	}
}
