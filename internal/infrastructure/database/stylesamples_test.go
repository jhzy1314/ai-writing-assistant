package database

import (
	"context"
	"testing"
)

// TestImportStyleSamples 批量导入：空内容跳过、重复导入幂等、字段正确
func TestImportStyleSamples(t *testing.T) {
	db, err := Open(context.Background(), ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := NewStore(db)
	ctx := context.Background()

	samples := []StyleSample{
		{Title: "龙族·第1章", Author: "江南", Category: "青春幻想/热血", SourceFile: "lz.txt", Content: "在他的心底深处，他一直痛恨自己没有胆量跟父亲一起死在那个雨夜里。"},
		{Title: "诡秘之主·第1章", Author: "爱潜水的乌贼", Category: "克苏鲁悬疑", SourceFile: "gm.txt", Content: "痛！好痛！头好痛！光怪陆离满是低语的梦境迅速支离破碎。"},
		{Title: "空内容", Author: "无", Content: "   \n  "}, // 应跳过
	}
	n, err := store.ImportStyleSamples(ctx, samples)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("应导入 2 条（跳过空内容），实际 %d", n)
	}
	items, err := store.ListStyleSamples(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("列表应有 2 条，实际 %d", len(items))
	}
	byTitle := map[string]StyleSample{}
	for _, m := range items {
		byTitle[m.Title] = m
	}
	if m, ok := byTitle["龙族·第1章"]; !ok || m.Author != "江南" || m.Category != "青春幻想/热血" {
		t.Fatalf("龙族样本入库异常: %+v", m)
	}
	if m, ok := byTitle["诡秘之主·第1章"]; !ok || m.Author != "爱潜水的乌贼" {
		t.Fatalf("诡秘之主样本入库异常: %+v", m)
	}

	// 幂等：重复导入同 source_file+title 应全部跳过
	n2, err := store.ImportStyleSamples(ctx, samples)
	if err != nil {
		t.Fatal(err)
	}
	if n2 != 0 {
		t.Fatalf("重复导入应 0 条，实际 %d", n2)
	}
	// 同 title 不同 source_file 可重复（不同书同章名不冲突）
	dup := []StyleSample{{Title: "龙族·第1章", Author: "江南", Category: "青春幻想/热血", SourceFile: "lz2.txt", Content: "不同来源的同章名样本应可导入。"}}
	n3, err := store.ImportStyleSamples(ctx, dup)
	if err != nil {
		t.Fatal(err)
	}
	if n3 != 1 {
		t.Fatalf("不同 source_file 应导入 1 条，实际 %d", n3)
	}
}

// TestReplaceStyleSamples 原子重建：只删本次 source_file 行、保留其它行与手工样本、空内容跳过
func TestReplaceStyleSamples(t *testing.T) {
	db, err := Open(context.Background(), ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := NewStore(db)
	ctx := context.Background()

	// 预置：两本书（lz.txt 两行）+ 手工样本（source_file 为空）
	base := []StyleSample{
		{Title: "龙族·第1章", Author: "江南", Category: "热血", SourceFile: "lz.txt", Content: "旧版第一章内容。"},
		{Title: "龙族·第2章", Author: "江南", Category: "热血", SourceFile: "lz.txt", Content: "旧版第二章内容。"},
		{Title: "手工样本", Author: "用户", Category: "其他", SourceFile: "", Content: "用户手动添加的样本不应被重建删除。"},
	}
	if _, err := store.ImportStyleSamples(ctx, base); err != nil {
		t.Fatal(err)
	}

	// 重建 lz.txt：新内容替换旧内容
	rebuilt := []StyleSample{
		{Title: "龙族·第1章", Author: "江南", Category: "热血", SourceFile: "lz.txt", Content: "新版第一章内容。"},
		{Title: "空内容", Author: "江南", SourceFile: "lz.txt", Content: "   "},
	}
	n, err := store.ReplaceStyleSamples(ctx, rebuilt)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("重建应插入 1 条（空内容跳过），实际 %d", n)
	}
	items, _ := store.ListStyleSamples(ctx, "")
	if len(items) != 2 {
		t.Fatalf("重建后应有 2 条（1 新版 + 1 手工），实际 %d: %+v", len(items), items)
	}
	byTitle := map[string]string{}
	for _, m := range items {
		byTitle[m.Title] = m.Content
	}
	if byTitle["龙族·第1章"] != "新版第一章内容。" {
		t.Fatalf("旧行应被替换为新内容: %+v", byTitle)
	}
	if byTitle["手工样本"] == "" {
		t.Fatal("手工样本（无 source_file）应被保留")
	}
}
