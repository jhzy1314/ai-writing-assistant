package pipeline

import (
	"strings"
	"testing"
)

// Scrub：删除章节元信息行，保留正文
func TestScrubChapterMeta(t *testing.T) {
	in := `（第3章）

他推开门，风灌进来。

（第3章正文）
本章完
待续

【以下是修订后的第3章完整正文】

她没回头。
——完——`
	out := scrubChapterMeta(in)
	for _, bad := range []string{"（第3章）", "本章完", "待续", "以下是修订后的第3章完整正文"} {
		if strings.Contains(out, bad) {
			t.Fatalf("元信息 %q 未被删除:\n%s", bad, out)
		}
	}
	if !strings.Contains(out, "他推开门") || !strings.Contains(out, "她没回头") {
		t.Fatalf("正文被误删:\n%s", out)
	}
}

// Scrub：正文中的"第X章"对话/叙述不能被误删
func TestScrubChapterMeta_KeepsProse(t *testing.T) {
	in := `"这是第3章的内容吗？"她问。
他翻开书，找到"第3章"的标题。
林云说道："本章完再说。"`
	out := scrubChapterMeta(in)
	if !strings.Contains(out, "这是第3章的内容吗") || !strings.Contains(out, "本章完再说") {
		t.Fatalf("正文中的元信息字样被误删:\n%s", out)
	}
}

// Scrub：连续空行压缩
func TestScrubChapterMeta_CollapseBlank(t *testing.T) {
	in := "第一段\n\n\n\n\n第二段"
	out := scrubChapterMeta(in)
	if strings.Contains(out, "\n\n\n\n") {
		t.Fatalf("空行未压缩:\n%q", out)
	}
	if !strings.Contains(out, "第一段") || !strings.Contains(out, "第二段") {
		t.Fatalf("正文丢失:\n%s", out)
	}
}

// 一致性检查解析：合法 JSON
func TestParseOutlineConsistencyResult_Valid(t *testing.T) {
	raw := `好的，检查完毕：{"conflict": true, "issues": ["大纲安排初次见面但前文已认识"], "revised_outline": "修订后大纲"}`
	conflict, revised := parseOutlineConsistencyResult(raw)
	if !conflict || !strings.Contains(revised, "修订后大纲") {
		t.Fatalf("解析失败 conflict=%v revised=%q", conflict, revised)
	}
}

// 一致性检查解析：无冲突
func TestParseOutlineConsistencyResult_NoConflict(t *testing.T) {
	raw := `{"conflict": false, "issues": [], "revised_outline": null}`
	conflict, revised := parseOutlineConsistencyResult(raw)
	if conflict || revised != "" {
		t.Fatalf("解析失败 conflict=%v revised=%q", conflict, revised)
	}
}

// 一致性检查解析：坏输入降级为无冲突
func TestParseOutlineConsistencyResult_Bad(t *testing.T) {
	for _, raw := range []string{"", "模型没按格式输出", "{broken json", "{{}}"} {
		conflict, revised := parseOutlineConsistencyResult(raw)
		if conflict || revised != "" {
			t.Fatalf("坏输入 %q 应降级为无冲突, got %v %q", raw, conflict, revised)
		}
	}
}
