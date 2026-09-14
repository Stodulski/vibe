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
# Ownership and grants are kept in the archive on purpose: vibe_app's access is
# only the GRANTs the ACCESS section of 001_init.sql issues, and a dump without
# them restores a database the API cannot read. Strip them at restore time with
# pg_restore --no-owner --no-privileges when the target cluster lacks the roles.
pg_dump "$DB_URL" --format=custom -f "$FILEPATH"
echo "[backup] Saved to $FILEPATH ($(du -h "$FILEPATH" | cut -f1))"

# Clean old backups
find "$BACKUP_DIR" -name "backup_*.dump" -mtime +$RETENTION_DAYS -delete 2>/dev/null || true
REMAINING=$(find "$BACKUP_DIR" -name "backup_*.dump" | wc -l)
echo "[backup] Done. $REMAINING backup(s) retained."
