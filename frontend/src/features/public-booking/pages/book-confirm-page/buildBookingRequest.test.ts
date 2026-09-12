import { describe, it, expect } from 'vitest';
import { buildBookingRequest } from './buildBookingRequest';
import type { BookingSlotInfo, PublicBookingFormData } from '@/features/public-booking';

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
    const request = buildBookingRequest(slotInfo, formData);
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
    });
  });

  it('carries the chosen duration through verbatim', () => {
    const request = buildBookingRequest({ ...slotInfo, durationMinutes: 120 }, formData);
    expect(request.duration_minutes).toBe(120);
  });

  it('converts a blank client_notes to undefined', () => {
    const request = buildBookingRequest(slotInfo, { ...formData, client_notes: '' });
    expect(request.client_notes).toBeUndefined();
  });

  it('preserves a non-blank client_notes', () => {
    const request = buildBookingRequest(slotInfo, {
      ...formData,
      client_notes: 'Llegamos 10 min antes',
    });
    expect(request.client_notes).toBe('Llegamos 10 min antes');
  });
});
