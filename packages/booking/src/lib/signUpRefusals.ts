import { HTTPError } from 'ky';
import { BOOKING_MESSAGES } from '../messages';
import { getProblem } from './ApiError';
import { getHttpErrorMessage } from './httpErrors';
import { turnstileCopy } from './bookingRefusals';

/** The Spanish sentence each way a sign-up can be refused answers with. */
export interface RefusalCopy {
  /** `409` `event-full`. */
  full: string;
  /** `409` `event-closed`. */
  closed: string;
  /** `409` `already-registered`. */
  alreadyRegistered: string;
  /** Any other `409`. */
  conflict: string;
  /** `429` `rate-limited`: too many unpaid holds. */
  tooManyUnpaid: string;
  /** Anything else. */
  fallback: string;
}

function conflictCopy(kind: string | undefined, copy: RefusalCopy): string {
  switch (kind) {
    case 'event-full':
      return copy.full;
    case 'event-closed':
      return copy.closed;
    case 'already-registered':
      return copy.alreadyRegistered;
    default:
      return copy.conflict;
  }
}

/**
 * The text a refused sign-up shows, by status and, for a 409, by problem kind.
 * The English detail of the server is never shown.
 */
export function refusalCopy(error: unknown, copy: RefusalCopy): string {
  if (error instanceof HTTPError) {
    switch (error.response.status) {
      case 409:
        return conflictCopy(getProblem(error)?.kind, copy);
      case 403:
        return BOOKING_MESSAGES.publicBooking.accountBlocked;
      case 422:
        // The bot check, worded as the booking hold words it.
        return turnstileCopy(error) ?? copy.fallback;
      case 429:
        return copy.tooManyUnpaid;
      case 503:
        return BOOKING_MESSAGES.publicBooking.paymentLinkError;
      default:
        break;
    }
  }
  return getHttpErrorMessage(error, copy.fallback);
}
