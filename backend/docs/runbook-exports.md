# Runbook — payments exports

The owner's monthly Excel export is built by a background job and left in a
**private** R2 bucket. The API hands the browser a presigned GET that lives
fifteen minutes; nothing else can read the file.

- `POST /api/v1/complexes/{id}/reports/exports` queues one and answers `202`
  with an id and a `status_url`.
- `GET /api/v1/complexes/{id}/reports/exports/{exportID}` reports it, and once
  it is `done` carries `download_url` and `expires_at`.

The old synchronous `GET .../reports/export` still works and is marked
`deprecated` in the OpenAPI document. It is removed once the frontend has
switched.

## What the operator has to configure

### 1. A second, private bucket

Set `R2_PRIVATE_BUCKET_NAME` to a bucket that has **no** public r2.dev
subdomain and **no** custom domain attached. The existing `R2_BUCKET_NAME`
serves club logos and cover images by public URL, and R2's public access is a
property of the whole bucket — so a prefix inside it would not be private
however unguessable the key is. The payload here is a month of one club's
ledger: client names, phone numbers and amounts.

The same `R2_ACCOUNT_ID` / `R2_ACCESS_KEY` / `R2_SECRET_KEY` are used, so the
token must be scoped to both buckets.

With the variable unset, both export endpoints answer `501` and nothing else
changes.

### 2. A 24 hour lifecycle rule on `exports/`

**This is not enforced by the application.** Objects are written to
`exports/{complexID}/{exportID}.xlsx` and nothing deletes them.

In the Cloudflare dashboard: **R2 → the private bucket → Settings → Object
lifecycle rules → Add rule**

| Field | Value |
| --- | --- |
| Rule name | `expire-exports` |
| Prefix | `exports/` |
| Action | Delete uploaded objects |
| After | 1 day |

Equivalent with the S3 API:

```bash
aws s3api put-bucket-lifecycle-configuration \
  --endpoint-url "https://$R2_ACCOUNT_ID.r2.cloudflarestorage.com" \
  --bucket "$R2_PRIVATE_BUCKET_NAME" \
  --lifecycle-configuration '{
    "Rules": [{
      "ID": "expire-exports",
      "Status": "Enabled",
      "Filter": {"Prefix": "exports/"},
      "Expiration": {"Days": 1}
    }]
  }'
```

The API already assumes this rule exists: a status read more than 24 hours
after the export finished answers `410 Gone` with `export_expired` instead of
signing a URL for an object that is not there. The job row itself is kept for
seven days (the queue's retention), which is why the two numbers have to be
reconciled somewhere, and this is where.

Without the rule nothing breaks for the owner — the 410 still fires — but the
bucket grows forever and every stale workbook is a copy of a club's ledger
that nobody meant to keep.

## Reading the queue

Exports are ordinary rows in `jobs`, type `export:payments`:

```sql
SELECT id, status, attempts, last_error, created_at, updated_at
FROM jobs
WHERE type = 'export:payments'
ORDER BY created_at DESC
LIMIT 20;
```

- `failed` is the dead letter. Nothing retries it, and the owner's page shows
  the machine code behind `last_error` (`report_export_too_large`, otherwise
  `report_export_failed`).
- The owner can ask again: a `POST` that finds a `failed` row for the same
  complex, period and Argentina day frees its deduplication key and queues a
  new job. The dead row stays as the record of what failed.
- A `pending` row that never moves means no instance has the `export:payments`
  handler registered — an old build still serving.

## For the frontend migration

The synchronous route is still there, so nothing is broken today. What has to
change, and what does not:

- `frontend/src/features/dashboard/api/dashboard.api.ts` — `exportPaymentsExcel`
  is the one request-level caller. It becomes a `POST` to
  `/api/v1/complexes/{id}/reports/exports`, then a poll of the `status_url` the
  `202` returns, then a plain navigation to `download_url`.
- `frontend/src/features/dashboard/pages/reports/useReportExport.ts` — the hook that
  drives the `Descargar Excel` button. It gains the polling state; the button's
  own contract (visible, disabled while working) does not change.
- `frontend/src/test/msw/handlers.ts` — needs the two new handlers.
- `frontend/e2e/specs/reports.spec.ts` — **no change**. Its four tests never
  issue an export request; `shows export button` only asserts the button is
  visible. So the backend and frontend changes do not have to merge together.

Three response shapes the client has to handle, none of which the synchronous
route had:

| Case | Answer |
| --- | --- |
| The job failed | `200`, `status: "failed"`, and an embedded `error` problem whose `detail` is the machine code to map to copy. Not a 4xx: reading the status succeeded. |
| The file is older than 24h | `410` with `export_expired`. Ask again — a new `POST` builds a fresh one. |
| No private bucket configured | `501`. Only a misconfigured deployment. |

A second `POST` for the same complex, period and Argentina day answers with the
first export's id, so a double click costs nothing and needs no button
debouncing of its own.

## Things that are deliberately absent

- **No `exports` table.** The job row is the state machine and the object key
  is derived from its payload, so there is no second write that can fail after
  the upload succeeded.
- **No stored `expires_at`.** The download URL is signed on every status read,
  which costs a local HMAC and is always truthful about its own expiry.
- **No cleanup job.** The lifecycle rule above is the deletion path; a sweep
  of our own would be a second, weaker copy of it.
