import { ES_AR } from '@/shared/i18n/es_AR';
import { getHttpErrorMessage, getHttpStatus } from '@/shared/lib/utils';

const t = ES_AR.events.conflict;

// The API words the 409 in English prose with no code and names the court at
// the end, so each reason is told apart by its stable lead and the court is
// what follows "hold(s) those hours on". Same reading as
// `isFixedBookingConflict`, which does it for a blocked slot.
const REASONS: { match: RegExp; say: (court: string) => string }[] = [
  { match: /^a fixed weekly booking \(turno fijo\) holds those hours on (.+)$/, say: t.fixed },
  { match: /^a booking already holds those hours on (.+)$/, say: t.booking },
  { match: /^those hours are blocked on (.+)$/, say: t.blocked },
  { match: /^another event already holds those hours on (.+)$/, say: t.event },
];

/**
 * What a 409 on creating an event says, in Spanish, or `null` when the error
 * is not one. A reason this build does not know falls back to a general line
 * rather than showing the server's English.
 */
export function eventConflictMessage(error: unknown): string | null {
  if (getHttpStatus(error) !== 409) return null;
  const detail = getHttpErrorMessage(error, '').trim();
  for (const { match, say } of REASONS) {
    const found = match.exec(detail);
    if (found?.[1]) return say(found[1]);
  }
  return t.unknown;
}
