package pipeline

import "testing"

// 解释性旁白：第三人称解说命中
func TestDetectSentenceIssues_ThirdPerson(t *testing.T) {
	text := "他推开门走进教室。\n他知道这意味着什么，但什么都没说。\n她坐下来，翻开练习册。"
	issues := DetectSentenceIssues(text)
	if len(issues) < 1 {
		t.Fatalf("应检测到'他知道这意味着什么'，got %d 个问题", len(issues))
	}
	if issues[0].ParaIndex != 2 {
		t.Fatalf("段落定位错误：got %d want 2", issues[0].ParaIndex)
	}
}

// 白名单：引号内对话不检测
func TestDetectSentenceIssues_QuoteWhitelist(t *testing.T) {
	text := "林云说：\"你知道这意味着什么吗？\"\n他摇头说：\"我不知道。\""
	issues := DetectSentenceIssues(text)
	if len(issues) != 0 {
		t.Fatalf("对话内容不应检测为解释性旁白，got %d", len(issues))
	}
}

// 白名单：第一人称转述体叙述不误杀（"我知道/我后来才明白"是正常写法）
func TestDetectSentenceIssues_FirstPerson(t *testing.T) {
	text := "我后来才知道，那天她其实来了。\n我知道他心里在想什么，但我没点破。"
	issues := DetectSentenceIssues(text)
	if len(issues) != 0 {
		t.Fatalf("第一人称转述体不应误杀，got %d: %+v", len(issues), issues)
	}
}

// 无主语总结词：第一人称段也检测（"这意味着"是 AI 腔）
func TestDetectSentenceIssues_SummaryWord(t *testing.T) {
	text := "我看着他，这意味着我们的关系已经结束了。"
	issues := DetectSentenceIssues(text)
	if len(issues) < 1 {
		t.Fatalf("第一人称段也应检测'这意味着'，got 0")
	}
}

// 干净文本不误报
func TestDetectSentenceIssues_Clean(t *testing.T) {
	text := "他放下书包，拉开椅子坐下。\n她端着一杯水走进来，杯底碰了一下桌面。\n早自习铃响了。"
	issues := DetectSentenceIssues(text)
	if len(issues) != 0 {
		t.Fatalf("干净文本不应误报，got %d: %+v", len(issues), issues)
	}
}

// 问题分类：文字类 vs 剧情类
func TestClassifyReviewIssues(t *testing.T) {
	issues := []string{
		"对话像背书，不像真人说话",
		"存在解释性旁白'他知道这意味着什么'",
		"人设崩塌：惊鸿突然性情大变",
		"剧情矛盾：前文两人已认识",
		"段落节奏太平",
	}
	text, plot := classifyReviewIssues(issues)
	if len(text) != 3 {
		t.Fatalf("文字类应为3个，got %d: %v", len(text), text)
	}
	if len(plot) != 2 {
		t.Fatalf("剧情类应为2个，got %d: %v", len(plot), plot)
	}
	// 无法归类 → 默认剧情类
	t2, p2 := classifyReviewIssues([]string{"某处衔接生硬"})
	if len(t2) != 0 || len(p2) != 1 {
		t.Fatalf("未归类问题应默认走剧情类，got text=%v plot=%v", t2, p2)
	}
}
