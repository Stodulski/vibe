import { describe, it, expect } from 'vitest';
import { makeConsumedHttpError } from '@/test/factories';
import { ES_AR } from '@/shared/i18n/es_AR';
import { eventConflictMessage } from './eventConflict';

const t = ES_AR.events.conflict;

async function conflict(detail: string) {
  return makeConsumedHttpError(409, { title: 'Conflict', detail });
}

describe('eventConflictMessage', () => {
  it.each([
    ['a booking already holds those hours on Cancha 2', t.booking('Cancha 2')],
    ['those hours are blocked on Cancha 2', t.blocked('Cancha 2')],
    ['a fixed weekly booking (turno fijo) holds those hours on Cancha 2', t.fixed('Cancha 2')],
    ['another event already holds those hours on Cancha 2', t.event('Cancha 2')],
  ])('says "%s" in Spanish and names the court', async (detail, expected) => {
    expect(eventConflictMessage(await conflict(detail))).toBe(expected);
  });

  it('keeps a court name that has spaces and accents intact', async () => {
    const message = eventConflictMessage(await conflict('another event already holds those hours on Cancha Central 1'));
    expect(message).toBe(t.event('Cancha Central 1'));
  });

  it('answers a general line for a 409 reason it does not know, never the server English', async () => {
    expect(eventConflictMessage(await conflict('something new'))).toBe(t.unknown);
  });

  it('is null for an error that is not a 409', async () => {
    expect(eventConflictMessage(await makeConsumedHttpError(422, { title: 'Invalid', detail: 'x' }))).toBeNull();
    expect(eventConflictMessage(new Error('boom'))).toBeNull();
  });
});
