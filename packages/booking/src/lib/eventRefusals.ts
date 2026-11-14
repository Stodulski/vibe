import { HTTPError } from 'ky';
import { BOOKING_MESSAGES } from '../messages';
import { getProblem } from './ApiError';
import { getHttpErrorMessage } from './httpErrors';

const t = BOOKING_MESSAGES;

/**
 * The text a refused sign-up shows. The 409s each carry their own problem kind
 * (`event-full`, `event-closed`, `already-registered`); the English detail is
 * never shown. Any other 409 keeps the catch-all sentence.
 */
function conflictCopy(kind: string | undefined): string {
  switch (kind) {
    case 'event-full':
      return t.events.eventFullRefusal;
    case 'event-closed':
      return t.events.signUpClosed;
    case 'already-registered':
      return t.events.alreadyRegistered;
    default:
      return t.events.signUpConflict;
  }
}

/** The text a refused sign-up shows, by status and, for a 409, by problem kind. */
export function signUpRefusalCopy(error: unknown): string {
  if (error instanceof HTTPError) {
    switch (error.response.status) {
      case 409:
        return conflictCopy(getProblem(error)?.kind);
      case 403:
        return t.publicBooking.accountBlocked;
      case 429:
        return t.events.tooManyUnpaid;
      case 503:
        return t.publicBooking.paymentLinkError;
      default:
        break;
    }
  }
  return getHttpErrorMessage(error, t.events.signUpError);
}
