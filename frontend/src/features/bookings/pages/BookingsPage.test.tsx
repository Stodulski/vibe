import { describe, it, expect, vi } from 'vitest';
import { screen } from '@testing-library/react';
import { renderWithProviders } from '@/test/test-utils';

vi.mock('@/shared/hooks/usePageTitle', () => ({ usePageTitle: vi.fn() }));
vi.mock('@/shared/components/ui/button', async () => {
  const { passthrough } = await import('@/test/ui-mocks');
  return { Button: passthrough('button') };
});
vi.mock('@/features/courts/components/BlockSlotModal', () => ({ BlockSlotModal: () => null }));
vi.mock('@/features/courts/components/BlockedSlotDetail', () => ({
  BlockedSlotDetail: () => null,
}));
vi.mock('@/features/courts/hooks/useDeleteBlockedSlot', () => ({
  useDeleteBlockedSlot: () => ({ mutate: vi.fn(), isPending: false }),
}));
vi.mock('@/features/complex/hooks/useSelectedComplex', () => ({
  useSelectedComplex: vi.fn().mockReturnValue({ selectedComplexId: 'c1' }),
}));
// Kept out of the factory so the mock body stays under the repo's
// max-lines-per-function cap as the page's state surface grows.
const bookingsPageState = {
  selectedComplexId: 'c1',
  selectedDate: '2026-03-18',
  dateLabel: '18 de marzo',
  isToday: true,
  calendarOpen: false,
  courts: [],
  bookings: [],
  blockedSlots: [],
  schedules: [],
  isLoading: false,
  isPast: false,
  complex: null,
  createOpen: false,
  createPrefill: undefined,
  detailOpen: false,
  selectedBooking: null,
  cancelOpen: false,
  cancelBookingInfo: undefined,
  paymentOpen: false,
  cancelBooking: { isPending: false },
  confirmPayment: { isPending: false },
  setCalendarOpen: vi.fn(),
  setSelectedDate: vi.fn(),
  handlePrevDay: vi.fn(),
  handleNextDay: vi.fn(),
  handleGoToToday: vi.fn(),
  handleSelectBooking: vi.fn(),
  handleCreateFromSlot: vi.fn(),
  handleCancelBooking: vi.fn(),
  handleConfirmPaymentOpen: vi.fn(),
  handleNoShow: vi.fn(),
  handleOpenCreate: vi.fn(),
  setCreateOpen: vi.fn(),
  setDetailOpen: vi.fn(),
  setSelectedBooking: vi.fn(),
  setCancelOpen: vi.fn(),
  handleConfirmCancel: vi.fn(),
  setPaymentOpen: vi.fn(),
  handlePaymentSubmit: vi.fn(),
  manualRefundOpen: false,
  manualRefundAmount: 0,
  markManualRefund: { isPending: false },
  setManualRefundOpen: vi.fn(),
  handleManualRefundOpen: vi.fn(),
  handleConfirmManualRefund: vi.fn(),
};
vi.mock('./bookings/useBookingsPage', () => ({
  useBookingsPage: () => bookingsPageState,
}));
vi.mock('./bookings/DateNavStrip', () => ({
  DateNavStrip: () => <div data-testid="date-nav">DateNavStrip</div>,
}));
vi.mock('./bookings/BookingContent', () => ({
  BookingContent: () => <div data-testid="booking-content">BookingContent</div>,
}));
vi.mock('./bookings/BookingsPageModals', () => ({
  BookingsPageModals: () => <div data-testid="booking-modals">BookingsPageModals</div>,
}));

describe('BookingsPage', () => {
  it('renders page title', async () => {
    const BookingsPage = (await import('./BookingsPage')).default;
    renderWithProviders(<BookingsPage />);
    expect(screen.getByText('Reservas')).toBeInTheDocument();
  });

  it('renders date nav strip', async () => {
    const BookingsPage = (await import('./BookingsPage')).default;
    renderWithProviders(<BookingsPage />);
    expect(screen.getByTestId('date-nav')).toBeInTheDocument();
  });

  it('renders booking content', async () => {
    const BookingsPage = (await import('./BookingsPage')).default;
    renderWithProviders(<BookingsPage />);
    expect(screen.getByTestId('booking-content')).toBeInTheDocument();
  });

  it('renders create booking button', async () => {
    const BookingsPage = (await import('./BookingsPage')).default;
    renderWithProviders(<BookingsPage />);
    expect(screen.getByText('Nueva reserva')).toBeInTheDocument();
  });

  it('renders block slot button', async () => {
    const BookingsPage = (await import('./BookingsPage')).default;
    renderWithProviders(<BookingsPage />);
    expect(screen.getByText('Bloquear horario')).toBeInTheDocument();
  });

  it('returns null when no complex is selected yet, without calling useBookingsPage', async () => {
    const { useSelectedComplex } = await import('@/features/complex/hooks/useSelectedComplex');
    vi.mocked(useSelectedComplex).mockReturnValueOnce({
      selectedComplexId: null,
      complex: null,
      complexes: [],
      needsOnboarding: false,
      isLoading: false,
    });
    const BookingsPage = (await import('./BookingsPage')).default;
    const { container } = renderWithProviders(<BookingsPage />);
    expect(container.innerHTML).toBe('');
  });
});
