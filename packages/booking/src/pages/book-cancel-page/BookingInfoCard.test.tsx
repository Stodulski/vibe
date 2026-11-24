import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { BookingInfoCard } from './BookingInfoCard';
import type { CancelInfoResponse } from '../../api/types';

const cancelInfo: CancelInfoResponse = {
  booking: {
    status: 'confirmed',
    date: '2026-03-20',
    start_time: '10:00',
    court_name: 'Cancha 1',
    complex_name: 'Club Norte',
    sport: 'padel',
    court_type: 'outdoor',
    surface: 'synthetic_grass',
    has_lighting: true,
  },
  can_cancel: true,
  can_refund: true,
  refund_method: 'mercadopago',
  cancellation_hours: 24,
};

describe('BookingInfoCard', () => {
  it('shows the court the way the court picker did: roof, surface and lighting', () => {
    render(<BookingInfoCard cancelInfo={cancelInfo} />);

    for (const fact of ['Pádel', 'Descubierta', 'Césped sintético', 'Con iluminación']) {
      expect(screen.getByText(fact)).toBeInTheDocument();
    }
  });
});
