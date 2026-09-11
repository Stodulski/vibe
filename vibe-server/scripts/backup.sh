#!/usr/bin/env bash
set -euo pipefail

DB_URL="${DATABASE_URL:?DATABASE_URL environment variable is required}"
BACKUP_DIR="${BACKUP_DIR:-./backups}"
RETENTION_DAYS=7

mkdir -p "$BACKUP_DIR"

TIMESTAMP=$(date +%Y-%m-%d_%H%M%S)
FILENAME="backup_${TIMESTAMP}.dump"
FILEPATH="${BACKUP_DIR}/${FILENAME}"

echo "[backup] Starting database backup..."
pg_dump "$DB_URL" --format=custom --no-owner --no-acl -f "$FILEPATH"
echo "[backup] Saved to $FILEPATH ($(du -h "$FILEPATH" | cut -f1))"

# Clean old backups
find "$BACKUP_DIR" -name "backup_*.dump" -mtime +$RETENTION_DAYS -delete 2>/dev/null || true
REMAINING=$(find "$BACKUP_DIR" -name "backup_*.dump" | wc -l)
echo "[backup] Done. $REMAINING backup(s) retained."
