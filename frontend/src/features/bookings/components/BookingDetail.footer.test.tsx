import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderWithProviders, screen } from '@/test/test-utils';
import { makeBooking } from '@/test/factories';
import { ES_AR } from '@/shared/i18n/es_AR';
import { BookingDetail } from './BookingDetail';
import type { Booking, Payment } from '@/shared/types/api.types';

vi.mock('../api/bookings.api', () => ({ bookingsApi: { getById: vi.fn() } }));

const t = ES_AR.bookings;

// A booking whose hours are still ahead: once they are over the detail offers
// attendance instead of cancelling.
const hourAhead = Date.now() + 3_600_000;
const upcoming = {
  starts_at: new Date(hourAhead).toISOString(),
  ends_at: new Date(hourAhead + 3_600_000).toISOString(),
};

async function renderDetail(booking: Booking, payments: Payment[] = [], onAttended = vi.fn()) {
  const { bookingsApi } = await import('../api/bookings.api');
  vi.mocked(bookingsApi.getById).mockResolvedValue({ booking, payments });
  renderWithProviders(
    <BookingDetail
      open
      onClose={vi.fn()}
      booking={booking}
      complexId="c1"
      onCancel={vi.fn()}
      onConfirmPayment={vi.fn()}
      onNoShow={vi.fn()}
      onAttended={onAttended}
      onManualRefund={vi.fn()}
    />,
  );
}

describe('BookingDetail footer', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('pins the positive primary in the shared footer', async () => {
    await renderDetail(
      makeBooking({ status: 'confirmed', collection_status: 'deposit_paid', refund_status: 'none', ...upcoming }),
    );

    const button = await screen.findByRole('button', { name: t.confirmPayment });
    expect(button.parentElement).toHaveAttribute('data-slot', 'sheet-footer');
  });

  it('has no positive action in the footer when the booking is fully paid', async () => {
    await renderDetail(
      makeBooking({ status: 'confirmed', collection_status: 'fully_paid', refund_status: 'none', ...upcoming }),
    );

    await screen.findByRole('button', { name: t.cancel });
    expect(screen.queryByRole('button', { name: t.confirmPayment })).not.toBeInTheDocument();
  });

  it('shows two secondary actions as footer buttons, with no overflow menu', async () => {
    await renderDetail(
      makeBooking({ status: 'confirmed', collection_status: 'deposit_paid', refund_status: 'none', ...upcoming }),
    );

    const cancel = await screen.findByRole('button', { name: t.cancel });
    const noShow = screen.getByRole('button', { name: t.statuses.no_show });
    expect(cancel.closest('[data-slot="sheet-footer"]')).not.toBeNull();
    expect(noShow.closest('[data-slot="sheet-footer"]')).not.toBeNull();
    expect(screen.queryByRole('button', { name: ES_AR.common.rowActionsLabel })).not.toBeInTheDocument();
  });

  it('puts a lone cancel in the footer for a pending booking', async () => {
    await renderDetail(makeBooking({ status: 'pending', collection_status: 'fully_paid', refund_status: 'none' }));

    const cancel = await screen.findByRole('button', { name: t.cancel });
    expect(cancel.closest('[data-slot="sheet-footer"]')).not.toBeNull();
    expect(screen.queryByRole('button', { name: t.statuses.no_show })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: ES_AR.common.rowActionsLabel })).not.toBeInTheDocument();
  });
});

describe('BookingDetail footer, actions the server would refuse', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('offers no payment to confirm on a no-show, which the server refuses', async () => {
    await renderDetail(makeBooking({ status: 'no_show', collection_status: 'deposit_paid', refund_status: 'none' }));

    await screen.findByText(t.statuses.no_show);
    expect(screen.queryByRole('button', { name: t.confirmPayment })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: t.confirmRemainingPayment })).not.toBeInTheDocument();
  });

  it('waits for a Mercado Pago refund in flight before offering to close the cash part', async () => {
    const payments: Payment[] = [
      {
        id: 'p1',
        booking_id: 'b1',
        complex_id: 'c1',
        amount: 5000,
        service_fee: 0,
        method: 'mercadopago',
        status: 'refund_pending',
        refund_amount: 0,
        mp_payment_id: 'mp1',
        created_at: '',
        updated_at: '',
      },
      {
        id: 'p2',
        booking_id: 'b1',
        complex_id: 'c1',
        amount: 5000,
        service_fee: 0,
        method: 'cash',
        status: 'fully_paid',
        refund_amount: 0,
        created_at: '',
        updated_at: '',
      },
    ];
    await renderDetail(
      makeBooking({ id: 'b1', status: 'cancelled', collection_status: 'fully_paid', refund_status: 'partial' }),
      payments,
    );

    await screen.findByText(t.refundOwedLabel);
    expect(screen.queryByRole('button', { name: t.markManualRefund })).not.toBeInTheDocument();
    expect(screen.getByText(t.refundWaitsForMp)).toBeInTheDocument();
  });
});

describe('BookingDetail footer, a booking whose hours are over', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('asks whether the client came instead of offering to cancel', async () => {
    const onAttended = vi.fn();
    const booking = makeBooking({ status: 'completed', collection_status: 'fully_paid', refund_status: 'none' });
    await renderDetail(booking, [], onAttended);

    const attended = await screen.findByRole('button', { name: t.statuses.completed });
    expect(screen.getByRole('button', { name: t.statuses.no_show })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: t.cancel })).not.toBeInTheDocument();

    attended.click();
    expect(onAttended).toHaveBeenCalledWith(booking);
  });

  it('no longer asks once the client was marked as come', async () => {
    await renderDetail(
      makeBooking({
        status: 'completed',
        collection_status: 'fully_paid',
        refund_status: 'none',
        attended_at: '2026-03-18T15:00:00Z',
      }),
    );

    await screen.findByRole('button', { name: t.statuses.no_show });
    expect(screen.queryByRole('button', { name: t.statuses.completed })).not.toBeInTheDocument();
  });

  it('offers no attendance while the hours are still ahead', async () => {
    await renderDetail(
      makeBooking({ status: 'confirmed', collection_status: 'fully_paid', refund_status: 'none', ...upcoming }),
    );

    await screen.findByRole('button', { name: t.cancel });
    expect(screen.queryByRole('button', { name: t.statuses.completed })).not.toBeInTheDocument();
  });
});
