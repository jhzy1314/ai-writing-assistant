package database

import (
	"context"
	"strings"
	"testing"
)

// TestSplitByTitlesNoPanic 回归测试：旧实现在第一个标题匹配时执行
// `segments[:len(segments)-1]`（len==0）会 panic —— slice bounds out of range [:-1]，
// 导致 AI 分章（POST /api/chapters/split auto 模式）100% 失败。
// 现删除死代码后应正常分割且不 panic。
func TestSplitByTitlesNoPanic(t *testing.T) {
	db, err := Open(context.Background(), ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := NewStore(db)
	ctx := context.Background()

	newProject := func(name string) string {
		p, err := store.CreateProject(ctx, name, "novel")
		if err != nil {
			t.Fatalf("创建项目失败: %v", err)
		}
		return p.ID
	}

	// 场景1：第一个标题位于文本开头（旧实现必 panic 的路径）
	req := &SplitChaptersRequest{
		ProjectID: newProject("分章测试1"),
		Content:   "第一章 风起\n正文内容……\n第二章 云涌\n更多正文",
	}
	chs, err := store.SplitByTitles(ctx, req, []string{"第一章 风起", "第二章 云涌"})
	if err != nil {
		t.Fatalf("场景1不应报错: %v", err)
	}
	if len(chs) != 2 {
		t.Fatalf("场景1期望 2 章，得到 %d", len(chs))
	}
	if chs[0].Title != "第一章 风起" || chs[1].Title != "第二章 云涌" {
		t.Fatalf("章节标题错误: %q, %q", chs[0].Title, chs[1].Title)
	}
	if !strings.Contains(chs[0].Content, "正文内容") || !strings.Contains(chs[1].Content, "更多正文") {
		t.Fatalf("章节内容切分错误: %q | %q", chs[0].Content, chs[1].Content)
	}

	// 场景2：标题前有序言（第一个标题不在开头）
	req2 := &SplitChaptersRequest{
		ProjectID: newProject("分章测试2"),
		Content:   "序言\n第一章 风起\n甲\n第二章 云涌\n乙",
	}
	chs2, err := store.SplitByTitles(ctx, req2, []string{"第一章 风起", "第二章 云涌"})
	if err != nil {
		t.Fatalf("场景2不应报错: %v", err)
	}
	if len(chs2) != 2 {
		t.Fatalf("场景2期望 2 章，得到 %d", len(chs2))
	}
	if !strings.Contains(chs2[0].Content, "甲") || !strings.Contains(chs2[1].Content, "乙") {
		t.Fatalf("场景2内容切分错误: %q | %q", chs2[0].Content, chs2[1].Content)
	}

	// 场景3：标题全部匹配不到 → 返回错误而非 panic
	req3 := &SplitChaptersRequest{ProjectID: newProject("分章测试3"), Content: "没有任何标题的纯文本"}
	if _, err := store.SplitByTitles(ctx, req3, []string{"第一章", "第二章"}); err == nil {
		t.Fatal("场景3应返回错误")
	}
}
