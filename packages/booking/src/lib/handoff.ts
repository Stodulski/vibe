import { z } from 'zod';
import { safeSessionStorage } from '@vibe/ui';
import { bookingSlotInfoSchema, type BookingSlotInfo } from '../components/booking-form/types';
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

export function confirmDraftKey(slug: string): string {
  return `vibe_booking_confirm_draft:${slug}`;
}

export function bookingResultKey(slug: string): string {
  return `vibe_booking_result:${slug}`;
}

/** The slot the person chose on the complex page, waiting for the confirm page. */
export function saveConfirmDraft(slug: string, draft: BookingSlotInfo): void {
  safeSessionStorage.set(confirmDraftKey(slug), JSON.stringify(draft));
}

export function readConfirmDraft(slug: string): BookingSlotInfo | null {
  return safeSessionStorage.getJSON(confirmDraftKey(slug), bookingSlotInfoSchema);
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
