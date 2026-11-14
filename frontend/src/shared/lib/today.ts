import { VENUE_TIME_ZONE } from '@/shared/lib/instants';

/**
 * "Now", read in the venue's timezone (Argentina) instead of the runtime's
 * own clock. Every complex on the platform is in Argentina — see
 * `VENUE_TIME_ZONE` — so a date/time decision computed on the browser's or
 * CI's own zone drifts from the server's the moment either isn't in it.
 */
export function nowInArgentina(): Date {
  return new Date(new Date().toLocaleString('en-US', { timeZone: VENUE_TIME_ZONE }));
}

/**
 * Today's date ("YYYY-MM-DD"), in the venue's timezone.
 *
 * Between 21:00 and 00:00 Argentina time, `new Date().toISOString()` (UTC)
 * already reads as tomorrow — the single source of the "three different
 * todays" bug this consolidates (see 02-bookings-clients.md M7).
 */
export function todayInArgentina(): string {
  const now = nowInArgentina();
  return `${String(now.getFullYear())}-${String(now.getMonth() + 1).padStart(2, '0')}-${String(now.getDate()).padStart(2, '0')}`;
}
