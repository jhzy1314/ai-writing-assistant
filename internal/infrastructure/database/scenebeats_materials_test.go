package database

import (
	"context"
	"testing"
)

func setupProject(t *testing.T, store *Store, name string) string {
	t.Helper()
	p, err := store.CreateProject(context.Background(), name, "novel")
	if err != nil {
		t.Fatal(err)
	}
	return p.ID
}

// TestSceneBeatSortOrder 验证场景卡自动递增 sort_order（单条 INSERT 原子计算）
func TestSceneBeatSortOrder(t *testing.T) {
	db, err := Open(context.Background(), ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := NewStore(db)
	ctx := context.Background()
	pid := setupProject(t, store, "场景卡测试")
	ch, err := store.CreateChapter(ctx, pid, "", "第一章", "内容")
	if err != nil {
		t.Fatal(err)
	}

	b1, err := store.CreateSceneBeat(ctx, SceneBeat{ProjectID: pid, ChapterID: ch.ID, Title: "场景A"})
	if err != nil {
		t.Fatal(err)
	}
	b2, err := store.CreateSceneBeat(ctx, SceneBeat{ProjectID: pid, ChapterID: ch.ID, Title: "场景B"})
	if err != nil {
		t.Fatal(err)
	}
	if b1.SortOrder != 1 || b2.SortOrder != 2 {
		t.Fatalf("sort_order 应自动递增 1,2，得到 %d,%d", b1.SortOrder, b2.SortOrder)
	}
	// 显式 sort_order 优先
	b3, err := store.CreateSceneBeat(ctx, SceneBeat{ProjectID: pid, ChapterID: ch.ID, Title: "场景C", SortOrder: 10})
	if err != nil {
		t.Fatal(err)
	}
	if b3.SortOrder != 10 {
		t.Fatalf("显式 sort_order 应为 10，得到 %d", b3.SortOrder)
	}
	items, err := store.ListSceneBeats(ctx, ch.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 {
		t.Fatalf("期望 3 个场景卡，得到 %d", len(items))
	}
}

// TestWritingMaterialVectorUpdate 验证向量与内容原子更新
func TestWritingMaterialVectorUpdate(t *testing.T) {
	db, err := Open(context.Background(), ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := NewStore(db)
	ctx := context.Background()
	pid := setupProject(t, store, "素材向量测试")

	m, err := store.CreateWritingMaterial(ctx, WritingMaterial{ProjectID: pid, Category: "句式", Content: "她攥紧衣角", Vector: "old-vec"})
	if err != nil {
		t.Fatal(err)
	}
	// 只改类别：向量不动
	cat := "动作描写"
	upd, err := store.UpdateWritingMaterialWithVector(ctx, m.ID, &cat, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if upd.Category != "动作描写" || upd.Vector != "old-vec" {
		t.Fatalf("只改类别时向量应保留: %+v", upd)
	}
	// 改内容 + 新向量：原子更新
	newContent := "他眯起眼睛"
	newVec := "new-vec"
	upd2, err := store.UpdateWritingMaterialWithVector(ctx, m.ID, nil, &newContent, newVec)
	if err != nil {
		t.Fatal(err)
	}
	if upd2.Content != newContent || upd2.Vector != "new-vec" {
		t.Fatalf("内容+向量应同时更新: %+v", upd2)
	}
	// 类别过滤
	items, err := store.ListWritingMaterials(ctx, pid, "动作描写")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("类别过滤应命中 1 条，得到 %d", len(items))
	}
	// 不存在 id 更新 → ErrNoRows 语义（GetWritingMaterial 返回错误）
	if _, err := store.UpdateWritingMaterialWithVector(ctx, "no-such-id", nil, nil, ""); err == nil {
		t.Fatal("更新不存在的素材应报错")
	}
}
