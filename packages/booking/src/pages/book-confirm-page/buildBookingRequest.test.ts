import { describe, it, expect } from 'vitest';
import { buildBookingRequest } from './buildBookingRequest';
import type { BookingSlotInfo } from '../../components/BookingForm';
import type { PublicBookingFormData } from '../../schemas/public-booking.schema';

const slotInfo: BookingSlotInfo = {
  complexId: 'c1',
  complexName: 'Club Norte',
  complexPhone: '1155550000',
  courtId: 'ct1',
  courtName: 'Cancha 1',
  date: '2026-03-20',
  startTime: '10:00',
  endTime: '11:30',
  durationMinutes: 90,
  price: 1_000_000,
  depositPercentage: 30,
  cancellationHours: 24,
};

const formData: PublicBookingFormData = {
  client_first_name: 'Juan',
  client_last_name: 'Perez',
  client_phone: '+541122334455',
  client_email: 'juan@example.com',
  client_notes: '',
};

describe('buildBookingRequest', () => {
  it('maps slotInfo and formData into a PublicBookingRequest', () => {
    const request = buildBookingRequest(slotInfo, formData, 'hold-1');
    expect(request).toEqual({
      complex_id: 'c1',
      court_id: 'ct1',
      date: '2026-03-20',
      start_time: '10:00',
      duration_minutes: 90,
      client_first_name: 'Juan',
      client_last_name: 'Perez',
      client_phone: '+541122334455',
      client_email: 'juan@example.com',
      client_notes: undefined,
      hold_token: 'hold-1',
    });
  });

  it('carries the chosen duration through verbatim', () => {
    const request = buildBookingRequest({ ...slotInfo, durationMinutes: 120 }, formData, 'hold-1');
    expect(request.duration_minutes).toBe(120);
  });

  it('converts a blank client_notes to undefined', () => {
    const request = buildBookingRequest(slotInfo, { ...formData, client_notes: '' }, 'hold-1');
    expect(request.client_notes).toBeUndefined();
  });

  it('preserves a non-blank client_notes', () => {
    const request = buildBookingRequest(
      slotInfo,
      { ...formData, client_notes: 'Llegamos 10 min antes' },
      'hold-1',
    );
    expect(request.client_notes).toBe('Llegamos 10 min antes');
  });
});

describe('buildBookingRequest hold', () => {
  it('sends the hold token and never a turnstile_token', () => {
    const request = buildBookingRequest(slotInfo, formData, 'hold-1');
    expect(request.hold_token).toBe('hold-1');
    expect(request).not.toHaveProperty('turnstile_token');
  });
});
