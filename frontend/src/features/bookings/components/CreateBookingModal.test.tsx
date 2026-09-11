import { makePrice } from '@/test/factories';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderWithProviders, screen, userEvent } from '@/test/test-utils';
import { CreateBookingModal } from './CreateBookingModal';
import type { CourtWithPrices } from '@/shared/types/api.types';
import { ES_AR } from '@/shared/i18n/es_AR';

const mockMutate = vi.fn();

vi.mock('../hooks/useCreateBooking', () => ({
  useCreateBooking: () => ({
    mutate: mockMutate,
    isPending: false,
  }),
}));

vi.mock('../hooks/useBookings', () => ({
  useBookings: () => ({
    data: [],
    isLoading: false,
  }),
}));

const mockCourts: CourtWithPrices[] = [
  {
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
    ],
  },
  {
    id: 'ct2',
    complex_id: 'c1',
    name: 'Cancha 2',
    sport: 'padel',
    court_type: 'indoor',
    is_active: true,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    prices: [],
  },
];

describe('CreateBookingModal', () => {
  const defaultProps = {
    open: true,
    onClose: vi.fn(),
    complexId: 'c1',
    courts: mockCourts,
    depositPercentage: 30,
    schedules: [],
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders dialog title', () => {
    renderWithProviders(<CreateBookingModal {...defaultProps} />);
    expect(screen.getAllByText(/nueva reserva/i).length).toBeGreaterThanOrEqual(1);
  });

  it('renders step 1 (schedule) fields and a "Siguiente" button, not the client/payment steps', () => {
    renderWithProviders(<CreateBookingModal {...defaultProps} />);
    expect(
      screen.getByText(
        (_, element) => element?.tagName.toLowerCase() === 'label' && element.textContent === 'Fecha (requerido)',
      ),
    ).toBeInTheDocument();
    expect(screen.getAllByText(/cancha/i).length).toBeGreaterThanOrEqual(1);
    expect(screen.getByRole('button', { name: /siguiente/i })).toBeInTheDocument();
    expect(screen.queryByLabelText(/nombre/i)).not.toBeInTheDocument();
    expect(screen.queryByLabelText(/tel.fono/i)).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /^nueva reserva$/i })).not.toBeInTheDocument();
  });

  it('renders a cancel button on step 1', () => {
    renderWithProviders(<CreateBookingModal {...defaultProps} />);
    expect(screen.getByRole('button', { name: /cancelar/i })).toBeInTheDocument();
  });

  // This asserted the opposite: that a club without MercadoPago was told it
  // could not create bookings, with the submit disabled behind the warning.
  // The server never required it — the MercadoPago rule covers PUBLIC bookings
  // only — so a venue taking cash could not write down a booking taken by
  // phone, which is how most courts still get reserved.
  it('creates bookings without MercadoPago, which only online booking needs', () => {
    renderWithProviders(<CreateBookingModal {...defaultProps} />);

    expect(screen.queryByText(/mercadopago/i)).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: /siguiente/i })).toBeEnabled();
  });

  it('does not render when closed', () => {
    renderWithProviders(<CreateBookingModal {...defaultProps} open={false} />);
    expect(screen.queryByText(/crear reserva/i)).not.toBeInTheDocument();
  });

  it('does not advance to step 2 when step 1 is invalid (no court/time chosen)', async () => {
    const user = userEvent.setup();
    renderWithProviders(<CreateBookingModal {...defaultProps} />);

    await user.click(screen.getByRole('button', { name: /siguiente/i }));

    // Still on step 1: the client fields never appear, and the "Siguiente"
    // button (not step 2's) is still on screen.
    expect(screen.queryByLabelText(/nombre/i)).not.toBeInTheDocument();
    expect(screen.queryByLabelText(/tel.fono/i)).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: /siguiente/i })).toBeInTheDocument();
    // Asserted against the key, not a copy of the sentence. This line used to
    // hold the Spanish verbatim and broke the day the string was corrected —
    // the test was checking the wording, which is not its job.
    const alertsText = screen
      .getAllByRole('alert')
      .map((el) => el.textContent)
      .join(' ');
    expect(alertsText).toContain(ES_AR.validation.selectCourt);
  });
});
