# Postgres backups

Railway does not back up its managed Postgres automatically. `.github/workflows/backend-backup.yml`
runs `pg_dump` once a day and uploads the result to Cloudflare R2. This page is what to read to
restore a database, to set the workflow up, or to check whether it is actually working.

## Quick path

1. Backups run automatically, daily at 06:00 UTC, and land in `postgres/` in the R2 bucket named
   by the `R2_BACKUP_BUCKET` secret. Nothing to do day-to-day.
2. Need one right now (before a risky migration, before an incident)? Run the workflow manually:
   `gh workflow run backend-backup.yml` (or the "Run workflow" button on the Actions tab).
3. Need to restore? Jump to [Restore procedure](#restore-procedure) below.

## What runs, and where it lands

`.github/workflows/backend-backup.yml`, on a schedule (`cron: "0 6 * * *"`) and on manual dispatch:

1. Installs `postgresql-client-17` from the official PGDG apt repository (the GitHub-hosted
   runner's default Postgres client is not pinned to a version that is guaranteed `>=` the
   server's — `pg_dump` can only dump a server at or below its own major version, never above it).
2. Runs `pg_dump --format=custom --no-owner --no-privileges "$BACKUP_DATABASE_URL"` and fails the
   job if the output is suspiciously small (under 512 bytes — a real dump always carries at least
   the custom-format header, so anything smaller means `pg_dump` failed or the connection dropped).
3. Names the file `vibe-<UTC timestamp>-<short commit sha>.dump`, gzips it, and uploads it with
   `aws s3 cp` to `s3://$R2_BACKUP_BUCKET/postgres/` against R2's S3-compatible endpoint
   (`https://$R2_BACKUP_ACCOUNT_ID.r2.cloudflarestorage.com`).
4. Deletes the local file from the runner afterwards. The upload log never prints the destination
   URL (it embeds the account id and bucket name); it only confirms the byte count.

**Retention**: not set by this workflow. Recommended: an R2 **lifecycle rule** on the bucket that
expires objects under `postgres/` after 30 days. This has to be set in the R2 dashboard (or with
`aws s3api put-bucket-lifecycle-configuration` against the R2 endpoint) — there is no repository
config for it, and it is not set yet. Until it is, the bucket grows forever.

**Checking the Railway Postgres version**, needed if `postgresql-client-17` ever stops being
`>=` the server: Railway dashboard → the Postgres service → **Data** tab, or run
`SELECT version();` against `BACKUP_DATABASE_URL`. Bump the `postgresql-client-17` install step in
the workflow if Railway upgrades past major version 17.

## Restore procedure

Restoring into a **fresh** Railway Postgres service (never the live one, unless deliberately
overwriting it — `pg_restore` with these flags drops and recreates objects):

1. Create a new Postgres service in Railway (or reuse an empty one), and copy its connection
   string.
2. Download the dump you need from the R2 bucket (`postgres/vibe-<timestamp>-<sha>.dump.gz`) —
   the Cloudflare dashboard, or:
   ```bash
   aws s3 cp "s3://$R2_BACKUP_BUCKET/postgres/<file>.dump.gz" . \
     --endpoint-url "https://$R2_BACKUP_ACCOUNT_ID.r2.cloudflarestorage.com"
   ```
3. Decompress it: `gunzip <file>.dump.gz`.
4. Restore:
   ```bash
   pg_restore --clean --if-exists --no-owner --no-privileges \
     --dbname "$NEW_DATABASE_URL" <file>.dump
   ```
   `--clean --if-exists` drops existing objects first, so this is also how to roll a database
   fully back to the dump's point in time. `--no-owner --no-privileges` matches how the dump was
   taken: the restore target's own roles own the result, not `vibe_migrator`/`vibe_app` from
   whatever cluster the dump came from.
5. Verify: `psql "$NEW_DATABASE_URL" -c '\dt'` lists the expected tables, and
   `SELECT count(*) FROM bookings;` (or another high-traffic table) returns a plausible number.
6. Point `DATABASE_URL` (and `DB_MIGRATOR_URL` if the roles differ) at the restored database only
   after step 5 passes.

`backend/scripts/backup.sh` remains available as the manual, ad-hoc path (a local `pg_dump`, kept
for a quick dump against any reachable database without touching GitHub Actions or R2) — it is not
what the scheduled workflow uses.

## Restore drills

An untested backup is a hope, not a backup. Run a real restore into a scratch Railway Postgres on
a recurring basis and log it here.

| Date | Who | Outcome |
|------|-----|---------|
| — | — | pending: first drill |

## Secrets to create (GitHub → repository → Settings → Secrets and variables → Actions)

| Secret | Value | Least-privilege notes |
|--------|-------|------------------------|
| `BACKUP_DATABASE_URL` | A Postgres connection string with read access to the database being backed up | A dedicated read-only role is preferable to reusing `DATABASE_URL`/`DB_MIGRATOR_URL`; `pg_dump` needs `SELECT` on every object, nothing else |
| `R2_BACKUP_ACCOUNT_ID` | Cloudflare account id | Not secret by itself, but stored alongside the credentials that are |
| `R2_BACKUP_ACCESS_KEY_ID` | R2 API token access key id | Scope the R2 API token to this one bucket, **Object Read & Write** only (no Admin, no other buckets) |
| `R2_BACKUP_SECRET_ACCESS_KEY` | R2 API token secret | Same token as above |
| `R2_BACKUP_BUCKET` | Bucket name | Should not be the same bucket `R2_BUCKET_NAME` uses for client-uploaded images — keep backups and public assets in separate buckets so a bucket-scoped token for one can never touch the other |

None of these are configured by this PR — creating them in GitHub, scoping the R2 token, and
setting the lifecycle rule are all steps the project owner does outside the repository.
