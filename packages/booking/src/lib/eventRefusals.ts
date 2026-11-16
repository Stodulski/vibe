import type { PublicOpenPlayEvent } from '../api/types';
import { BOOKING_MESSAGES } from '../messages';
import { getProblem } from './ApiError';
import { declarationRefusalCopy } from './declaredPlayers';
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

/**
 * The text a refused event sign-up shows. A gender or level the server found
 * not to fit is named in the words of the event's own rule.
 */
export function signUpRefusalCopy(
  error: unknown,
  event?: Pick<PublicOpenPlayEvent, 'modality' | 'level_min' | 'level_max'>,
): string {
  const declared = event ? declarationRefusalCopy(getProblem(error), { kind: 'event', event }) : undefined;
  return declared ?? refusalCopy(error, EVENT_REFUSALS);
}
