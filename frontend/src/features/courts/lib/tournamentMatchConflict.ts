import { getHttpErrorMessage, getHttpStatus } from '@/shared/lib/utils';

/**
 * Whether a 409 is the server saying the slot belongs to a scheduled
 * tournament match. The API names it only in English prose ("…belongs to a
 * tournament match") with no code, so that fragment is what identifies it.
 */
export function isTournamentMatchConflict(error: unknown): boolean {
  return getHttpStatus(error) === 409 && getHttpErrorMessage(error, '').includes('belongs to a tournament match');
}
