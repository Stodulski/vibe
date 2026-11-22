import { keepTogether } from '@/shared/lib/keepTogether';
import { describe, it, expect } from 'vitest';
import { makeConsumedHttpError } from '@/test/factories';
import { ES_AR } from '@/shared/i18n/es_AR';
import {
  courtFeeErrorMessage,
  drawErrorMessage,
  resultErrorMessage,
  scheduleConflictMessage,
  scheduleErrorMessage,
} from './tournamentErrors';

const t = ES_AR.tournaments;

function http(status: number, detail: string) {
  return makeConsumedHttpError(status, { title: 'x', detail });
}

describe('scheduleConflictMessage', () => {
  it.each([
    ['a booking already holds those hours on Cancha 2', t.schedule.conflict.booking(keepTogether('Cancha 2'))],
    ['those hours are blocked on Cancha 2', t.schedule.conflict.blocked(keepTogether('Cancha 2'))],
    [
      'a fixed weekly booking (turno fijo) holds those hours on Cancha 2',
      t.schedule.conflict.fixed(keepTogether('Cancha 2')),
    ],
    [
      'another tournament match already holds those hours on Cancha 2',
      t.schedule.conflict.match(keepTogether('Cancha 2')),
    ],
    [
      'an event already holds those hours on Cancha Central 1',
      t.schedule.conflict.event(keepTogether('Cancha Central 1')),
    ],
  ])('says "%s" in Spanish and names the court', async (detail, expected) => {
    expect(scheduleConflictMessage(await http(409, detail))).toBe(expected);
  });

  it('shows the server sentence when a visitor is booking the hours online, with when they free up', async () => {
    const detail = 'Este horario se está reservando online. Si no se paga, se libera a las 18:47.';
    expect(scheduleConflictMessage(await http(409, detail))).toBe(detail);
  });

  it('answers a general line for a reason it does not know, never the server English', async () => {
    expect(scheduleConflictMessage(await http(409, 'something new'))).toBe(t.schedule.conflict.unknown);
  });

  it('is null when the error is not a 409', async () => {
    expect(scheduleConflictMessage(await http(422, 'x'))).toBeNull();
    expect(scheduleConflictMessage(new Error('boom'))).toBeNull();
  });
});

describe('scheduleErrorMessage', () => {
  it('tells a day outside the tournament and falls back to a general line', async () => {
    expect(scheduleErrorMessage(await http(422, 'date outside'))).toBe(t.schedule.outsideDays);
    expect(scheduleErrorMessage(new Error('boom'))).toBe(t.schedule.error);
  });
});

describe('resultErrorMessage', () => {
  it('says why a result cannot be loaded', async () => {
    expect(resultErrorMessage(await http(409, 'next match has a result'))).toBe(t.result.apiErrors.locked);
    expect(resultErrorMessage(await http(422, 'invalid score'))).toBe(t.result.apiErrors.invalid);
    expect(resultErrorMessage(await http(400, 'match not ready'))).toBe(t.result.apiErrors.notReady);
    expect(resultErrorMessage(new Error('boom'))).toBe(t.result.apiErrors.unknown);
  });
});

describe('drawErrorMessage', () => {
  it('says why the draw cannot be made', async () => {
    expect(drawErrorMessage(await http(409, 'result loaded'))).toBe(t.draw.errors.locked);
    expect(drawErrorMessage(await http(400, 'fewer than two'))).toBe(t.draw.errors.notEnough);
    expect(drawErrorMessage(await http(422, 'groups'))).toBe(t.draw.errors.settings);
    expect(drawErrorMessage(new Error('boom'))).toBe(t.draw.errors.unknown);
  });

  it('says why the groups do not fit and what to change when it knows the settings', async () => {
    const message = drawErrorMessage(await http(422, 'groups'), { pairs: 4, groupSize: 3, qualifiers: 2 });
    expect(message).toBe(
      'Con 4 parejas no se pueden armar zonas de 3 donde pasen 2. Bajá cuántas pasan o el tamaño de la zona.',
    );
  });
});

describe('courtFeeErrorMessage', () => {
  const e = t.courtFee.errors;

  it('tells a closed cash apart from a pair that was already charged', async () => {
    expect(courtFeeErrorMessage(await http(409, 'no cash session is open'), 'charge')).toBe(
      ES_AR.validation.server.cashClosed,
    );
    expect(courtFeeErrorMessage(await http(409, 'no cash session is open'), 'undo')).toBe(
      ES_AR.validation.server.cashClosed,
    );
    expect(
      courtFeeErrorMessage(await http(409, 'the court fee was already charged for this pair and match'), 'charge'),
    ).toBe(e.alreadyCharged);
    expect(courtFeeErrorMessage(await http(409, 'the court fee is not charged for this pair and match'), 'undo')).toBe(
      e.notCharged,
    );
  });

  it('says a pair that owes nothing, and leaves any other failure to the general line', async () => {
    expect(courtFeeErrorMessage(await http(400, 'a bye'), 'charge')).toBe(e.notChargeable);
    expect(courtFeeErrorMessage(await http(500, 'boom'), 'charge')).toBeUndefined();
    expect(courtFeeErrorMessage(new Error('boom'), 'undo')).toBeUndefined();
  });
});
