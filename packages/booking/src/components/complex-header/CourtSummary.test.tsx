import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';

import type { CourtPrice, CourtWithPrices } from '../../api/types';
import { CourtSummary } from './CourtSummary';

function band(price: number): CourtPrice {
  return {
    id: `band-${String(price)}`,
    court_id: 'court',
    price,
    day_type: 'monday',
    time_from: '08:00',
    time_to: '12:00',
    from_min: 480,
    to_min: 720,
  };
}

function court(id: string, sport: CourtWithPrices['sport'], prices: CourtPrice[]): CourtWithPrices {
  return {
    id,
    complex_id: 'complex',
    name: `Cancha ${id}`,
    sport,
    court_type: 'outdoor',
    is_active: true,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    prices,
  };
}

describe('CourtSummary', () => {
  it('states how many courts of each sport there are and the cheapest hour', () => {
    render(<CourtSummary courts={[court('1', 'soccer', [band(2400000)]), court('2', 'soccer', [band(2100000)])]} />);

    expect(screen.getByText('2 canchas de fútbol')).toBeInTheDocument();
    expect(screen.getByText('desde $21.000 la hora')).toBeInTheDocument();
  });

  it('uses the singular for one court and leaves the price out when none is priced', () => {
    render(<CourtSummary courts={[court('1', 'padel', [])]} />);

    expect(screen.getByText('1 cancha de pádel')).toBeInTheDocument();
    expect(screen.queryByText(/desde/)).not.toBeInTheDocument();
  });

  it('renders nothing without courts', () => {
    const { container } = render(<CourtSummary courts={[]} />);

    expect(container).toBeEmptyDOMElement();
  });
});
