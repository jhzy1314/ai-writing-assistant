package database

import (
	"context"
	"fmt"
	"os"
	"strings"
)

// ============================================================
// backup.go —— SQLite 一致性快照备份
// 用 VACUUM INTO 生成一致性快照（WAL 模式下裸文件复制可能拿到
// 不一致副本：未 checkpoint 的已提交事务在 -wal 文件中，只拷 .db 会丢数据）
// ============================================================

// BackupTo 将数据库备份为一致性快照文件（VACUUM INTO）。
// 快照为紧凑单文件：包含全部已提交数据、清除空闲页空洞。
// 要求 dst 不存在（或为空文件）；路径中的单引号已做 SQL 字面量转义。
func (s *Store) BackupTo(ctx context.Context, dst string) error {
	if st, err := os.Stat(dst); err == nil && st.Size() > 0 {
		return fmt.Errorf("备份目标已存在且非空: %s", dst)
	}
	// VACUUM INTO 的路径是 SQL 字符串字面量，转义单引号防注入（路径虽来自代码，仍稳妥处理）
	quoted := strings.ReplaceAll(dst, "'", "''")
	_, err := s.db.ExecContext(ctx, "VACUUM INTO '"+quoted+"'")
	if err != nil {
		return fmt.Errorf("VACUUM INTO 失败: %w", err)
	}
	return nil
}
