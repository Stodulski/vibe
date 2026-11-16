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

describe('BookingSummaryCard bare, inside the checkout band', () => {
  it('draws no card of its own: no surface, rounding or padding', () => {
    render(<BookingSummaryCard slotInfo={baseSlotInfo} pricing={computeBookingPricing(baseSlotInfo)} bare />);

    const section = screen.getByRole('region', { name: /resumen de la reserva/i });
    expect(section).not.toHaveClass('bg-primary-500');
    expect(section).not.toHaveClass('rounded-3xl');
    expect(section).not.toHaveClass('p-6');
  });
});

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

describe('BookingSummaryCard, the amount due now', () => {
  it('never breaks the label or the amount, and lets the amount drop under the label when a 320px ticket is too narrow', () => {
    renderCard(baseSlotInfo);

    const label = screen.getByText('Pagás ahora');
    expect(label).toHaveClass('whitespace-nowrap');
    expect(label.nextElementSibling).toHaveClass('whitespace-nowrap');
    expect(label.parentElement).toHaveClass('flex-wrap');
  });
});
