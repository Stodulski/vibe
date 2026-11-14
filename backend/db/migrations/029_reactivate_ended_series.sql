-- +goose Up

SET LOCAL lock_timeout = '3s';
SET LOCAL app.bypass_tenant = 'on';

-- ============================================================================
-- FIXED WEEKLY SERIES ENDED TOO EARLY
-- ============================================================================
-- "Cancelar turno fijo" from a later date set the series' last day and also
-- marked it 'ended' at once. booking_series_occurrences() only reads active
-- series, so the dates between today and that last day stopped holding their
-- court and the daily job stopped booking them. The service now keeps the
-- series active while a date of it is left; this puts back the series ended
-- that way. A series is ended correctly when its last day is already behind
-- the venue's today or before its first date, and those rows are untouched.
UPDATE booking_series
SET status = 'active'
WHERE status = 'ended'
  AND ends_on IS NOT NULL
  AND ends_on >= starts_on
  AND ends_on >= (now() AT TIME ZONE 'America/Argentina/Buenos_Aires')::date;

-- +goose Down

-- Nothing to undo: the rows it touched were ended by mistake, and the
-- materialization job ends each of them once its last day passes.
SELECT 1;
