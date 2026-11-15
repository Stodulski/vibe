import { Fragment, type ReactNode } from 'react';
import { AlertCircle } from 'lucide-react';
import { ES_AR } from '@/shared/i18n/es_AR';
import { formatHourRange, formatPrice } from '@/shared/lib/utils';
import { BookingStatusText, PaymentStatusText } from '../BookingStatusBadge';
import { InfoRow } from '@/shared/components/common/InfoRow';
import { EmptyState } from '@/shared/components/common/EmptyState';
import { Skeleton } from '@/shared/components/ui/skeleton';
import { bookingDisplayStatus, paymentDisplayStatus } from '@/shared/types/api.types';
import type { Booking, BookingRefundStatus, Payment } from '@/shared/types/api.types';
import { keepTogether } from '@/shared/lib/keepTogether';
import { manualRefundOwed } from '../../lib/manualRefundOwed';
import { mercadoPagoRefundInFlight } from '../../lib/mercadoPagoRefundInFlight';

const t = ES_AR;

function methodLabel(method: string): string {
  // Honestly-typed lookup (not `as keyof typeof`): `payment.method` is a
  // plain string from the API, not provably one of the known keys, so the
  // `??` fallback below is a real runtime possibility, not dead code.
  return (t.bookings.paymentMethods as Record<string, string>)[method] ?? method;
}

function paymentRole(index: number): string {
  return index === 0 ? t.bookings.deposit : t.bookings.remainderPayment;
}

function PaymentExtraRows({ payment, prefix }: { payment: Payment; prefix?: string }) {
  const refundLabel = prefix ? `${t.bookings.refunded} (${prefix})` : t.bookings.refunded;
  const feeLabel = prefix ? `${t.serviceFee.label} (${prefix})` : t.serviceFee.label;
  return (
    <>
      {payment.service_fee > 0 && (
        <InfoRow label={feeLabel}>
          <span className="score-text font-medium">{formatPrice(payment.service_fee)}</span>
        </InfoRow>
      )}
      {payment.refund_amount > 0 && (
        <InfoRow label={refundLabel}>
          <span className="score-text text-warning-text font-medium">{formatPrice(payment.refund_amount)}</span>
          {payment.service_fee > 0 && (
            <p className="text-text-tertiary mt-0.5 text-xs">
              {formatPrice(payment.refund_amount - payment.service_fee)} + {formatPrice(payment.service_fee)}{' '}
              {t.bookings.serviceFeeSuffix}
            </p>
          )}
        </InfoRow>
      )}
    </>
  );
}

// What this single payment covers: the whole price, the deposit, or some
// other partial amount. Naming it lets the row carry the method beside the
// figure instead of repeating a number the section already showed.
function singlePaymentLabel(payment: Payment, bookingPrice: number, depositAmount: number): string {
  if (payment.amount === depositAmount) return t.bookings.deposit;
  if (payment.amount === bookingPrice) return t.bookings.price;
  return t.bookings.paymentAmount;
}

function SinglePaymentRows({
  payment,
  bookingPrice,
  depositAmount,
}: {
  payment: Payment;
  bookingPrice: number;
  depositAmount: number;
}) {
  return (
    <>
      <InfoRow label={singlePaymentLabel(payment, bookingPrice, depositAmount)}>
        <span className="score-text font-medium">
          {formatPrice(payment.amount)} · {methodLabel(payment.method)}
        </span>
      </InfoRow>
      <PaymentExtraRows payment={payment} />
    </>
  );
}

// Each payment on its own line, then the total they add up to — reading
// top-down like a receipt, instead of the price floating above the parts.
function MultiPaymentRows({ payments, total }: { payments: Payment[]; total: number }) {
  return (
    <>
      {payments.map((payment, index) => {
        const role = paymentRole(index);
        return (
          <Fragment key={payment.id}>
            <InfoRow label={role}>
              <span className="score-text font-medium">
                {formatPrice(payment.amount)} · {methodLabel(payment.method)}
              </span>
            </InfoRow>
            <PaymentExtraRows payment={payment} prefix={role} />
          </Fragment>
        );
      })}
      <InfoRow label={t.bookings.total}>
        <span className="score-text font-semibold">{formatPrice(total)}</span>
      </InfoRow>
    </>
  );
}

// A bare figure is worth its own row only while the ledger isn't already
// showing that same amount next to the method that paid it.
function showsBareRow(payments: Payment[], amount: number): boolean {
  if (payments.length >= 2) return false;
  const [only] = payments;
  return only?.amount !== amount;
}

// A couple of label/value skeleton rows in the ledger's own shape, so the
// section doesn't collapse to nothing while the detail request is in flight.
function PaymentLedgerSkeleton() {
  return (
    <div className="space-y-2" role="status" aria-label={t.common.loading}>
      <div className="flex items-center justify-between">
        <Skeleton className="h-4 w-16 rounded" />
        <Skeleton className="h-4 w-24 rounded" />
      </div>
    </div>
  );
}

function PaymentLedgerError({ onRetry }: { onRetry: () => void }) {
  return (
    <EmptyState
      icon={AlertCircle}
      title={t.bookings.paymentsLoadError}
      description={t.bookings.paymentsLoadErrorDescription}
      actionLabel={t.layout.retry}
      onAction={onRetry}
    />
  );
}

// The refund state as it really stands. The booking row keeps 'none' while a
// Mercado Pago refund is in flight (only the payment row moves), which read
// "Seña pagada" on a cancelled booking whose money was on its way back.
function refundState(booking: Booking, payments: Payment[]): BookingRefundStatus {
  if (booking.refund_status === 'none' && mercadoPagoRefundInFlight(payments)) return 'pending';
  return booking.refund_status;
}

// One status, never two side by side: once a refund is open it is the state
// that matters ("Reembolso parcial"), and what was collected is already on the
// price rows below. Until then, the collection status.
function PaymentStatusRow({ booking, payments }: { booking: Booking; payments: Payment[] }) {
  const status = paymentDisplayStatus({ ...booking, refund_status: refundState(booking, payments) });
  return (
    <InfoRow label={t.bookings.paymentStatus}>
      <PaymentStatusText status={status} />
    </InfoRow>
  );
}

// Price and deposit only get a bare row when no payment already carries that
// same figure alongside its method: a multi-payment ledger shows the price
// as its Total, and a single payment is labelled by what it covers.
function PriceAndDepositRows({ booking, payments }: { booking: Booking; payments: Payment[] }) {
  return (
    <>
      {showsBareRow(payments, booking.price) && (
        <InfoRow label={t.bookings.price}>
          <span className="score-text font-medium">{formatPrice(booking.price)}</span>
        </InfoRow>
      )}
      {booking.deposit_amount > 0 && showsBareRow(payments, booking.deposit_amount) && (
        <InfoRow label={t.bookings.deposit}>
          <span className="score-text font-medium">{formatPrice(booking.deposit_amount)}</span>
        </InfoRow>
      )}
    </>
  );
}

// What happens to the money of a cancelled booking, in words: the status pair
// above says a refund is open, this says how much and who moves it. A
// 'partial' refund is the cash or transfer part the owner hands back in
// person; 'pending' is Mercado Pago still sending it.
function RefundSummaryRow({ booking, payments }: { booking: Booking; payments: Payment[] }) {
  const refund = refundState(booking, payments);
  if (refund === 'pending') {
    return (
      <InfoRow label={t.bookings.refundInProgressLabel}>
        <span className="text-text-secondary text-sm">{t.bookings.refundOwner.queued}</span>
      </InfoRow>
    );
  }
  if (refund !== 'partial') return null;
  const owed = manualRefundOwed(payments);
  if (owed <= 0) return null;
  return (
    <InfoRow label={t.bookings.refundOwedLabel}>
      <span className="score-text text-warning-text font-medium">{formatPrice(owed)}</span>
      <p className="text-text-tertiary mt-0.5 text-xs">
        {mercadoPagoRefundInFlight(payments) ? t.bookings.refundWaitsForMp : t.bookings.refundOwedHint}
      </p>
    </InfoRow>
  );
}

function PaymentLedger({
  payments,
  booking,
  isPending,
  isError,
  onRetry,
}: {
  payments: Payment[];
  booking: Booking;
  isPending: boolean;
  isError: boolean;
  onRetry: () => void;
}) {
  if (isPending) return <PaymentLedgerSkeleton />;
  if (isError) return <PaymentLedgerError onRetry={onRetry} />;
  const [first, ...rest] = payments;
  if (!first) return null;
  if (rest.length === 0) {
    return <SinglePaymentRows payment={first} bookingPrice={booking.price} depositAmount={booking.deposit_amount} />;
  }
  return <MultiPaymentRows payments={payments} total={booking.price} />;
}

export function BookingInfoSection({
  booking,
  payments,
  clientRow,
  paymentsPending = false,
  paymentsError = false,
  onRetryPayments,
}: {
  booking: Booking;
  payments: Payment[];
  clientRow?: ReactNode;
  /** Loading/error state of the request `payments` came from — a caller that
   *  always has its payments ready up front (e.g. a test fixture) can omit
   *  all three and get the plain ledger. */
  paymentsPending?: boolean;
  paymentsError?: boolean;
  onRetryPayments?: () => void;
}) {
  // No heading of its own: the sheet header already names this section,
  // right under the title.
  return (
    <div className="space-y-3">
      <InfoRow label={t.bookings.status}>
        <BookingStatusText status={bookingDisplayStatus(booking)} />
      </InfoRow>
      <PaymentStatusRow booking={booking} payments={payments} />
      <PriceAndDepositRows booking={booking} payments={payments} />
      <PaymentLedger
        payments={payments}
        booking={booking}
        isPending={paymentsPending}
        isError={paymentsError}
        onRetry={
          onRetryPayments ??
          (() => {
            /* no-op: caller didn't wire a retry */
          })
        }
      />
      {!paymentsPending && !paymentsError && <RefundSummaryRow booking={booking} payments={payments} />}
      <InfoRow label={t.bookings.time}>
        <span className="score-text font-medium">{formatHourRange(booking.starts_at, booking.ends_at)}</span>
      </InfoRow>
      <InfoRow label={t.bookings.duration}>
        <span className="score-text font-medium">{booking.duration_minutes} min</span>
      </InfoRow>
      <InfoRow label={t.bookings.court}>
        <span className="truncate">{keepTogether(booking.court_name)}</span>
      </InfoRow>
      {clientRow}
    </div>
  );
}
