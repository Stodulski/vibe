import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderWithProviders, screen } from '@/test/test-utils';
import { CourtForm } from './CourtForm';

const mockCreateMutate = vi.fn();
const mockUpdateMutate = vi.fn();

vi.mock('../hooks/useCreateCourt', () => ({
  useCreateCourt: () => ({
    mutate: mockCreateMutate,
    isPending: false,
  }),
}));

vi.mock('../hooks/useUpdateCourt', () => ({
  useUpdateCourt: () => ({
    mutate: mockUpdateMutate,
    isPending: false,
  }),
}));

describe('CourtForm', () => {
  const defaultProps = {
    open: true,
    onClose: vi.fn(),
    complexId: 'c1',
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders create title when no court provided', () => {
    renderWithProviders(<CourtForm {...defaultProps} />);
    expect(screen.getByText(/agregar cancha/i)).toBeInTheDocument();
  });

  it('renders edit title when court is provided', () => {
    const court = {
      id: 'ct1',
      complex_id: 'c1',
      name: 'Cancha 1',
      sport: 'padel' as const,
      court_type: 'outdoor' as const,
      is_active: true,
      created_at: '2026-01-01T00:00:00Z',
      updated_at: '2026-01-01T00:00:00Z',
      prices: [],
    };
    renderWithProviders(<CourtForm {...defaultProps} court={court} />);
    expect(screen.getByText(/editar cancha/i)).toBeInTheDocument();
  });

  it('renders name input', () => {
    renderWithProviders(<CourtForm {...defaultProps} />);
    expect(screen.getByLabelText(/nombre/i)).toBeInTheDocument();
  });

  it('renders sport selector', () => {
    renderWithProviders(<CourtForm {...defaultProps} />);
    expect(screen.getByText(/deporte/i)).toBeInTheDocument();
  });

  it('renders court type selector', () => {
    renderWithProviders(<CourtForm {...defaultProps} />);
    expect(screen.getByText(/tipo de cancha/i)).toBeInTheDocument();
  });

  it('renders cancel button', () => {
    renderWithProviders(<CourtForm {...defaultProps} />);
    expect(screen.getByRole('button', { name: /cancelar/i })).toBeInTheDocument();
  });

  it('renders create button for new court', () => {
    renderWithProviders(<CourtForm {...defaultProps} />);
    expect(screen.getByRole('button', { name: /crear/i })).toBeInTheDocument();
  });

  it('does not render when closed', () => {
    renderWithProviders(<CourtForm {...defaultProps} open={false} />);
    expect(screen.queryByText(/crear cancha|nueva cancha/i)).not.toBeInTheDocument();
  });
});
