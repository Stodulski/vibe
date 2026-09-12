import { describe, it, expect } from 'vitest';
import { buildConfirmState } from './buildConfirmState';
import type { PublicComplex } from '@/shared/types/api.types';
import type { SelectedSlot } from '@/features/public-booking';

const complex: PublicComplex = {
  id: 'c1',
  name: 'Club Norte',
  slug: 'club-norte',
  amenities: [],
  address: 'Av. Libertador 1234',
  city: 'Buenos Aires',
  province: 'CABA',
  country_code: 'AR',
  currency: 'ARS',
  phone: '+5491155550000',
  deposit_percentage: 30,
  cancellation_hours: 24,
  payments_enabled: true,
};

const selectedSlot: SelectedSlot = {
  courtId: 'ct1',
  courtName: 'Cancha 1',
  sport: 'padel',
  courtType: 'indoor',
  slot: {
    start_time: '10:00',
    end_time: '12:00',
    start_min: 600,
    duration_minutes: 120,
    price: 1_000_000,
    available: true,
  },
  durationMinutes: 120,
  totalPrice: 1_000_000,
  endTime: '12:00',
};

describe('buildConfirmState', () => {
  it('maps complex + selectedSlot fields into BookingSlotInfo', () => {
    const state = buildConfirmState(complex, selectedSlot, '2026-03-20');
    expect(state).toEqual({
      complexId: 'c1',
      complexName: 'Club Norte',
      complexPhone: '+5491155550000',
      courtId: 'ct1',
      courtName: 'Cancha 1',
      sport: 'padel',
      courtType: 'indoor',
      courtDescription: undefined,
      date: '2026-03-20',
      startTime: '10:00',
      endTime: '12:00',
      durationMinutes: 120,
      price: 1_000_000,
      depositPercentage: 30,
      cancellationHours: 24,
    });
  });

  it('carries the selected slot duration through verbatim (no multiplication)', () => {
    const state = buildConfirmState(complex, { ...selectedSlot, durationMinutes: 90 }, '2026-03-20');
    expect(state.durationMinutes).toBe(90);
  });
});
