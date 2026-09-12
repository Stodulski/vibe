import { describe, it, expect, beforeEach } from 'vitest';
import { BOOKING_INFO_KEY, readStoredBookingInfo } from './storedBookingInfo';

const validBookingInfo = {
  courtName: 'Cancha 1',
  date: '2026-03-20',
  startTime: '10:00',
  startsAt: '2026-03-20T10:00:00-03:00',
  endsAt: '2026-03-20T11:30:00-03:00',
  price: 1_000_000,
  depositAmount: 300_000,
  complexName: 'Club Norte',
  complexPhone: '1155550000',
  cancellationHours: 24,
  clientPhone: '1122334455',
};

describe('readStoredBookingInfo', () => {
  beforeEach(() => {
    sessionStorage.clear();
  });

  it('returns null when nothing is stored', () => {
    expect(readStoredBookingInfo()).toBeNull();
  });

  it('returns the parsed value when it matches the schema', () => {
    sessionStorage.setItem(BOOKING_INFO_KEY, JSON.stringify(validBookingInfo));
    expect(readStoredBookingInfo()).toEqual(validBookingInfo);
  });

  it('returns null on malformed JSON instead of throwing', () => {
    sessionStorage.setItem(BOOKING_INFO_KEY, 'not-json{');
    expect(readStoredBookingInfo()).toBeNull();
  });

  it('returns null when a required field is missing', () => {
    sessionStorage.setItem(BOOKING_INFO_KEY, JSON.stringify({ courtName: 'Cancha 1' }));
    expect(readStoredBookingInfo()).toBeNull();
  });

  it('returns null when a field is null instead of the expected type', () => {
    sessionStorage.setItem(BOOKING_INFO_KEY, JSON.stringify({ ...validBookingInfo, price: null }));
    expect(readStoredBookingInfo()).toBeNull();
  });

  it('returns null when the stored value is a bare array, not the expected object', () => {
    sessionStorage.setItem(BOOKING_INFO_KEY, JSON.stringify([1, 2, 3]));
    expect(readStoredBookingInfo()).toBeNull();
  });
});
