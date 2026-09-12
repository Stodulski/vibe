import { useRef } from 'react';

/** A fresh `Idempotency-Key`. A UUID is 36 chars, well inside the backend's 64-char limit. */
export function newIdempotencyKey(): string {
  return crypto.randomUUID();
}

export interface AttemptKey {
  /**
   * Starts a new attempt. Call from `onMutate`, which React Query runs once
   * per `mutate()` and never again for that call's retries.
   */
  begin: () => string;
  /**
   * The key of the attempt in flight. Call from `mutationFn`, which React
   * Query *does* re-run per retry — so every retry of one submit sends the
   * key `begin` minted for it, and the server replays its first answer
   * instead of creating a second booking or charging twice.
   */
  current: () => string;
}

/**
 * One `Idempotency-Key` per submit attempt.
 *
 * The backend deduplicates `POST /book`, `POST /complexes/:id/bookings`,
 * `confirm-payment` and `manual-refund` on this header: the same key with the
 * same body replays the stored response (`Idempotent-Replay: true`), the same
 * key with a different body is a 409, and a request still in flight under
 * that key is a 409 too. So the key has to be stable across the retries of
 * one attempt and different for a genuinely new one — a key minted inside
 * `mutationFn` would be neither, since that function re-runs per retry.
 */
export function useIdempotencyKey(): AttemptKey {
  const key = useRef<string>(newIdempotencyKey());

  return {
    begin: () => {
      key.current = newIdempotencyKey();
      return key.current;
    },
    current: () => key.current,
  };
}
