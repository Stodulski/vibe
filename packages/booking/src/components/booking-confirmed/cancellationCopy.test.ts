import { describe, expect, it } from 'vitest';
import { cancellationLineText, cancellationWindowText } from './cancellationCopy';
import type { BookingInfo } from './types';

function bookingInfo(overrides: Partial<BookingInfo> = {}): BookingInfo {
  return {
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
    clientPhone: '1123456789',
    ...overrides,
  };
}

describe('cancellationWindowText', () => {
  it('states the cancellation window in hours', () => {
    expect(cancellationWindowText(24)).toBe('Podés cancelar hasta 24h antes del turno');
  });

  it('returns null when there is no window', () => {
    expect(cancellationWindowText(0)).toBeNull();
  });

  it('returns null for a negative window', () => {
    expect(cancellationWindowText(-1)).toBeNull();
  });
});

describe('cancellationLineText generic branch', () => {
  it('keeps the same sentence for an older server that only sends the hours', () => {
    expect(cancellationLineText(bookingInfo({ cancellationHours: 24 }))).toBe(
      'Podés cancelar hasta 24h antes del turno',
    );
  });

  it('hides the line when the hours are zero and there is no cancellation object', () => {
    expect(cancellationLineText(bookingInfo({ cancellationHours: 0 }))).toBeNull();
  });
});
