package database

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestBackupTo 验证一致性快照备份：备份文件可独立打开且数据完整
func TestBackupTo(t *testing.T) {
	db, err := Open(context.Background(), ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := NewStore(db)
	ctx := context.Background()

	p, err := store.CreateProject(ctx, "备份测试", "novel")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateChapter(ctx, p.ID, "", "第一章", "备份测试正文内容"); err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(t.TempDir(), "snapshot.db")
	if err := store.BackupTo(ctx, dst); err != nil {
		t.Fatal(err)
	}

	// 备份文件应能独立打开且数据完整
	backupDB, err := Open(ctx, dst)
	if err != nil {
		t.Fatalf("备份文件无法打开: %v", err)
	}
	defer backupDB.Close()
	bstore := NewStore(backupDB)

	projs, err := bstore.ListProjects(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(projs) != 1 || projs[0].Name != "备份测试" {
		t.Fatalf("备份中项目异常: %+v", projs)
	}
	chs, err := bstore.ListChapters(ctx, p.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(chs) != 1 || chs[0].Content != "备份测试正文内容" {
		t.Fatalf("备份中章节异常: %+v", chs)
	}

	// 目标已存在且非空时应报错（VACUUM INTO 语义）
	if err := store.BackupTo(ctx, dst); err == nil {
		t.Fatal("对已存在的非空目标备份应报错")
	}
	// 目标存在但为空文件时允许（SQLite 语义：可写入空文件）
	emptyDst := filepath.Join(t.TempDir(), "empty.db")
	if f, err := os.Create(emptyDst); err != nil {
		t.Fatal(err)
	} else {
		f.Close()
	}
	if err := store.BackupTo(ctx, emptyDst); err != nil {
		t.Fatalf("空文件目标应允许: %v", err)
	}
}
