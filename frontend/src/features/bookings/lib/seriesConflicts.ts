import { HTTPError } from 'ky';
import type { BookingSeriesConflict, BookingSeriesFreeCourt } from '@/shared/types/api.types';

const REASONS = new Set<BookingSeriesConflict['reason']>([
  'cancelled_by_owner',
  'overlap',
  'blocked',
  'closed_day',
  'no_price',
  'client_blocked',
  'missed',
]);

const HELD_BY = new Set<NonNullable<BookingSeriesConflict['held_by']>>([
  'booking',
  'fixed',
  'event',
  'match',
  'online_hold',
]);

function isFreeCourt(value: unknown): value is BookingSeriesFreeCourt {
  if (typeof value !== 'object' || value === null) return false;
  const { court_id, court_name } = value as { court_id?: unknown; court_name?: unknown };
  return typeof court_id === 'string' && typeof court_name === 'string';
}

function isConflict(value: unknown): value is BookingSeriesConflict {
  if (typeof value !== 'object' || value === null) return false;
  const { date, reason, held_by, free_courts } = value as {
    date?: unknown;
    reason?: unknown;
    held_by?: unknown;
    free_courts?: unknown;
  };
  const heldByOk = held_by === undefined || (typeof held_by === 'string' && HELD_BY.has(held_by as never));
  const freeOk = free_courts === undefined || (Array.isArray(free_courts) && free_courts.every(isFreeCourt));
  return typeof date === 'string' && typeof reason === 'string' && REASONS.has(reason as never) && heldByOk && freeOk;
}

/**
 * The dates a 409 from the create-series endpoint refused, read off the
 * problem body's `conflicts` extension, each with the courts free that day. Empty when the error is anything else
 * (including a 409 that names no date, such as a blocked client), so the
 * caller falls back to the generic error toast.
 */
export function seriesConflictsFrom(error: unknown): BookingSeriesConflict[] {
  if (!(error instanceof HTTPError) || error.response.status !== 409) return [];
  const data: unknown = error.data;
  if (typeof data !== 'object' || data === null) return [];
  const { conflicts } = data as { conflicts?: unknown };
  return Array.isArray(conflicts) ? conflicts.filter(isConflict) : [];
}
