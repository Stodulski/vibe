import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderWithProviders, screen } from '@/test/test-utils';
import { makeBooking } from '@/test/factories';
import { ES_AR } from '@/shared/i18n/es_AR';
import { BookingDetail } from './BookingDetail';
import type { Booking } from '@/shared/types/api.types';

vi.mock('../api/bookings.api', () => ({ bookingsApi: { getById: vi.fn() } }));

const t = ES_AR.bookings;

async function renderDetail(booking: Booking) {
  const { bookingsApi } = await import('../api/bookings.api');
  vi.mocked(bookingsApi.getById).mockResolvedValue({ booking, payments: [] });
  renderWithProviders(
    <BookingDetail
      open
      onClose={vi.fn()}
      booking={booking}
      complexId="c1"
      onCancel={vi.fn()}
      onConfirmPayment={vi.fn()}
      onNoShow={vi.fn()}
      onManualRefund={vi.fn()}
    />,
  );
}

describe('BookingDetail footer', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('pins the positive primary in the shared footer, with the standard button radius', async () => {
    await renderDetail(makeBooking({ status: 'confirmed', collection_status: 'deposit_paid', refund_status: 'none' }));

    const button = await screen.findByRole('button', { name: t.confirmPayment });
    expect(button.parentElement).toHaveAttribute('data-slot', 'sheet-footer');
    expect(button.parentElement).toHaveClass('sm:px-6');
    expect(button).not.toHaveClass('rounded-lg');
  });

  it('has no footer when there is no positive action', async () => {
    await renderDetail(makeBooking({ status: 'confirmed', collection_status: 'fully_paid', refund_status: 'none' }));

    await screen.findByRole('button', { name: ES_AR.common.rowActionsLabel });
    expect(document.querySelector('[data-slot="sheet-footer"]')).toBeNull();
  });
});
