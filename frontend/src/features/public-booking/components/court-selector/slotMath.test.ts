import { computeTotalPrice, getEndTime, getTimeGroup } from './slotMath';
import type { CourtAvailability } from '@/shared/types/api.types';

const court: CourtAvailability = {
  court_id: 'court-1',
  court_name: 'Cancha 1',
  sport: 'padel',
  court_type: 'indoor',
  slots: [
    {
      start_time: '08:00',
      end_time: '09:30',
      start_min: 480,
      duration_minutes: 90,
      price: 500000,
      available: true,
    },
    {
      start_time: '09:30',
      end_time: '11:00',
      start_min: 570,
      duration_minutes: 90,
      price: 500000,
      available: true,
    },
    {
      start_time: '11:00',
      end_time: '12:30',
      start_min: 660,
      duration_minutes: 90,
      price: 600000,
      available: false,
    },
  ],
};

const [firstSlot, , thirdSlot] = court.slots;
if (!firstSlot || !thirdSlot) throw new Error('court fixture must have at least 3 slots');

describe('computeTotalPrice', () => {
  it('returns the slot price as-is (already priced by the server for the requested duration)', () => {
    expect(computeTotalPrice(firstSlot)).toBe(500000);
    expect(computeTotalPrice(thirdSlot)).toBe(600000);
  });
});

describe('getEndTime', () => {
  it('returns the slot end_time as-is (already computed by the server for the requested duration)', () => {
    expect(getEndTime(firstSlot)).toBe('09:30');
    expect(getEndTime(thirdSlot)).toBe('12:30');
  });
});

describe('getTimeGroup', () => {
  it('classifies before-noon as morning', () => {
    expect(getTimeGroup(8 * 60)).toBe('morning');
  });

  it('classifies noon-6pm as afternoon', () => {
    expect(getTimeGroup(14 * 60)).toBe('afternoon');
  });

  it('classifies after-6pm as evening', () => {
    expect(getTimeGroup(20 * 60)).toBe('evening');
  });

  it('keeps a past-midnight hour in the night it closes', () => {
    // Minute 1470 is 00:30 on the clock and hour 24 in its window. Reading
    // the clock face filed it under morning, which put the end of the night
    // at the top of the page.
    expect(getTimeGroup(1470)).toBe('evening');
  });
});

// `groupSlotsByTime` bucketed one court's slots and went with the
// court-major grid. Its replacement buckets hours across every court —
// covered by timeOptions.test.ts.
