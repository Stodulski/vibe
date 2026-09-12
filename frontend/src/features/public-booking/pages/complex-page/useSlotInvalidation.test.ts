import { renderHook } from '@testing-library/react';
import { toast } from 'sonner';
import { useSlotInvalidation, isSlotStillAvailable, isStartTimeStillOffered } from './useSlotInvalidation';
import type { AvailabilityData } from '@/shared/types/api.types';
import type { SelectedSlot } from '@/features/public-booking';

vi.mock('sonner', () => ({ toast: { error: vi.fn() } }));

const selectedSlot: SelectedSlot = {
  courtId: 'ct1',
  courtName: 'Cancha 1',
  sport: 'padel',
  courtType: 'indoor',
  slot: {
    start_time: '10:00',
    end_time: '11:00',
    start_min: 600,
    duration_minutes: 60,
    price: 500_000,
    available: true,
  },
  durationMinutes: 60,
  totalPrice: 500_000,
  endTime: '11:00',
};

function availabilityWith(available: boolean): AvailabilityData {
  return {
    date: '2026-03-20',
    day: 'friday',
    is_open: true,
    courts: [
      {
        court_id: 'ct1',
        court_name: 'Cancha 1',
        sport: 'padel',
        court_type: 'indoor',
        slots: [
          {
            start_time: '10:00',
            end_time: '11:00',
            start_min: 600,
            duration_minutes: 60,
            price: 500_000,
            available,
          },
        ],
      },
    ],
  };
}

beforeEach(() => {
  vi.clearAllMocks();
});

describe('isSlotStillAvailable', () => {
  it('is true when nothing is selected', () => {
    expect(isSlotStillAvailable(null, availabilityWith(true))).toBe(true);
  });

  it('is true when there is nothing to check against yet', () => {
    expect(isSlotStillAvailable(selectedSlot, undefined)).toBe(true);
  });

  it('is true when the court and hour are still available', () => {
    expect(isSlotStillAvailable(selectedSlot, availabilityWith(true))).toBe(true);
  });

  it('is false when the hour is no longer available', () => {
    expect(isSlotStillAvailable(selectedSlot, availabilityWith(false))).toBe(false);
  });

  it('is false when the court disappeared entirely', () => {
    const availability: AvailabilityData = { ...availabilityWith(true), courts: [] };
    expect(isSlotStillAvailable(selectedSlot, availability)).toBe(false);
  });
});

describe('isStartTimeStillOffered', () => {
  const courts = availabilityWith(true).courts;

  it('is true for null (no open question to invalidate)', () => {
    expect(isStartTimeStillOffered(null, courts)).toBe(true);
  });

  it('is true when some court still has that hour available', () => {
    expect(isStartTimeStillOffered('10:00', courts)).toBe(true);
  });

  it('is false once no court offers that hour any more', () => {
    expect(isStartTimeStillOffered('10:00', availabilityWith(false).courts)).toBe(false);
    expect(isStartTimeStillOffered('11:00', courts)).toBe(false);
  });
});

describe('useSlotInvalidation', () => {
  it('returns null and does nothing else when there is no selected slot', () => {
    const { result } = renderHook(() => useSlotInvalidation(null, availabilityWith(true)));
    expect(result.current).toBeNull();
    expect(toast.error).not.toHaveBeenCalled();
  });

  it('returns the slot as-is when availability has not loaded yet', () => {
    const { result } = renderHook(() => useSlotInvalidation(selectedSlot, undefined));
    expect(result.current).toBe(selectedSlot);
    expect(toast.error).not.toHaveBeenCalled();
  });

  it('returns the slot and does not toast while it stays available', () => {
    const { result } = renderHook(() => useSlotInvalidation(selectedSlot, availabilityWith(true)));
    expect(result.current).toBe(selectedSlot);
    expect(toast.error).not.toHaveBeenCalled();
  });

  it('returns null and toasts once a refetch shows the slot is no longer available', async () => {
    const { result, rerender } = renderHook(
      ({ availability }: { availability: AvailabilityData }) => useSlotInvalidation(selectedSlot, availability),
      { initialProps: { availability: availabilityWith(true) } },
    );
    expect(result.current).toBe(selectedSlot);

    rerender({ availability: availabilityWith(false) });

    expect(result.current).toBeNull();
    await vi.waitFor(() => {
      expect(toast.error).toHaveBeenCalledTimes(1);
    });
  });
});
