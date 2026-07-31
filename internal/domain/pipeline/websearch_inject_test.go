package pipeline

import (
	"strings"
	"testing"
)

func TestWebRefInjection(t *testing.T) {
	req := GenerateRequest{
		UserDemand: "写一个发生在巴黎的悬疑故事",
		WebSearch:  true,
		WebInfo:    "检索词：巴黎悬疑小说\n1. 标题\nhttps://example.com\n摘要内容",
	}
	// thinker
	p := buildThinkerUserPrompt(req, ContextBundle{}, PipelineStandard)
	if !strings.Contains(p, "【联网参考信息】") || !strings.Contains(p, "严禁大段复制") {
		t.Fatalf("thinker prompt missing web ref block")
	}
	// worker
	p2 := buildWorkerUserPrompt(req, ContextBundle{}, "outline", 0, 1, "")
	if !strings.Contains(p2, "【联网参考信息】") {
		t.Fatalf("worker prompt missing web ref block")
	}
	// verifier
	p3 := buildVerifierUserPrompt(req, ContextBundle{}, "content", PipelineStandard, "")
	if !strings.Contains(p3, "【联网参考信息】") {
		t.Fatalf("verifier prompt missing web ref block")
	}
	// helper
	p4 := buildHelperUserPrompt(req, ContextBundle{}, 500)
	if !strings.Contains(p4, "【联网参考信息】") {
		t.Fatalf("helper prompt missing web ref block")
	}
	// 关闭时不注入
	req.WebInfo = ""
	p5 := buildThinkerUserPrompt(req, ContextBundle{}, PipelineStandard)
	if strings.Contains(p5, "【联网参考信息】") {
		t.Fatalf("web ref should not inject when WebInfo empty")
	}
	t.Log("all injection checks passed")
}
