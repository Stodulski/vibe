import { z } from 'zod';

/**
 * `BookingSlotInfo` used to be a plain interface, cast into from
 * `location.state` (router history, which survives refresh and can carry
 * anything a previous navigation put there) with `as BookingSlotInfo`. The
 * schema is now the single source of truth: `BookConfirmPage` validates the
 * router state against it with `safeParse` instead of trusting the cast.
 */
export const bookingSlotInfoSchema = z.object({
  complexId: z.string().min(1),
  complexName: z.string().min(1),
  complexPhone: z.string(),
  courtId: z.string().min(1),
  courtName: z.string().min(1),
  /** What was chosen, so the summary can say it and the back link can restore it. */
  sport: z.string().optional(),
  courtType: z.string().optional(),
  courtDescription: z.string().optional(),
  date: z.string().min(1),
  startTime: z.string().min(1),
  endTime: z.string().min(1),
  durationMinutes: z.number(),
  price: z.number(),
  depositPercentage: z.number(),
  cancellationHours: z.number(),
  /**
   * Service fee in centavos as returned by the backend.
   * When provided, it is used directly instead of the local estimate so that
   * the frontend and backend never diverge silently.
   * If absent (e.g. before a quote endpoint exists), the component falls back
   * to the local estimate: max(round(mpAmount * 7%), 1000).
   */
  serviceFee: z.number().optional(),
});

export type BookingSlotInfo = z.infer<typeof bookingSlotInfoSchema>;
