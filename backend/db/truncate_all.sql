-- Empty all tables (preserves schema, indexes, and migrations)
-- Usage: psql $DATABASE_URL -f db/truncate_all.sql

BEGIN;

TRUNCATE
    jobs,
    payments,
    failed_refunds,
    slot_locks,
    bookings,
    blocked_slots,
    court_prices,
    courts,
    complex_schedules,
    clients,
    audit_log,
    complexes,
    email_verification_tokens,
    password_reset_tokens,
    refresh_tokens,
    users
CASCADE;

COMMIT;
