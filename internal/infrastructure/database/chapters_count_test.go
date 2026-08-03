package database

import (
	"context"
	"testing"
)

// TestRecountAllWordCounts 验证 v5 迁移：按新口径（非空白字符数，含标点）批量重算章节字数，
// 修正旧口径/被改坏的历史数据。启动时（main.go）自动调用，幂等。
func TestRecountAllWordCounts(t *testing.T) {
	db, err := Open(context.Background(), ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := NewStore(db)
	ctx := context.Background()

	p, err := store.CreateProject(ctx, "字数迁移测试", "novel")
	if err != nil {
		t.Fatal(err)
	}
	content := "你好，世界！\n  换行 空格"
	ch, err := store.CreateChapter(ctx, p.ID, "", "第一章", content)
	if err != nil {
		t.Fatal(err)
	}
	want := wordCount(content)
	if ch.WordCount != want {
		t.Fatalf("新建章节 word_count=%d，期望 %d（新口径）", ch.WordCount, want)
	}

	// 模拟旧口径数据：故意改坏 word_count
	if _, err := db.ExecContext(ctx, `UPDATE chapters SET word_count=999 WHERE id=?`, ch.ID); err != nil {
		t.Fatal(err)
	}

	n, err := store.RecountAllWordCounts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n < 1 {
		t.Fatalf("期望至少重算 1 章，得到 %d", n)
	}
	var wc int
	if err := db.QueryRowContext(ctx, `SELECT word_count FROM chapters WHERE id=?`, ch.ID).Scan(&wc); err != nil {
		t.Fatal(err)
	}
	if wc != want {
		t.Fatalf("重算后 word_count=%d，期望 %d", wc, want)
	}

	// 幂等：再次执行不报错且数值不变
	if _, err := store.RecountAllWordCounts(ctx); err != nil {
		t.Fatalf("第二次执行不应报错: %v", err)
	}
	if err := db.QueryRowContext(ctx, `SELECT word_count FROM chapters WHERE id=?`, ch.ID).Scan(&wc); err != nil {
		t.Fatal(err)
	}
	if wc != want {
		t.Fatalf("第二次执行后 word_count=%d，期望不变 %d", wc, want)
	}
}
