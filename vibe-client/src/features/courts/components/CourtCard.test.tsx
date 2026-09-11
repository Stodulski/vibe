import { makePrice } from '@/test/factories';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { CourtCard } from './CourtCard';
import type { CourtWithPrices } from '@/shared/types/api.types';

vi.mock('../hooks/useUpdateCourt', () => ({
  useUpdateCourt: () => ({
    mutate: vi.fn(),
    isPending: false,
  }),
}));

vi.mock('../hooks/useDeleteCourt', () => ({
  useDeleteCourt: () => ({
    mutate: vi.fn(),
    isPending: false,
  }),
}));

const mockCourt: CourtWithPrices = {
  id: 'ct1',
  complex_id: 'c1',
  name: 'Cancha 1',
  sport: 'padel',
  court_type: 'outdoor',
  is_active: true,
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
  prices: [
    makePrice({
      id: 'p1',
      court_id: 'ct1',
      price: 1500000,
      day_type: 'monday',
      time_from: '08:00',
      time_to: '23:00',
    }),
    makePrice({
      id: 'p2',
      court_id: 'ct1',
      price: 2000000,
      day_type: 'saturday',
      time_from: '08:00',
      time_to: '23:00',
    }),
  ],
};

const defaultProps = {
  court: mockCourt,
  complexId: 'c1',
  onEdit: vi.fn(),
  onPrices: vi.fn(),
};

describe('CourtCard - content', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders court name', () => {
    render(<CourtCard {...defaultProps} />);
    expect(screen.getByText('Cancha 1')).toBeInTheDocument();
  });

  it('renders sport type', () => {
    render(<CourtCard {...defaultProps} />);
    expect(screen.getByText(/p.del/i)).toBeInTheDocument();
  });

  it('renders court type', () => {
    render(<CourtCard {...defaultProps} />);
    expect(screen.getByText(/descubierta/i)).toBeInTheDocument();
  });

  it('renders active toggle switch', () => {
    render(<CourtCard {...defaultProps} />);
    const switchEl = screen.getByRole('switch');
    expect(switchEl).toBeInTheDocument();
    expect(switchEl).toBeChecked();
  });

  it('renders no prices message when court has no prices', () => {
    const courtNoPrices = { ...mockCourt, prices: [] };
    render(<CourtCard {...defaultProps} court={courtNoPrices} />);
    expect(screen.getByText(/sin precios configurados|no hay precios/i)).toBeInTheDocument();
  });

  it('renders weekday price range', () => {
    render(<CourtCard {...defaultProps} />);
    expect(screen.getByText(/lun - vie/i)).toBeInTheDocument();
  });
});

describe('CourtCard - actions', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders edit button', () => {
    render(<CourtCard {...defaultProps} />);
    expect(screen.getByRole('button', { name: /editar/i })).toBeInTheDocument();
  });

  it('renders prices button', () => {
    render(<CourtCard {...defaultProps} />);
    expect(screen.getByRole('button', { name: /precios/i })).toBeInTheDocument();
  });

  it('renders delete button', () => {
    render(<CourtCard {...defaultProps} />);
    expect(screen.getByRole('button', { name: /eliminar/i })).toBeInTheDocument();
  });

  it('calls onEdit when edit button is clicked', async () => {
    const user = userEvent.setup();
    render(<CourtCard {...defaultProps} />);

    await user.click(screen.getByRole('button', { name: /editar/i }));
    expect(defaultProps.onEdit).toHaveBeenCalledWith(mockCourt);
  });

  it('calls onPrices when prices button is clicked', async () => {
    const user = userEvent.setup();
    render(<CourtCard {...defaultProps} />);

    await user.click(screen.getByRole('button', { name: /precios/i }));
    expect(defaultProps.onPrices).toHaveBeenCalledWith(mockCourt);
  });
});
