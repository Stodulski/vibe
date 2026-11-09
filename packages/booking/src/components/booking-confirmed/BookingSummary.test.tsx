import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';
import { BookingSummary } from './BookingSummary';
import type { BookingInfo } from './types';

const bookingInfo: BookingInfo = {
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
  clientPhone: '1155551111',
};

describe('BookingSummary ticket', () => {
  it('is a solid green card at every width', () => {
    render(<BookingSummary bookingInfo={bookingInfo} />);

    expect(screen.getByRole('region', { name: 'Detalle de la reserva' })).toHaveClass('bg-primary-500');
  });
});
