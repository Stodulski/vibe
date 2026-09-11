import type { Client } from '@/shared/types/api.types';

/**
 * The share of a client's bookings they actually turned up for, as a percentage.
 *
 * A client with no bookings yet is 100%, not 0%: they have not missed anything.
 * Scoring them as if they had would put a brand new client in the same bucket
 * as one who no-showed five times.
 *
 * Extracted because two components were each computing it from scratch, which
 * is one edit away from the card and the list disagreeing about the same
 * person.
 */
export function attendancePct(client: Client): number {
  if (client.total_bookings <= 0) return 100;
  return Math.round(((client.total_bookings - client.no_shows) / client.total_bookings) * 100);
}

/**
 * The tone attendance is shown in. A real quality state, so it earns the
 * success/warning/error tokens rather than the brand colour.
 */
export function attendanceTone(pct: number): { text: string; bar: string } {
  if (pct >= 80) return { text: 'text-success-text', bar: 'bg-success-text' };
  if (pct >= 50) return { text: 'text-warning-text', bar: 'bg-warning-text' };
  return { text: 'text-error-text', bar: 'bg-error-text' };
}
