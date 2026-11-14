import { ES_AR } from '@/shared/i18n/es_AR';
import { getHttpErrorMessage, getHttpStatus } from '@/shared/lib/utils';

const t = ES_AR.tournaments;
const c = t.schedule.conflict;

// The API words a 409 in English prose with no code and names the court at the
// end, so each reason is told apart by its stable lead (the same reading
// `eventConflictMessage` does for events).
const REASONS: { match: RegExp; say: (court: string) => string }[] = [
  { match: /^a fixed weekly booking \(turno fijo\) holds those hours on (.+)$/, say: c.fixed },
  { match: /^a booking already holds those hours on (.+)$/, say: c.booking },
  { match: /^those hours are blocked on (.+)$/, say: c.blocked },
  { match: /^another tournament match already holds those hours on (.+)$/, say: c.match },
  { match: /^(?:an|another) event already holds those hours on (.+)$/, say: c.event },
];

/**
 * What a 409 on scheduling a match says, in Spanish, or `null` when the error is
 * not one. A reason this build does not know falls back to a general line
 * rather than showing the server's English.
 */
export function scheduleConflictMessage(error: unknown): string | null {
  if (getHttpStatus(error) !== 409) return null;
  const detail = getHttpErrorMessage(error, '').trim();
  for (const { match, say } of REASONS) {
    const found = match.exec(detail);
    if (found?.[1]) return say(found[1]);
  }
  return c.unknown;
}

/** Any scheduling failure in Spanish: a conflict names the court, a 422 is a day outside the tournament. */
export function scheduleErrorMessage(error: unknown): string {
  const conflict = scheduleConflictMessage(error);
  if (conflict) return conflict;
  return getHttpStatus(error) === 422 ? t.schedule.outsideDays : t.schedule.error;
}

export function resultErrorMessage(error: unknown): string {
  const e = t.result.apiErrors;
  switch (getHttpStatus(error)) {
    case 409:
      return e.locked;
    case 422:
      return e.invalid;
    case 400:
      return e.notReady;
    default:
      return e.unknown;
  }
}

export function drawErrorMessage(error: unknown): string {
  const e = t.draw.errors;
  switch (getHttpStatus(error)) {
    case 409:
      return e.locked;
    case 400:
      return e.notEnough;
    case 422:
      return e.settings;
    default:
      return e.unknown;
  }
}
