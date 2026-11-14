import { HTTPError } from 'ky';
import type { BookingSeriesConflict } from '@/shared/types/api.types';

const REASONS = new Set<BookingSeriesConflict['reason']>([
  'cancelled_by_owner',
  'overlap',
  'blocked',
  'closed_day',
  'no_price',
  'client_blocked',
  'missed',
]);

function isConflict(value: unknown): value is BookingSeriesConflict {
  if (typeof value !== 'object' || value === null) return false;
  const { date, reason } = value as { date?: unknown; reason?: unknown };
  return typeof date === 'string' && typeof reason === 'string' && REASONS.has(reason as never);
}

/**
 * The dates a 409 from the create-series endpoint refused, read off the
 * problem body's `conflicts` extension. Empty when the error is anything else
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
