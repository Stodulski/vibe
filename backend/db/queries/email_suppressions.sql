-- name: InsertEmailSuppression :exec
-- Idempotent: a second unsubscribe of the same address is a no-op.
INSERT INTO email_suppressions (email, reason)
VALUES (sqlc.arg(email)::text, sqlc.arg(reason)::text)
ON CONFLICT (email) DO NOTHING;
