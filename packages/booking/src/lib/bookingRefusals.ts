import { HTTPError } from 'ky';
import { BOOKING_MESSAGES } from '../messages';
import { getProblem } from './ApiError';
import { getHttpErrorMessage } from './httpErrors';

const t = BOOKING_MESSAGES;

/**
 * The problem kind `POST /book` answers with (409) when its `hold_token` is
 * expired, unknown or already converted. The storefront reads all three as
 * "your time ran out".
 */
export const HOLD_EXPIRED_KIND = 'hold-expired';

/**
 * The text a 409 should show, chosen by the problem+json `kind` the backend
 * minted it with (`internal/httpx/problem.go`).
 *
 * `slot-unavailable` and `duplicate-booking` are both 409s and mean opposite
 * things to the person reading them: one says somebody else took the hours,
 * the other says they already booked them themselves. Switching on the status
 * alone told everyone the first story.
 */
export function conflictCopy(kind: string | undefined): string {
  switch (kind) {
    case 'slot-unavailable':
      return t.publicBooking.slotConflict;
    case 'duplicate-booking':
      return t.publicBooking.duplicateBooking;
    // `stale-version`, the generic `conflict`, and any kind a later backend
    // adds: nothing specific can be said, so ask for a retry.
    default:
      return t.publicBooking.bookingConflict;
  }
}

/**
 * The text for a `turnstile_token` validation error, by the code the backend
 * puts in the message (`required`, `invalid`, `unavailable`, as auth does), or
 * `undefined` when the 422 is about something else. An unknown code reads as
 * a failed check rather than leaking the raw code.
 */
export function turnstileCopy(error: HTTPError): string | undefined {
  const code = getProblem(error)?.errors.find((e) => e.field === 'turnstile_token')?.message;
  if (code === undefined) return undefined;
  switch (code) {
    case 'required':
      return t.publicBooking.turnstileRequired;
    case 'unavailable':
      return t.publicBooking.turnstileUnavailable;
    default:
      return t.publicBooking.turnstileInvalid;
  }
}

/**
 * The text for a refused `POST /book/holds`. It refuses the slot the way
 * `POST /book` does (409 by kind, 422 for the Turnstile token), plus a 429 when
 * this visitor holds or asks for too many slots at once.
 */
export function holdRefusalCopy(error: unknown): string {
  if (error instanceof HTTPError) {
    const status = error.response.status;
    if (status === 409) return conflictCopy(getProblem(error)?.kind);
    if (status === 429) return t.publicBooking.holdRateLimited;
    if (status === 422) {
      const copy = turnstileCopy(error);
      if (copy) return copy;
    }
    // A 404 court, a complex without payments, a 500: nothing the visitor
    // can act on beyond trying again.
    return t.publicBooking.holdCreateError;
  }
  return getHttpErrorMessage(error, t.publicBooking.holdCreateError);
}
