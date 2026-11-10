import './zodConfig';
import { z } from 'zod';
import { safeSessionStorage } from '@vibe/ui';
import { bookingSlotInfoSchema } from '../components/booking-form/types';
import { bookingInfoSchema } from '../components/booking-confirmed/types';

/**
 * Data that moves between booking pages without router state.
 *
 * Each page is its own document now, so the state a router used to carry from
 * one step to the next lives in sessionStorage, one entry per slug. It survives
 * a refresh, as `history.state` used to. Every read validates the stored value
 * with a schema, because anything in sessionStorage can be edited by hand.
 */

const bookingResultSchema = z.object({
  token: z.string().min(1),
  bookingInfo: bookingInfoSchema.optional(),
});

export type BookingResult = z.infer<typeof bookingResultSchema>;

/**
 * The confirm draft: the slot, plus the hold the complex page took on it when
 * the court was picked. Without a hold the confirm page has nothing to convert,
 * so a draft missing one is no draft at all.
 */
const confirmDraftSchema = bookingSlotInfoSchema.extend({
  holdToken: z.string().min(1),
  /** The server's `expires_at`, an RFC 3339 instant. */
  holdExpiresAt: z.string().refine((value) => !Number.isNaN(Date.parse(value))),
});

export type ConfirmDraft = z.infer<typeof confirmDraftSchema>;

export function confirmDraftKey(slug: string): string {
  return `vibe_booking_confirm_draft:${slug}`;
}

export function bookingResultKey(slug: string): string {
  return `vibe_booking_result:${slug}`;
}

/** The slot the person chose on the complex page, and its hold, waiting for the confirm page. */
export function saveConfirmDraft(slug: string, draft: ConfirmDraft): void {
  safeSessionStorage.set(confirmDraftKey(slug), JSON.stringify(draft));
}

export function readConfirmDraft(slug: string): ConfirmDraft | null {
  return safeSessionStorage.getJSON(confirmDraftKey(slug), confirmDraftSchema);
}

export function clearConfirmDraft(slug: string): void {
  safeSessionStorage.remove(confirmDraftKey(slug));
}

/** What the success page shows: the booking's token and, when known, its summary. */
export function saveBookingResult(slug: string, result: BookingResult): void {
  safeSessionStorage.set(bookingResultKey(slug), JSON.stringify(result));
}

export function readBookingResult(slug: string): BookingResult | null {
  return safeSessionStorage.getJSON(bookingResultKey(slug), bookingResultSchema);
}

export function clearBookingResult(slug: string): void {
  safeSessionStorage.remove(bookingResultKey(slug));
}

function holdExpiredNoticeKey(slug: string): string {
  return `vibe_booking_hold_expired:${slug}`;
}

/**
 * Left by the confirm page when the hold ran out, for the complex page it sends
 * the visitor back to. A toast would not survive the full page load between them.
 */
export function saveHoldExpiredNotice(slug: string): void {
  safeSessionStorage.set(holdExpiredNoticeKey(slug), '1');
}

/** Whether a hold-expired notice is waiting for the complex page. */
export function hasHoldExpiredNotice(slug: string): boolean {
  return safeSessionStorage.get(holdExpiredNoticeKey(slug)) !== null;
}

/** Removes the notice once shown, so it shows once. */
export function clearHoldExpiredNotice(slug: string): void {
  safeSessionStorage.remove(holdExpiredNoticeKey(slug));
}
