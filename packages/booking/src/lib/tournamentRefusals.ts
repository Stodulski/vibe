import { BOOKING_MESSAGES } from '../messages';
import { getProblem } from './ApiError';
import { declarationRefusalCopy, type PairCategory } from './declaredPlayers';
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

/**
 * The text a refused pair sign-up shows. A declaration the server found not to
 * fit is named in the words of its category's rule, the same ones the form
 * checks before it sends.
 */
export function tournamentRefusalCopy(error: unknown, category?: PairCategory): string {
  const declared = category ? declarationRefusalCopy(getProblem(error), { kind: 'pair', category }) : undefined;
  return declared ?? refusalCopy(error, TOURNAMENT_REFUSALS);
}
