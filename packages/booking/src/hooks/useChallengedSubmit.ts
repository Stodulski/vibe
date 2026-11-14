import { useCallback, useEffect, useRef } from 'react';
import { toast } from 'sonner';
import { BOOKING_MESSAGES } from '../messages';
import type { HoldChallenge } from '../pages/complex-page/useHoldChallenge';

/**
 * Sends a form with the Turnstile token of its invisible widget, the way the
 * complex page takes a hold: a submit before the challenge has a token is
 * parked, the visitor is told the check is finishing, and it goes out by itself
 * as soon as the token arrives. With no site key the form is sent at once,
 * without a token. The caller resets the challenge when the request settles: a
 * token is single use.
 */
export function useChallengedSubmit<T>(
  challenge: HoldChallenge,
  send: (data: T, token: string | undefined) => void,
): (data: T) => void {
  const parked = useRef<{ data: T } | null>(null);
  const latest = useRef(send);
  useEffect(() => {
    latest.current = send;
  });
  const { token } = challenge;
  useEffect(() => {
    const waiting = parked.current;
    if (!token || !waiting) return;
    parked.current = null;
    latest.current(waiting.data, token);
  }, [token]);
  return useCallback(
    (data: T) => {
      if (challenge.required && !token) {
        parked.current = { data };
        toast.info(BOOKING_MESSAGES.publicBooking.holdChallengePending);
        return;
      }
      latest.current(data, token);
    },
    [challenge.required, token],
  );
}
