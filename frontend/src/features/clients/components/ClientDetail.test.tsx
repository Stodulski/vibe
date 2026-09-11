import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderWithProviders, screen } from '@/test/test-utils';
import { ClientDetail } from './ClientDetail';
import type { Client } from '@/shared/types/api.types';

vi.mock('../hooks/useClient', () => ({
  useClient: () => ({
    data: null,
    isLoading: false,
  }),
}));

vi.mock('../hooks/useUpdateClient', () => ({
  useUpdateClient: () => ({
    mutate: vi.fn(),
    isPending: false,
  }),
}));

export const mockClient: Client = {
  id: 'cl1',
  complex_id: 'c1',
  first_name: 'Juan',
  last_name: 'Perez',
  phone: '1155550000',
  email: 'juan@test.com',
  notes: 'Cliente frecuente',
  is_blocked: false,
  total_bookings: 15,
  no_shows: 1,
  created_at: '2026-01-15T10:00:00Z',
  updated_at: '2026-01-15T10:00:00Z',
};

export const defaultProps = {
  open: true,
  onClose: vi.fn(),
  client: mockClient,
  complexId: 'c1',
  onBlock: vi.fn(),
};

describe('ClientDetail', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders client full name', () => {
    renderWithProviders(<ClientDetail {...defaultProps} />);
    expect(screen.getByText('Juan Perez')).toBeInTheDocument();
  });

  it('renders client phone', () => {
    renderWithProviders(<ClientDetail {...defaultProps} />);
    expect(screen.getByText('1155550000')).toBeInTheDocument();
  });

  it('renders client email', () => {
    renderWithProviders(<ClientDetail {...defaultProps} />);
    expect(screen.getByText('juan@test.com')).toBeInTheDocument();
  });

  it('renders total bookings stat', () => {
    renderWithProviders(<ClientDetail {...defaultProps} />);
    expect(screen.getByText('15')).toBeInTheDocument();
    expect(screen.getByText('Reservas')).toBeInTheDocument();
  });

  it('renders no shows stat', () => {
    renderWithProviders(<ClientDetail {...defaultProps} />);
    expect(screen.getByText('1')).toBeInTheDocument();
    expect(screen.getByText('Ausencias')).toBeInTheDocument();
  });

  it('renders attendance percentage', () => {
    renderWithProviders(<ClientDetail {...defaultProps} />);
    // (15 - 1) / 15 * 100 = 93%
    expect(screen.getByText('93%')).toBeInTheDocument();
    expect(screen.getByText('Asistencia')).toBeInTheDocument();
  });

  // The initials avatar is gone; the header now identifies the client by name
  // with the phone as its subtitle.
  it('renders the phone as the header subtitle', () => {
    renderWithProviders(<ClientDetail {...defaultProps} />);
    expect(screen.getByText(mockClient.phone)).toBeInTheDocument();
  });
});
