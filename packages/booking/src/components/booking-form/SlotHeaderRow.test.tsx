import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { SlotHeaderRow } from './SlotHeaderRow';
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

describe('SlotHeaderRow — sport and court type line', () => {
  it('joins the sport and the court type with a middle dot, and shows the description', () => {
    render(
      <SlotHeaderRow
        slotInfo={{
          ...baseSlotInfo,
          sport: 'padel',
          courtType: 'indoor',
          courtDescription: 'Césped sintético',
        }}
      />,
    );

    expect(screen.getByText('Pádel')).toBeInTheDocument();
    expect(screen.getByText('Techada')).toBeInTheDocument();
    expect(screen.getByText('Césped sintético')).toBeInTheDocument();
  });

  it('renders neither line when sport, court type and description are all absent', () => {
    render(<SlotHeaderRow slotInfo={baseSlotInfo} />);

    expect(screen.queryByText(/·/)).not.toBeInTheDocument();
    expect(screen.queryByText('Césped sintético')).not.toBeInTheDocument();
  });

  it('shows the sport alone, with no separator, when the court type is absent', () => {
    render(<SlotHeaderRow slotInfo={{ ...baseSlotInfo, sport: 'padel' }} />);

    expect(screen.getByText('Pádel')).toBeInTheDocument();
    expect(screen.queryByText(/·/)).not.toBeInTheDocument();
  });
});
