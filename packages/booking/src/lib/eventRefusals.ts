import { HTTPError } from 'ky';
import { BOOKING_MESSAGES } from '../messages';
import { getHttpErrorMessage } from './httpErrors';

const t = BOOKING_MESSAGES;

/** The text a refused sign-up shows. The 409 prose is English and says three different things, so one sentence covers them. */
export function signUpRefusalCopy(error: unknown): string {
  if (error instanceof HTTPError) {
    switch (error.response.status) {
      case 409:
        return t.events.signUpConflict;
      case 403:
        return t.publicBooking.accountBlocked;
      case 429:
        return t.publicBooking.holdRateLimited;
      case 503:
        return t.publicBooking.paymentLinkError;
      default:
        break;
    }
  }
  return getHttpErrorMessage(error, t.events.signUpError);
}
