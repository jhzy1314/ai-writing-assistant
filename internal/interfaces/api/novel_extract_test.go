package api

import (
	"errors"
	"strings"
	"testing"
)

// TestExtractJSON 验证 AI 输出的鲁棒 JSON 提取（数组/对象、围栏、前后说明）
func TestExtractJSON(t *testing.T) {
	cases := []struct {
		name   string
		text   string
		isObj  bool
		expect int // 提取到的数组长度或对象字段数
	}{
		{"纯数组", `[{"a":1},{"a":2}]`, false, 2},
		{"markdown围栏", "```json\n[{\"a\":1}]\n```", false, 1},
		{"大写围栏", "```JSON\n[{\"a\":1}]\n```", false, 1},
		{"前后说明", "好的，识别结果如下：[{\"a\":1}] 以上就是全部", false, 1},
		{"对象", "根据分析：{\"characters\":[{\"name\":\"林风\"}],\"relations\":[]}", true, 2},
		{"对象围栏", "```json\n{\"characters\":[]}\n```", true, 1},
		{"非法JSON", "这不是JSON", false, -1},
		{"括号不配对", "[{\"a\":1}", false, -1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.isObj {
				var out map[string]interface{}
				ok := extractJSONObject(c.text, &out)
				if c.expect < 0 {
					if ok {
						t.Fatal("期望解析失败，实际成功")
					}
					return
				}
				if !ok {
					t.Fatal("解析失败")
				}
				if len(out) != c.expect {
					t.Fatalf("字段数=%d，期望 %d", len(out), c.expect)
				}
				return
			}
			var out []map[string]interface{}
			ok := extractJSONArray(c.text, &out)
			if c.expect < 0 {
				if ok {
					t.Fatal("期望解析失败，实际成功")
				}
				return
			}
			if !ok {
				t.Fatal("解析失败")
			}
			if len(out) != c.expect {
				t.Fatalf("数组长度=%d，期望 %d", len(out), c.expect)
			}
		})
	}
}

// TestJSONParseError 验证 jsonParseError 可用 errors.As 区分（handler 据此返回 422 而非 503）
func TestJSONParseError(t *testing.T) {
	err := &jsonParseError{msg: "AI 输出不是合法 JSON: xxx"}
	var pe *jsonParseError
	if !errors.As(err, &pe) {
		t.Fatal("errors.As 应命中 jsonParseError")
	}
	var other error = errors.New("模型不可用")
	if errors.As(other, &pe) {
		t.Fatal("普通错误不应命中 jsonParseError")
	}
}

// TestTruncate 验证截断行为
func TestTruncate(t *testing.T) {
	s := "一二三四五六七八九十"
	if got := truncate(s, 5); got != "一二三四五…" {
		t.Fatalf("截断结果异常: %q", got)
	}
	if got := truncate(s, 100); got != s {
		t.Fatalf("不超限不应截断: %q", got)
	}
}

// TestSplitCandidates 验证【候选N】标记切分
func TestSplitCandidates(t *testing.T) {
	text := "【候选1】\n第一段正文内容\n【候选2】第二段内容\n【候选3】\n第三段内容"
	cands := splitCandidates(text, 3)
	if len(cands) != 3 {
		t.Fatalf("期望 3 个候选，得到 %d: %v", len(cands), cands)
	}
	if !strings.Contains(cands[0], "第一段正文") || !strings.Contains(cands[1], "第二段") || !strings.Contains(cands[2], "第三段") {
		t.Fatalf("切分内容错误: %v", cands)
	}
	// 候选数超过 count 时截断
	more := "【候选1】a\n【候选2】b\n【候选3】c\n【候选4】d\n【候选5】e"
	if got := splitCandidates(more, 3); len(got) != 3 {
		t.Fatalf("期望截断到 3 个，得到 %d", len(got))
	}
	// 无标记时返回 nil
	if got := splitCandidates("没有标记的文本", 3); got != nil {
		t.Fatalf("无标记应返回 nil: %v", got)
	}
}
