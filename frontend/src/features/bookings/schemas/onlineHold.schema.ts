import { z } from 'zod';

/**
 * Hours a visitor is booking online, as `GET /complexes/{id}/bookings` carries
 * them in `online_holds`: where and when, and when they free up if nobody pays.
 */
export const onlineHoldSchema = z.object({
  court_id: z.string(),
  date: z.string(),
  start_time: z.string(),
  end_time: z.string(),
  duration_minutes: z.number(),
  frees_at: z.string(),
});

export type OnlineHold = z.infer<typeof onlineHoldSchema>;

const onlineHoldsSchema = z.array(onlineHoldSchema);

/**
 * The day's online holds out of a bookings list response. A response without
 * them (an older backend), or with a malformed list, reads as none: the grid
 * then draws the hours as free, which is what it did before holds were sent.
 */
export function parseOnlineHolds(value: unknown): OnlineHold[] {
  const parsed = onlineHoldsSchema.safeParse(value);
  return parsed.success ? parsed.data : [];
}
