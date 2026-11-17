import { getProblem } from '../../lib/ApiError';
import { BOOKING_MESSAGES } from '../../messages';

const t = BOOKING_MESSAGES.waitlist;

/** What the popup says when the server refuses the join: the place freed, the slot is gone, too many waits. */
export function waitlistRefusal(error: unknown): string {
  const problem = getProblem(error);
  if (problem?.status === 429) return t.tooMany;
  const messages = problem?.errors.map((item) => item.message).join(' ') ?? '';
  if (messages.includes('is free now')) return t.placeFree;
  if (messages.includes('already waiting')) return t.capReached;
  if (messages.includes('not a slot') || messages.includes('must be in the future')) return t.slotGone;
  return t.error;
}
