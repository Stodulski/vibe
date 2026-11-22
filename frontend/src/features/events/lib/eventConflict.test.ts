import { keepTogether } from '@/shared/lib/keepTogether';
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
    ['a booking already holds those hours on Cancha 2', t.booking(keepTogether('Cancha 2'))],
    ['those hours are blocked on Cancha 2', t.blocked(keepTogether('Cancha 2'))],
    ['a fixed weekly booking (turno fijo) holds those hours on Cancha 2', t.fixed(keepTogether('Cancha 2'))],
    ['another event already holds those hours on Cancha 2', t.event(keepTogether('Cancha 2'))],
  ])('says "%s" in Spanish and names the court', async (detail, expected) => {
    expect(eventConflictMessage(await conflict(detail))).toBe(expected);
  });

  it('keeps a court name that has spaces and accents intact', async () => {
    const message = eventConflictMessage(await conflict('another event already holds those hours on Cancha Central 1'));
    expect(message).toBe(t.event(keepTogether('Cancha Central 1')));
  });

  it('shows the server sentence when a visitor is booking the hours online, with when they free up', async () => {
    const detail = 'Este horario se está reservando online. Si no se paga, se libera a las 18:47.';
    expect(eventConflictMessage(await conflict(detail))).toBe(detail);
  });

  it('answers a general line for a 409 reason it does not know, never the server English', async () => {
    expect(eventConflictMessage(await conflict('something new'))).toBe(t.unknown);
  });

  it('is null for an error that is not a 409', async () => {
    expect(eventConflictMessage(await makeConsumedHttpError(422, { title: 'Invalid', detail: 'x' }))).toBeNull();
    expect(eventConflictMessage(new Error('boom'))).toBeNull();
  });
});
