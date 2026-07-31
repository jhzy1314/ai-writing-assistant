package pipeline

import (
	"strings"
	"testing"
)

func TestReviewPassed(t *testing.T) {
	tests := []struct {
		name   string
		review string
		want   bool
	}{
		{"empty string", "", true},
		{"校验通过 header", "【校验通过】", true},
		{"contains 校验通过", "正文……校验通过……结尾", true},
		{"not passed", "发现3处问题：1. 角色OOC 2. 世界观冲突 3. 逻辑矛盾", false},
		{"english pass", "Review passed, no issues found", false},
		{"校验通过 in middle", "发现问题：无\n校验通过\n建议改进：无", true},
		{"trimmed pass", "  校验通过  ", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := reviewPassed(tt.review)
			if got != tt.want {
				t.Errorf("reviewPassed(%q) = %v, want %v", tt.review, got, tt.want)
			}
		})
	}
}

func TestExtractIssues(t *testing.T) {
	t.Run("extract from review text", func(t *testing.T) {
		review := "1. 角色A在第三章表现出OOC，与其设定矛盾\n2. 世界观中火系魔法不应在雨天使用\n3. 第5段存在重复描写"
		issues := extractIssues(review)
		if len(issues) != 3 {
			t.Fatalf("expected 3 issues, got %d: %v", len(issues), issues)
		}
	})
	t.Run("skip pass line", func(t *testing.T) {
		review := "校验通过\n整体质量良好"
		issues := extractIssues(review)
		for _, iss := range issues {
			if strings.Contains(iss, "校验通过") {
				t.Errorf("should not contain pass line: %q", iss)
			}
		}
	})
	t.Run("max 12 issues", func(t *testing.T) {
		var lines []string
		for i := 0; i < 20; i++ {
			lines = append(lines, "问题"+string(rune('A'+i%26))+"：测试")
		}
		review := strings.Join(lines, "\n")
		issues := extractIssues(review)
		if len(issues) > 12 {
			t.Errorf("max issues exceeded: got %d, want <=12", len(issues))
		}
	})
	t.Run("empty review returns self", func(t *testing.T) {
		review := "some issue without newlines"
		issues := extractIssues(review)
		if len(issues) == 0 {
			t.Error("should return at least the review itself")
		}
	})
}

func TestTruncateForReview(t *testing.T) {
	t.Run("short text unchanged", func(t *testing.T) {
		s := "只有一百字的内容"
		got := truncateForReview(s)
		if got != s {
			t.Errorf("short text should not change: got %d chars", len([]rune(got)))
		}
	})
	t.Run("long text truncated", func(t *testing.T) {
		s := strings.Repeat("文字内容测试", 2000) // 12000 runes
		got := truncateForReview(s)
		r := []rune(got)
		if len(r) > 8100 {
			t.Errorf("too long: %d runes", len(r))
		}
		if !strings.Contains(got, "后文略") {
			t.Error("should contain truncation suffix")
		}
	})
	t.Run("exact boundary", func(t *testing.T) {
		s := strings.Repeat("文", 8000) + "extra"
		got := truncateForReview(s)
		r := []rune(got)
		if len(r) > 8100 {
			t.Errorf("boundary failed: %d runes", len(r))
		}
	})
}

func TestExtractIssuesWithSkipLogic(t *testing.T) {
	review := `【校验通过】没有问题
整体质量良好`
	issues := extractIssues(review)
	if len(issues) != 1 || issues[0] != "整体质量良好" {
		t.Errorf("unexpected issues: %v", issues)
	}
}

func TestNumSegments(t *testing.T) {
	tests := []struct {
		target int
		want   int
	}{
		{0, 1},
		{100, 1},
		{3500, 1},
		{4000, 2},
		{4001, 2},
		{7000, 2},
		{7001, 3},
		{10500, 3},
	}
	for _, tt := range tests {
		got := numSegments(GenerateRequest{TargetWord: tt.target})
		if got != tt.want {
			t.Errorf("numSegments(%d) = %d, want %d", tt.target, got, tt.want)
		}
	}
}

func TestNeedsSegmentation(t *testing.T) {
	if needsSegmentation(GenerateRequest{TargetWord: 3000}) {
		t.Error("3000 words should not need segmentation")
	}
	if !needsSegmentation(GenerateRequest{TargetWord: 5000}) {
		t.Error("5000 words should need segmentation")
	}
}

func TestTruncateForReviewEarlyChapters(t *testing.T) {
	s := "短"
	got := truncateForReview(s)
	if got != s {
		t.Errorf("unexpected truncation: %q", got)
	}
}

func TestNumSegmentsEdge(t *testing.T) {
	if got := numSegments(GenerateRequest{TargetWord: 3500}); got != 1 {
		t.Errorf("3500 words: got %d, want 1", got)
	}
	if got := numSegments(GenerateRequest{TargetWord: 3501}); got != 2 {
		t.Errorf("3501 words: got %d, want 2", got)
	}
}

func TestSegmentSizeConstant(t *testing.T) {
	if segmentSize != 3500 {
		t.Errorf("segmentSize = %d, want 3500", segmentSize)
	}
}
