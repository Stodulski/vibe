import { BOOKING_MESSAGES } from '../../messages';

const t = BOOKING_MESSAGES;

/**
 * How long a hold lasts: the backend's BOOKING_PAYMENT_EXPIRY, counted from the
 * hold's creation (backend/internal/bookings/hold.go). No countdown shows more.
 */
export const HOLD_WINDOW_SECONDS = 10 * 60;

/** Seconds left at which a screen reader hears how much time remains, largest first. */
const THRESHOLDS = [
  { seconds: 5 * 60, message: t.publicBooking.holdFiveMinutesLeft },
  { seconds: 60, message: t.publicBooking.holdOneMinuteLeft },
] as const;

/** `mm:ss`, as the countdown shows it. */
export function formatRemaining(totalSeconds: number): string {
  const seconds = Math.max(0, totalSeconds);
  const mm = String(Math.floor(seconds / 60)).padStart(2, '0');
  const ss = String(seconds % 60).padStart(2, '0');
  return `${mm}:${ss}`;
}

/** Whole seconds until `expiresAt`, never below zero and never above the hold window. */
export function secondsUntil(expiresAt: string): number {
  const seconds = Math.ceil((Date.parse(expiresAt) - Date.now()) / 1000);
  return Math.min(HOLD_WINDOW_SECONDS, Math.max(0, seconds));
}

/**
 * The server's `expires_at` of a hold just taken, as an instant of this
 * client's clock. The server counts on its own clock; a phone whose clock runs
 * behind reads that instant as further away than the hold window (a count that
 * starts at 10:01 or more). A hold just taken has at most the window left, so
 * the deadline is never later than the window from now. A clock that runs
 * ahead only shortens the count: the client never outlives the server's hold.
 */
export function clientHoldDeadline(expiresAt: string, now: number = Date.now()): string {
  return new Date(Math.min(Date.parse(expiresAt), now + HOLD_WINDOW_SECONDS * 1000)).toISOString();
}

/** The message of the smallest threshold already reached, or `null` above all of them. */
export function thresholdMessage(remaining: number): string | null {
  let message: string | null = null;
  for (const threshold of THRESHOLDS) {
    if (remaining <= threshold.seconds) message = threshold.message;
  }
  return message;
}
