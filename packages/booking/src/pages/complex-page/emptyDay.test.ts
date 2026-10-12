import { describe, expect, it } from 'vitest';
import type { CourtAvailability } from '../../api/types';
import { classifyEmptyDay, hasFreeSlot } from './emptyDay';

function court(available: boolean[]): CourtAvailability {
  return {
    court_id: 'ct1',
    court_name: 'Cancha 1',
    sport: 'padel',
    court_type: 'outdoor',
    slots: available.map((free, index) => ({
      start_time: `${String(10 + index)}:00`,
      end_time: `${String(11 + index)}:30`,
      start_min: 600 + index * 60,
      duration_minutes: 90,
      price: 1_000_000,
      available: free,
    })),
  };
}

describe('hasFreeSlot', () => {
  it('is false when no court has a slot', () => {
    expect(hasFreeSlot([])).toBe(false);
    expect(hasFreeSlot([court([])])).toBe(false);
  });

  it('is false when every slot is taken', () => {
    expect(hasFreeSlot([court([false, false]), court([false])])).toBe(false);
  });

  it('is true as soon as one slot of one court is free', () => {
    expect(hasFreeSlot([court([false]), court([false, true])])).toBe(true);
  });
});

describe('classifyEmptyDay', () => {
  it('has nothing to say when the grid already shows a free slot', () => {
    expect(classifyEmptyDay({ shownHasFreeSlot: true, unfilteredHasFreeSlot: true, otherDurations: false })).toBeNull();
  });

  it('calls the day filtered when free slots exist but the sport or duration filter hides them', () => {
    expect(classifyEmptyDay({ shownHasFreeSlot: false, unfilteredHasFreeSlot: true, otherDurations: 'checking' })).toBe(
      'filtered',
    );
  });

  it('waits for the other durations before it calls the day full', () => {
    expect(
      classifyEmptyDay({ shownHasFreeSlot: false, unfilteredHasFreeSlot: false, otherDurations: 'checking' }),
    ).toBe('checking');
  });

  it('calls the day filtered when another duration still has a free slot', () => {
    expect(classifyEmptyDay({ shownHasFreeSlot: false, unfilteredHasFreeSlot: false, otherDurations: true })).toBe(
      'filtered',
    );
  });

  it('calls the day full when no slot is free for any sport or duration', () => {
    expect(classifyEmptyDay({ shownHasFreeSlot: false, unfilteredHasFreeSlot: false, otherDurations: false })).toBe(
      'full',
    );
  });
});
