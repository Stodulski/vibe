import { describe, it, expect, vi } from 'vitest';
import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { BookingInfoSection } from './BookingInfoSection';
import { renderWithProviders } from '@/test/test-utils';
import { makeBooking } from '@/test/factories';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { Booking, Payment } from '@/shared/types/api.types';

function createBooking(overrides?: Partial<Booking>): Booking {
  return makeBooking({
    id: 'b1',
    complex_id: 'c1',
    court_id: 'ct1',
    client_id: 'cl1',
    court_name: 'Cancha 1',
    client_name: 'Cliente 1',
    client_phone: '1155550000',
    date: '2026-03-16',
    start_time: '10:00',
    duration_minutes: 90,
    price: 15000,
    deposit_amount: 0,
    status: 'confirmed',
    collection_status: 'fully_paid',
    refund_status: 'none',
    notes: '',
    reminder_sent_2h: false,
    created_at: '2026-03-14T10:00:00Z',
    updated_at: '2026-03-14T10:00:00Z',
    ...overrides,
  });
}

function createPayment(overrides?: Partial<Payment>): Payment {
  return {
    id: 'p1',
    booking_id: 'b1',
    complex_id: 'c1',
    amount: 15000,
    service_fee: 0,
    method: 'cash',
    status: 'fully_paid',
    refund_amount: 0,
    created_at: '2026-03-14T10:00:00Z',
    updated_at: '2026-03-14T10:00:00Z',
    ...overrides,
  };
}

describe('BookingInfoSection', () => {
  it('states the price once, with the method, when one payment covers it', () => {
    const booking = createBooking({ price: 15000 });
    const payment = createPayment({ amount: 15000, method: 'cash' });
    renderWithProviders(<BookingInfoSection booking={booking} payments={[payment]} />);

    expect(screen.getByText('$150 · Efectivo')).toBeInTheDocument();
    expect(screen.getAllByText('Precio')).toHaveLength(1);
    expect(screen.queryByText('Monto')).not.toBeInTheDocument();
  });

  it('labels a lone deposit payment as Seña and keeps the price row', () => {
    const booking = createBooking({
      price: 15000,
      deposit_amount: 4500,
      collection_status: 'deposit_paid',
      refund_status: 'none',
    });
    const payment = createPayment({ amount: 4500, method: 'transfer' });
    renderWithProviders(<BookingInfoSection booking={booking} payments={[payment]} />);

    expect(screen.getByText('Precio')).toBeInTheDocument();
    expect(screen.getByText('$150')).toBeInTheDocument();
    expect(screen.getByText('Seña')).toBeInTheDocument();
    expect(screen.getByText('$45 · Transferencia')).toBeInTheDocument();
  });

  it('falls back to Monto for a payment that is neither the price nor the deposit', () => {
    const booking = createBooking({ price: 15000, deposit_amount: 4500 });
    const payment = createPayment({ amount: 7000, method: 'transfer' });
    renderWithProviders(<BookingInfoSection booking={booking} payments={[payment]} />);

    expect(screen.getByText('Monto')).toBeInTheDocument();
    expect(screen.getByText('$70 · Transferencia')).toBeInTheDocument();
  });

  it('shows a row per payment with role, amount and method when there are two payments', () => {
    const booking = createBooking({ price: 15000 });
    const deposit = createPayment({ id: 'p1', amount: 4500, method: 'transfer' });
    const remainder = createPayment({ id: 'p2', amount: 10500, method: 'cash' });
    renderWithProviders(<BookingInfoSection booking={booking} payments={[deposit, remainder]} />);

    expect(screen.getByText('Seña')).toBeInTheDocument();
    expect(screen.getByText('$45 · Transferencia')).toBeInTheDocument();
    expect(screen.getByText('Resto')).toBeInTheDocument();
    expect(screen.getByText('$105 · Efectivo')).toBeInTheDocument();
  });

  it('shows a loading placeholder instead of an empty ledger while payments are still loading', () => {
    const booking = createBooking({ price: 15000 });
    renderWithProviders(<BookingInfoSection booking={booking} payments={[]} paymentsPending />);

    expect(screen.getByRole('status', { name: ES_AR.common.loading })).toBeInTheDocument();
  });

  it('shows a retry action instead of treating a failed fetch as no payments', async () => {
    const user = userEvent.setup();
    const booking = createBooking({ price: 15000 });
    const onRetryPayments = vi.fn();
    renderWithProviders(
      <BookingInfoSection booking={booking} payments={[]} paymentsError onRetryPayments={onRetryPayments} />,
    );

    expect(screen.getByText(ES_AR.bookings.paymentsLoadError)).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: ES_AR.layout.retry }));
    expect(onRetryPayments).toHaveBeenCalledTimes(1);
  });
});

describe('BookingInfoSection, a booking paid twice online', () => {
  it('labels a full online payment as the price and a second online payment as repeated', () => {
    const booking = createBooking({ price: 15000, deposit_amount: 4500, status: 'cancelled' });
    const paid = createPayment({ id: 'p1', amount: 15000, method: 'mercadopago' });
    const repeated = createPayment({ id: 'p2', amount: 15000, method: 'mercadopago', refund_amount: 15000 });
    renderWithProviders(<BookingInfoSection booking={booking} payments={[paid, repeated]} />);

    expect(screen.getByText('Precio')).toBeInTheDocument();
    expect(screen.getByText('Pago repetido')).toBeInTheDocument();
    expect(screen.getByText('Reembolsado (Pago repetido)')).toBeInTheDocument();
    expect(screen.queryByText('Seña')).not.toBeInTheDocument();
    expect(screen.queryByText('Resto')).not.toBeInTheDocument();
  });

  it('labels a second online deposit as repeated, not as the rest of the price', () => {
    const booking = createBooking({ price: 15000, deposit_amount: 4500, status: 'cancelled' });
    const deposit = createPayment({ id: 'p1', amount: 4500, method: 'mercadopago' });
    const again = createPayment({ id: 'p2', amount: 4500, method: 'mercadopago', refund_amount: 4500 });
    renderWithProviders(<BookingInfoSection booking={booking} payments={[deposit, again]} />);

    expect(screen.getByText('Seña')).toBeInTheDocument();
    expect(screen.getByText('Pago repetido')).toBeInTheDocument();
    expect(screen.queryByText('Resto')).not.toBeInTheDocument();
  });
});

describe('BookingInfoSection refund summary', () => {
  it('says how much is still owed back by hand on a cancelled cash booking', () => {
    const booking = createBooking({ status: 'cancelled', refund_status: 'partial', price: 3_600_000 });
    const payment = createPayment({ amount: 3_600_000, method: 'cash' });
    renderWithProviders(<BookingInfoSection booking={booking} payments={[payment]} />);

    expect(screen.getByText(ES_AR.bookings.refundOwedLabel)).toBeInTheDocument();
    expect(screen.getByText('$36.000')).toBeInTheDocument();
    expect(screen.getByText(ES_AR.bookings.refundOwedHint)).toBeInTheDocument();
  });

  it('says a Mercado Pago refund is on its way', () => {
    const booking = createBooking({ status: 'cancelled', refund_status: 'pending' });
    renderWithProviders(<BookingInfoSection booking={booking} payments={[createPayment({ method: 'mercadopago' })]} />);

    expect(screen.getByText(ES_AR.bookings.refundOwner.queued)).toBeInTheDocument();
  });

  it('shows no refund summary when nothing is owed back', () => {
    renderWithProviders(<BookingInfoSection booking={createBooking()} payments={[createPayment()]} />);

    expect(screen.queryByText(ES_AR.bookings.refundOwedLabel)).not.toBeInTheDocument();
  });
});

describe('BookingInfoSection payment status', () => {
  it('shows only the refund status once a refund is open', () => {
    const booking = createBooking({ status: 'cancelled', collection_status: 'fully_paid', refund_status: 'partial' });
    renderWithProviders(<BookingInfoSection booking={booking} payments={[createPayment()]} />);

    expect(screen.getByText(ES_AR.bookings.paymentStatuses.partial_refund)).toBeInTheDocument();
    expect(screen.queryByText(ES_AR.bookings.paymentStatuses.fully_paid)).not.toBeInTheDocument();
  });

  it('shows the collection status while nothing is owed back', () => {
    renderWithProviders(<BookingInfoSection booking={createBooking()} payments={[createPayment()]} />);

    expect(screen.getByText(ES_AR.bookings.paymentStatuses.fully_paid)).toBeInTheDocument();
  });
});

describe('BookingInfoSection, a played booking not paid in full', () => {
  it('keeps the booking status about the booking and puts the money owed in the payment status', () => {
    const booking = createBooking({
      status: 'completed',
      collection_status: 'unpaid',
      attended_at: '2026-03-18T15:00:00Z',
    });
    renderWithProviders(<BookingInfoSection booking={booking} payments={[]} />);

    expect(screen.getByText(ES_AR.bookings.statuses.completed)).toBeInTheDocument();
    expect(screen.getByText(ES_AR.bookings.paymentStatuses.outstanding)).toBeInTheDocument();
    expect(screen.queryByText(ES_AR.bookings.paymentStatuses.unpaid)).not.toBeInTheDocument();
  });

  it('counts a booking with only its deposit paid as money still owed', () => {
    const booking = createBooking({ status: 'completed', collection_status: 'deposit_paid', deposit_amount: 5000 });
    renderWithProviders(<BookingInfoSection booking={booking} payments={[]} />);

    expect(screen.getByText(ES_AR.bookings.paymentStatuses.outstanding)).toBeInTheDocument();
  });

  it('reads paid in full once the booking is paid in full', () => {
    const booking = createBooking({
      status: 'completed',
      collection_status: 'fully_paid',
      attended_at: '2026-03-18T15:00:00Z',
    });
    renderWithProviders(<BookingInfoSection booking={booking} payments={[createPayment()]} />);

    expect(screen.getByText(ES_AR.bookings.statuses.completed)).toBeInTheDocument();
    expect(screen.queryByText(ES_AR.bookings.paymentStatuses.outstanding)).not.toBeInTheDocument();
  });
});

describe('BookingInfoSection, a booking whose hours are over', () => {
  it('reads "Turno finalizado" until someone marks whether the client came', () => {
    renderWithProviders(
      <BookingInfoSection
        booking={createBooking({ status: 'completed', collection_status: 'fully_paid' })}
        payments={[]}
      />,
    );

    expect(screen.getByText(ES_AR.bookings.statuses.finished)).toBeInTheDocument();
    expect(screen.queryByText(ES_AR.bookings.statuses.completed)).not.toBeInTheDocument();
  });

  it('reads "Turno finalizado" for a confirmed booking the completion sweep has not reached yet', () => {
    const ended = new Date(Date.now() - 60_000).toISOString();
    renderWithProviders(
      <BookingInfoSection
        booking={createBooking({ status: 'confirmed', collection_status: 'deposit_paid', ends_at: ended })}
        payments={[]}
      />,
    );

    expect(screen.getByText(ES_AR.bookings.statuses.finished)).toBeInTheDocument();
    expect(screen.getByText(ES_AR.bookings.paymentStatuses.outstanding)).toBeInTheDocument();
  });

  it('keeps an upcoming confirmed booking as confirmed', () => {
    const ahead = new Date(Date.now() + 3_600_000).toISOString();
    renderWithProviders(
      <BookingInfoSection
        booking={createBooking({ status: 'confirmed', collection_status: 'unpaid', ends_at: ahead })}
        payments={[]}
      />,
    );

    expect(screen.getByText(ES_AR.bookings.statuses.confirmed)).toBeInTheDocument();
    expect(screen.getByText(ES_AR.bookings.paymentStatuses.unpaid)).toBeInTheDocument();
  });
});

describe('BookingInfoSection, a Mercado Pago refund in flight', () => {
  it('reads as a pending refund even before the booking row says so', () => {
    const booking = createBooking({ status: 'cancelled', collection_status: 'deposit_paid', refund_status: 'none' });
    const payment = createPayment({ method: 'mercadopago', status: 'refund_pending', mp_payment_id: 'mp1' });
    renderWithProviders(<BookingInfoSection booking={booking} payments={[payment]} />);

    expect(screen.getByText(ES_AR.bookings.paymentStatuses.refund_pending)).toBeInTheDocument();
    expect(screen.queryByText(ES_AR.bookings.paymentStatuses.deposit_paid)).not.toBeInTheDocument();
    expect(screen.getByText(ES_AR.bookings.refundOwner.queued)).toBeInTheDocument();
  });
});
