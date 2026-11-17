import { describe, expect, it } from 'vitest';
import type { CourtAvailability } from '../../api/types';
import { takenSlotOptions } from './takenSlots';

function court(id: string, name: string, slots: [string, number, boolean][]): CourtAvailability {
  return {
    court_id: id,
    court_name: name,
    sport: 'padel',
    court_type: 'outdoor',
    has_lighting: true,
    slots: slots.map(([start, startMin, available]) => ({
      start_time: start,
      end_time: '',
      start_min: startMin,
      duration_minutes: 90,
      price: 1000,
      available,
    })),
  };
}

describe('takenSlotOptions', () => {
  const now = new Date('2099-12-05T12:00:00-03:00');

  it('lists the taken slots still ahead, soonest first, one per court and time', () => {
    const options = takenSlotOptions(
      [
        court('c2', 'Pádel 2', [['19:00', 1140, false]]),
        court('c1', 'Pádel 1', [
          ['10:00', 600, false],
          ['18:00', 1080, false],
          ['19:00', 1140, true],
        ]),
      ],
      '2099-12-05',
      now,
    );

    expect(options.map((option) => option.label)).toEqual(['18:00 · Pádel 1', '19:00 · Pádel 2']);
    expect(options[0]?.target).toEqual({
      kind: 'slot',
      court_id: 'c1',
      date: '2099-12-05',
      start_time: '18:00',
      duration_minutes: 90,
    });
  });

  it('keeps an overnight slot past midnight as ahead', () => {
    const options = takenSlotOptions([court('c1', 'Pádel 1', [['00:30', 1470, false]])], '2099-12-05', now);

    expect(options.map((option) => option.label)).toEqual(['00:30 · Pádel 1']);
  });

  it('offers nothing when every slot is free', () => {
    expect(takenSlotOptions([court('c1', 'Pádel 1', [['19:00', 1140, true]])], '2099-12-05', now)).toEqual([]);
  });
});
