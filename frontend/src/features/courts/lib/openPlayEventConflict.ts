import { getHttpErrorMessage, getHttpStatus } from '@/shared/lib/utils';

/**
 * Whether a 409 is the server saying the slot belongs to an open-play event.
 * The API names it only in English prose ("…belongs to an open-play event
 * (canchas libres)") with no code, so the one stable fragment, the Spanish
 * term in parentheses, is what identifies it.
 */
export function isOpenPlayEventConflict(error: unknown): boolean {
  return getHttpStatus(error) === 409 && getHttpErrorMessage(error, '').includes('(canchas libres)');
}
