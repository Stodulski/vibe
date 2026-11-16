import { BOOKING_MESSAGES } from '../messages';
import { refusalCopy, type RefusalCopy } from './signUpRefusals';

const t = BOOKING_MESSAGES.tournaments;

const TOURNAMENT_REFUSALS: RefusalCopy = {
  full: t.categoryFullRefusal,
  closed: t.signUpClosed,
  alreadyRegistered: t.alreadyRegistered,
  conflict: t.signUpConflict,
  tooManyUnpaid: t.tooManyUnpaid,
  fallback: t.signUpError,
};

/** The text a refused pair sign-up shows. */
export function tournamentRefusalCopy(error: unknown): string {
  return refusalCopy(error, TOURNAMENT_REFUSALS);
}
