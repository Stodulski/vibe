import { useEffect, useRef, useState } from 'react';
import { BOOKING_MESSAGES } from '../../messages';
import { formatRemaining, secondsUntil, thresholdMessage } from './holdTime';

const t = BOOKING_MESSAGES;

interface HoldCountdownProps {
  /** The hold's `expires_at` from the server. The count is never a client-side 15:00. */
  expiresAt: string;
  /** Called once, when the time is up (at once when it already was). */
  onExpire: () => void;
}

/**
 * How long the slot stays held for this visitor, as `mm:ss`.
 *
 * The ticking text is a `timer`, which screen readers do not announce on their
 * own; a separate polite live region speaks only when a threshold is crossed
 * (five minutes, one minute), so nobody hears every second. The band the count
 * starts in is not announced. The expiry itself is told on the page the
 * visitor is sent back to.
 */
export function HoldCountdown({ expiresAt, onExpire }: HoldCountdownProps) {
  const [clock, setClock] = useState(() => ({ remaining: secondsUntil(expiresAt), announcement: '' }));
  const onExpireRef = useRef(onExpire);
  const expired = useRef(false);

  useEffect(() => {
    onExpireRef.current = onExpire;
  }, [onExpire]);

  useEffect(() => {
    let band = thresholdMessage(secondsUntil(expiresAt));
    const interval = setInterval(() => {
      const remaining = secondsUntil(expiresAt);
      const next = thresholdMessage(remaining);
      const crossed = next !== null && next !== band;
      band = next;
      setClock((previous) => ({ remaining, announcement: crossed ? next : previous.announcement }));
    }, 1000);
    return () => {
      clearInterval(interval);
    };
  }, [expiresAt]);

  useEffect(() => {
    if (clock.remaining > 0 || expired.current) return;
    expired.current = true;
    onExpireRef.current();
  }, [clock.remaining]);

  // Only the time, large and in the brand green. The sentence it stands for is
  // its accessible name, so a screen reader still says what the count is.
  return (
    <div>
      <p
        role="timer"
        aria-atomic="true"
        aria-label={t.publicBooking.holdCountdownLabel}
        className="font-display text-primary-400 text-center text-3xl font-bold tabular-nums"
      >
        {formatRemaining(clock.remaining)}
      </p>
      <span data-testid="hold-countdown-announcement" aria-live="polite" className="sr-only">
        {clock.announcement}
      </span>
    </div>
  );
}
