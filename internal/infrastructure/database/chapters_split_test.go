package database

import (
	"context"
	"strings"
	"testing"
)

// TestFindTitleIndex 回归测试：AI 分章定位标题时，
// 正文中「详见第十章」式引用不应被误判为章节边界。
func TestFindTitleIndex(t *testing.T) {
	content := "暗恋这件难过的小事\n\n第一卷\n\n**第九章**\n\n正文段落一。\n" +
		"他翻到第十章看了看内容。\n" + // 正文引用，不应命中
		"这是第十章所在段的结尾文字。第十章\n" + // 段尾标题
		"他在黑暗里笑了一下。　　**第十一章**\n" +
		"结尾。　　第十二章"
	// 第九章：独立行（带 **，返回位置应指向「第九章」本身，不含 **）
	start9, end9 := findTitleIndex(content, "第九章")
	if start9 < 0 {
		t.Fatal("独立行标题应命中")
	}
	if content[start9:end9] != "第九章" {
		t.Fatalf("** 独立行定位错误：应指向「第九章」，得到 %q", content[start9:end9])
	}
	// 第十章：应命中段尾标题（结尾文字。第十章）而非更靠前的正文引用
	start10, end10 := findTitleIndex(content, "第十章")
	if start10 < 0 {
		t.Fatal("段尾标题应命中")
	}
	tailPos := strings.Index(content, "结尾文字。第十章") + len("结尾文字。")
	if start10 != tailPos {
		t.Fatalf("定位错误：应命中段尾标题（位置 %d），实际 %d（正文引用在 %d）", tailPos, start10, strings.Index(content, "第十章"))
	}
	if content[start10:end10] != "第十章" {
		t.Fatalf("段尾标题定位错误：得到 %q", content[start10:end10])
	}
	// 第十一章：带 ** 的段尾标题
	start11, end11 := findTitleIndex(content, "第十一章")
	if start11 < 0 {
		t.Fatal("带 ** 段尾标题应命中")
	}
	if content[start11:end11] != "第十一章" {
		t.Fatalf("带 ** 段尾标题定位错误：得到 %q", content[start11:end11])
	}
	// 第十二章：全角空格分隔
	if start12, _ := findTitleIndex(content, "第十二章"); start12 < 0 {
		t.Fatal("全角空格段尾标题应命中")
	}
	// 不存在的标题
	if start, _ := findTitleIndex(content, "第二十章"); start != -1 {
		t.Fatalf("不存在的标题应返回 -1，得到 %d", start)
	}
}

// TestSplitTrailingChapterTitle 回归测试：docx 导出的章节标题常与正文同段
// （如「……那一页。第十章」「……笑了一下。　　**第十一章**」），
// 必须识别为章节边界而不是正文。
func TestSplitTrailingChapterTitle(t *testing.T) {
	content := "**第九章**\n" +
		"正文第一段内容，描写场景。\n" +
		"这是第十章所在段的结尾文字。第十章\n" +
		"接下来的剧情继续展开。\n" +
		"他在黑暗里笑了一下。　　**第十一章**\n" +
		"新的剧情片段开始了。\n" +
		"今天发生了两件事，应该记下来。　　第十二章\n" +
		"结尾正文。"
	segs := splitContent(content, "auto")
	if len(segs) != 4 {
		t.Fatalf("期望识别 4 章（九/十/十一/十二），得到 %d 章: %+v", len(segs), segs)
	}
	wantTitles := []string{"第九章", "第十章", "第十一章", "第十二章"}
	for i, want := range wantTitles {
		if segs[i].title != want {
			t.Fatalf("第 %d 章标题期望 %q，得到 %q", i+1, want, segs[i].title)
		}
	}
	// 正文归属正确：第九章正文以段尾标题前的文字收尾；第十章正文含其后的段落
	if !strings.Contains(segs[0].content, "结尾文字。") {
		t.Fatalf("第九章正文应含段尾标题前的文字: %q", segs[0].content)
	}
	if !strings.Contains(segs[1].content, "接下来的剧情继续展开。") || !strings.Contains(segs[1].content, "他在黑暗里笑了一下。") {
		t.Fatalf("第十章正文丢失: %q", segs[1].content)
	}
	if !strings.Contains(segs[2].content, "新的剧情片段开始了。") || !strings.Contains(segs[2].content, "今天发生了两件事，应该记下来。") {
		t.Fatalf("第十一章正文丢失: %q", segs[2].content)
	}
	// 普通正文句子不以「第X章」结尾时不应误判
	plain := splitContent("他翻到第十章看了看内容。\n后面还有一段。", "auto")
	if len(plain) != 1 {
		t.Fatalf("正文中的「第十章」不应被识别为标题，得到 %d 段", len(plain))
	}
}

// TestSplitDefaultAppendKeepsOldChapters 回归测试（数据保护）：
// 默认（replace=false）分章必须追加且绝不删改已有章节；
// replace=true 才软删旧章节。防止再次发生「导入把用户原有章节删掉」的事故。
func TestSplitDefaultAppendKeepsOldChapters(t *testing.T) {
	db, err := Open(context.Background(), ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := NewStore(db)
	ctx := context.Background()

	p, err := store.CreateProject(ctx, "追加测试", "novel")
	if err != nil {
		t.Fatal(err)
	}
	// 先建 1 个已有章节
	old, err := store.CreateChapterWithVersion(ctx, p.ID, "", "原有第一章", "这是用户原有章节内容，必须保留。")
	if err != nil {
		t.Fatal(err)
	}

	// 默认追加：2 个新段 → 旧章节不能被软删，新章节序号从 2 开始
	longA := strings.Repeat("新章节一的正文内容，描写场景与人物动作。", 8) // >100 字，避免小段合并
	longB := strings.Repeat("新章节二的正文内容，推进剧情与对话。", 8)
	req := &SplitChaptersRequest{
		ProjectID: p.ID,
		Content:   "## 新章节一\n" + longA + "\n## 新章节二\n" + longB,
		SplitBy:   "## ",
	}
	chs, err := store.SplitChapters(ctx, req)
	if err != nil {
		t.Fatalf("SplitChapters 失败: %v", err)
	}
	if len(chs) != 2 {
		t.Fatalf("期望追加 2 章，得到 %d", len(chs))
	}
	all, err := store.ListChapters(ctx, p.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	var active, softDeleted int
	var orders []int
	for _, c := range all {
		if c.IsDeleted == 1 {
			softDeleted++
		} else {
			active++
			orders = append(orders, c.SortOrder)
		}
	}
	if active != 3 {
		t.Fatalf("追加后应有 3 个活跃章节，得到 %d（旧章节被误删！）", active)
	}
	if softDeleted != 0 {
		t.Fatalf("默认追加不应软删任何章节，得到 %d 条回收站", softDeleted)
	}
	// 旧章节 id 仍在活跃列表中
	found := false
	for _, c := range all {
		if c.IsDeleted == 0 && c.ID == old.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("原有章节被删除，数据保护失效")
	}
	if orders[0] != 1 || orders[1] != 2 || orders[2] != 3 {
		t.Fatalf("追加模式 sort_order 应顺延 1,2,3，得到 %v", orders)
	}

	// 显式 replace=true：旧章节软删，新章节从 1 开始
	req.Replace = true
	chs2, err := store.SplitChapters(ctx, req)
	if err != nil {
		t.Fatalf("replace 分章失败: %v", err)
	}
	if len(chs2) != 2 {
		t.Fatalf("期望 2 章，得到 %d", len(chs2))
	}
	// ListChapters 只返回活跃章节，回收站统计直接查库
	var active2, deleted2 int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM chapters WHERE project_id=? AND is_deleted=0`, p.ID).Scan(&active2); err != nil {
		t.Fatal(err)
	}
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM chapters WHERE project_id=? AND is_deleted=1`, p.ID).Scan(&deleted2); err != nil {
		t.Fatal(err)
	}
	if active2 != 2 || deleted2 != 3 {
		t.Fatalf("replace 后期望 2 活跃/3 回收站，得到 %d 活跃/%d 回收站", active2, deleted2)
	}

	// 预览模式：只计算不写库（前端「预览结果」按钮必须走此路径）
	req.Preview = true
	prev, err := store.SplitChapters(ctx, req)
	if err != nil {
		t.Fatalf("预览失败: %v", err)
	}
	if len(prev) != 2 {
		t.Fatalf("预览期望 2 章，得到 %d", len(prev))
	}
	var active3 int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM chapters WHERE project_id=? AND is_deleted=0`, p.ID).Scan(&active3); err != nil {
		t.Fatal(err)
	}
	if active3 != 2 {
		t.Fatalf("预览不应写库：期望仍 2 活跃，得到 %d", active3)
	}
}

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
