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
  it('shows the sport and the court type', () => {
    render(<SlotHeaderRow slotInfo={{ ...baseSlotInfo, sport: 'padel', courtType: 'indoor' }} />);

    expect(screen.getByText('Pádel')).toBeInTheDocument();
    expect(screen.getByText('Techada')).toBeInTheDocument();
  });

  it('shows the surface and the lighting after the court type, like the court picker', () => {
    render(
      <SlotHeaderRow
        slotInfo={{ ...baseSlotInfo, sport: 'padel', courtType: 'indoor', surface: 'clay', hasLighting: true }}
      />,
    );

    expect(screen.getByText('Polvo de ladrillo')).toBeInTheDocument();
    expect(screen.getByText('Con iluminación')).toBeInTheDocument();
  });

  it('renders neither line when sport and court type are both absent', () => {
    render(<SlotHeaderRow slotInfo={baseSlotInfo} />);

    expect(screen.queryByText(/·/)).not.toBeInTheDocument();
  });

  it('shows the sport alone, with no separator, when the court type is absent', () => {
    render(<SlotHeaderRow slotInfo={{ ...baseSlotInfo, sport: 'padel' }} />);

    expect(screen.getByText('Pádel')).toBeInTheDocument();
    expect(screen.queryByText(/·/)).not.toBeInTheDocument();
  });
});
