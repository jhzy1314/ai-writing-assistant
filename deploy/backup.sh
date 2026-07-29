#!/bin/bash
set -e
DB="/opt/ai-novel-studio/data/ai-novel.db"
BACKUP_DIR="/opt/ai-novel-studio/data/backups"
KEEP=30

mkdir -p "$BACKUP_DIR"
cp "$DB" "$BACKUP_DIR/ai-novel-$(date +%F-%H%M%S).db"
echo "[$(date '+%F %T')] Backup OK: $(ls $BACKUP_DIR | wc -l) files"

# 清理旧备份
cd "$BACKUP_DIR" && ls -t | tail -n +$((KEEP+1)) | xargs -r rm
