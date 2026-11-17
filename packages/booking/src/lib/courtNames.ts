import { BOOKING_MESSAGES } from '../messages';

/** Beyond this many courts a ticket or a detail counts them instead of naming each one. */
export const MAX_NAMED_COURTS = 2;

/**
 * The courts of an entry in words: one or two are named, more are counted
 * ("6 canchas"). Callers show nothing for an entry without courts.
 */
export function courtNamesLine(names: readonly string[]): string {
  return names.length <= MAX_NAMED_COURTS ? names.join(', ') : BOOKING_MESSAGES.events.courtsCount(names.length);
}
