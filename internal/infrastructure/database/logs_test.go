package database

import (
	"context"
	"testing"
)

// TestGenerationLogCacheTokens 验证 cache_hit_tokens 列在迁移后写入/读取往返一致
func TestGenerationLogCacheTokens(t *testing.T) {
	db, err := Open(context.Background(), ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := NewStore(db)
	ctx := context.Background()

	want := GenerationLog{
		ProjectID:        "p1",
		Role:             "worker",
		ModelName:        "deepseek-chat",
		PromptTokens:     6460,
		CompletionTokens: 500,
		CacheHitTokens:   6400, // 99% 命中场景
		DurationMs:       1200,
		Status:           "ok",
	}
	if err := store.InsertLog(ctx, want); err != nil {
		t.Fatal(err)
	}
	got, err := store.ListLogs(ctx, "p1", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("期望 1 条日志，得到 %d", len(got))
	}
	l := got[0]
	if l.CacheHitTokens != want.CacheHitTokens {
		t.Errorf("CacheHitTokens 往返不一致: 写入 %d 读取 %d", want.CacheHitTokens, l.CacheHitTokens)
	}
	if l.PromptTokens != want.PromptTokens || l.CompletionTokens != want.CompletionTokens {
		t.Errorf("token 字段不一致: %+v vs %+v", want, l)
	}
	if l.Role != want.Role || l.ModelName != want.ModelName {
		t.Errorf("角色/模型不一致: %+v vs %+v", want, l)
	}
}
