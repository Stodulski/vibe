import { describe, expect, it } from 'vitest';
import type { CourtAvailability } from '../../api/types';
import { buildRulerHours, nearestFree } from './rulerHours';

function court(id: string, taken: string[]): CourtAvailability {
  return {
    court_id: id,
    court_name: `Cancha ${id}`,
    sport: 'padel',
    court_type: 'outdoor',
    has_lighting: false,
    slots: ['18:00', '19:00', '20:00'].map((time) => ({
      start_time: time,
      end_time: '',
      start_min: Number(time.slice(0, 2)) * 60,
      duration_minutes: 60,
      price: 1000,
      available: !taken.includes(time),
    })),
  };
}

describe('buildRulerHours', () => {
  it('lists every hour once, free when any court is free and taken when none is', () => {
    const hours = buildRulerHours([court('1', ['18:00', '19:00']), court('2', ['18:00'])]);

    expect(hours.map((hour) => [hour.startTime, hour.free !== null])).toEqual([
      ['18:00', false],
      ['19:00', true],
      ['20:00', true],
    ]);
    expect(hours[1]?.free?.courts.map((entry) => entry.court.court_id)).toEqual(['2']);
  });

  it('leaves out a taken hour that already began on the day shown', () => {
    const now = new Date('2026-11-18T18:30:00-03:00');
    const hours = buildRulerHours([court('1', ['18:00', '20:00'])], '2026-11-18', now);

    expect(hours.map((hour) => hour.startTime)).toEqual(['19:00', '20:00']);
  });
});

describe('nearestFree', () => {
  it('finds the closest free hour, the later one on a tie, and null when none is free', () => {
    const hours = buildRulerHours([court('1', ['19:00'])]);

    expect(nearestFree(hours, 1)).toBe(2);
    expect(nearestFree(buildRulerHours([court('1', ['18:00', '19:00', '20:00'])]), 1)).toBeNull();
  });
});
