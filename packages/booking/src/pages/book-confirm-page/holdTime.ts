import { BOOKING_MESSAGES } from '../../messages';

const t = BOOKING_MESSAGES;

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

/** Whole seconds until `expiresAt`, never below zero. */
export function secondsUntil(expiresAt: string): number {
  return Math.max(0, Math.ceil((Date.parse(expiresAt) - Date.now()) / 1000));
}

/** The message of the smallest threshold already reached, or `null` above all of them. */
export function thresholdMessage(remaining: number): string | null {
  let message: string | null = null;
  for (const threshold of THRESHOLDS) {
    if (remaining <= threshold.seconds) message = threshold.message;
  }
  return message;
}
