import { BOOKING_MESSAGES } from '../messages';
import { refusalCopy, type RefusalCopy } from './signUpRefusals';

const t = BOOKING_MESSAGES.events;

const EVENT_REFUSALS: RefusalCopy = {
  full: t.eventFullRefusal,
  closed: t.signUpClosed,
  alreadyRegistered: t.alreadyRegistered,
  conflict: t.signUpConflict,
  tooManyUnpaid: t.tooManyUnpaid,
  fallback: t.signUpError,
};

/** The text a refused event sign-up shows. */
export function signUpRefusalCopy(error: unknown): string {
  return refusalCopy(error, EVENT_REFUSALS);
}
