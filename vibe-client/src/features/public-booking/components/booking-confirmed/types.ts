import { z } from 'zod';
import type { Sport, CourtType } from '@/shared/types/api.types';

// Tied to the shared `Sport`/`CourtType` unions with `satisfies` rather than
// re-declared blind: if either shared union ever changes, this array stops
// compiling instead of silently drifting out of sync.
const SPORTS = ['padel', 'tennis', 'soccer', 'basketball'] as const satisfies readonly Sport[];
const COURT_TYPES = ['indoor', 'outdoor', 'semi_covered'] as const satisfies readonly CourtType[];

/**
 * The cancellation window as `GET /book/status` reports it right now
 * (mirrors `BookingStatusCancellation`, in camelCase like the rest of this
 * shape). Absent whenever the API response predates it — the "before/after"
 * generic copy driven by `cancellationHours` above is what renders instead.
 */
const bookingCancellationSchema = z.object({
  canCancel: z.boolean(),
  /** RFC3339, Argentina offset. Null when cancellable with refund right up to the turn itself. */
  refundDeadline: z.string().nullable(),
  canRefundNow: z.boolean(),
});

export type BookingCancellation = z.infer<typeof bookingCancellationSchema>;

/**
 * `BookingInfo` used to be a plain interface, cast into from sessionStorage
 * with `as BookingInfo` after only a `typeof === 'object'` check. sessionStorage
 * is editable and its shape can change between deploys, so the schema is now
 * the single source of truth: `readStoredBookingInfo` validates against it
 * with `safeParse` and returns `null` on anything that doesn't match.
 */
export const bookingInfoSchema = z.object({
  courtName: z.string(),
  date: z.string(),
  startTime: z.string(),
  /** RFC3339 instant, Argentina offset — the pair below is what gets rendered. */
  startsAt: z.string(),
  /**
   * RFC3339 instant, Argentina offset. It replaced an `HH:MM` end, which could
   * not say that a 23:00 booking of two hours finishes on the following day —
   * and the confirmation screen is where the customer reads their hours back.
   *
   * On a copy cached before payment this is built from the picked slot
   * (`venueInstant`); once `GET /book/status` answers, its own `ends_at` wins.
   */
  endsAt: z.string(),
  price: z.number(),
  depositAmount: z.number(),
  complexName: z.string(),
  complexPhone: z.string(),
  cancellationHours: z.number(),
  clientPhone: z.string(),
  /** API-only fields below — absent on a `BookingInfo` built from the pre-booking form and cached to sessionStorage, present once `GET /book/status` has answered with them. */
  complexAddress: z.string().optional(),
  sport: z.enum(SPORTS).optional(),
  courtType: z.enum(COURT_TYPES).optional(),
  /** Centavos. */
  serviceFee: z.number().optional(),
  /** Centavos. */
  remainingAmount: z.number().optional(),
  cancellation: bookingCancellationSchema.optional(),
});

export type BookingInfo = z.infer<typeof bookingInfoSchema>;
