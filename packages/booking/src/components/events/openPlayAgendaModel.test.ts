import { describe, expect, it } from 'vitest';
import { EVENT, MEN_6_TO_8 } from '../../test/eventFixtures';
import { openPlayRows } from './openPlayAgendaModel';

describe('openPlayRows', () => {
  it('lists only the events open for sign-up with a place left', () => {
    const rows = openPlayRows([
      EVENT,
      { ...EVENT, id: 'full', spots_left: 0 },
      { ...EVENT, id: 'cancelled', status: 'cancelled' },
      { ...EVENT, id: 'started', date: '2000-01-01' },
    ]);

    expect(rows.map((row) => row.event.id)).toEqual(['ev1']);
  });

  it('sorts by day, then by start time', () => {
    const rows = openPlayRows([
      { ...EVENT, id: 'late', date: '2099-12-06', start_time: '18:00', end_time: '20:00' },
      EVENT,
      { ...EVENT, id: 'early', start_time: '10:00', end_time: '12:00' },
    ]);

    expect(rows.map((row) => row.event.id)).toEqual(['early', 'ev1', 'late']);
  });

  it('labels each row with its sport and titles it by its modality and range', () => {
    const rows = openPlayRows([{ ...MEN_6_TO_8, id: 'ev-men' }, EVENT]);
    const men = rows.find((row) => row.event.id === 'ev-men');
    const anyone = rows.find((row) => row.event.id === 'ev1');

    expect(men?.label).toBe('Cancha abierta de pádel');
    expect(men?.title).toBe('Caballeros · 6ta a 8va');
    expect(anyone?.title).toBe('Abierta a todos');
  });
});
