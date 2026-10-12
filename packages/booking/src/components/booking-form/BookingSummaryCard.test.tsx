import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';
import { BookingSummaryCard } from './BookingSummaryCard';
import { computeBookingPricing } from './pricing';
import type { BookingSlotInfo } from './types';

const baseSlotInfo: BookingSlotInfo = {
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

function renderCard(slotInfo: BookingSlotInfo) {
  return render(<BookingSummaryCard slotInfo={slotInfo} pricing={computeBookingPricing(slotInfo)} />);
}

describe('BookingSummaryCard cancellation policy', () => {
  it('states the cancellation window before the customer pays', () => {
    renderCard({ ...baseSlotInfo, cancellationHours: 24 });

    expect(screen.getByText('Podés cancelar hasta 24h antes del turno')).toBeInTheDocument();
  });

  it('hides the cancellation line when the club has no window', () => {
    renderCard({ ...baseSlotInfo, cancellationHours: 0 });

    expect(screen.queryByText(/Podés cancelar/)).not.toBeInTheDocument();
  });
});

describe('BookingSummaryCard remainder note', () => {
  it('says the remaining balance is paid at the club when something is still owed', () => {
    renderCard({ ...baseSlotInfo, depositPercentage: 30 });

    expect(screen.getByText('Resta pagar')).toBeInTheDocument();
    expect(screen.getByText('El saldo restante se abona directamente en el complejo.')).toBeInTheDocument();
  });

  it('leaves the note out when nothing is left to pay', () => {
    renderCard({ ...baseSlotInfo, depositPercentage: 100 });

    expect(screen.queryByText('Resta pagar')).not.toBeInTheDocument();
    expect(screen.queryByText('El saldo restante se abona directamente en el complejo.')).not.toBeInTheDocument();
  });
});
