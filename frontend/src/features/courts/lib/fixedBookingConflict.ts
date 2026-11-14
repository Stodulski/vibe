import { getHttpErrorMessage, getHttpStatus } from '@/shared/lib/utils';

/**
 * Whether a 409 is the server saying the slot belongs to a fixed weekly
 * booking. The API names it only in English prose ("…belongs to a fixed weekly
 * booking (turno fijo)") with no code, so the one stable fragment, the
 * Spanish term in parentheses, is what identifies it.
 */
export function isFixedBookingConflict(error: unknown): boolean {
  return getHttpStatus(error) === 409 && getHttpErrorMessage(error, '').includes('(turno fijo)');
}
