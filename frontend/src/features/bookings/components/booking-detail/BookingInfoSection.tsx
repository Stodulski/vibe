import { Fragment, type ReactNode } from 'react';
import { AlertCircle } from 'lucide-react';
import { ES_AR } from '@/shared/i18n/es_AR';
import { formatDateShort, formatHourRange, formatPrice } from '@/shared/lib/utils';
import { BookingStatusText, PaymentStatusText } from '../BookingStatusBadge';
import { InfoRow } from '@/shared/components/common/InfoRow';
import { EmptyState } from '@/shared/components/common/EmptyState';
import { Skeleton } from '@/shared/components/ui/skeleton';
import type { Booking, BookingRefundStatus, PaymentDisplayStatus, Payment } from '@/shared/types/api.types';

// The refund half of the pair, in the vocabulary the badge already has labels,
// colours and icons for. 'none' is absent because that half is not rendered at
// all when there is nothing owed back.
const REFUND_DISPLAY: Record<Exclude<BookingRefundStatus, 'none'>, PaymentDisplayStatus> = {
  pending: 'refund_pending',
  partial: 'partial_refund',
  full: 'refunded',
};

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

// The one screen with room for both halves, so it shows both. Under the
// single payment_status field a deposit-only booking being refunded read
// just "Reembolso pendiente" and the deposit half was gone; the server keeps
// the two apart now (collection_status / refund_status) and so does this row. The refund half is
// omitted entirely when there is nothing to give back, which is every
// booking that never got refunded.
function PaymentStatusRow({ booking }: { booking: Booking }) {
  return (
    <InfoRow label={t.bookings.paymentStatus}>
      <span className="flex flex-wrap items-center gap-x-1.5">
        <PaymentStatusText status={booking.collection_status} />
        {booking.refund_status !== 'none' && (
          <>
            <span aria-hidden="true" className="text-text-disabled">
              ·
            </span>
            <PaymentStatusText status={REFUND_DISPLAY[booking.refund_status]} />
          </>
        )}
      </span>
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
        <BookingStatusText status={booking.status} />
      </InfoRow>
      <PaymentStatusRow booking={booking} />
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
      <InfoRow label={t.bookings.date}>
        <span className="score-text font-medium">{formatDateShort(booking.date)}</span>
      </InfoRow>
      <InfoRow label={t.bookings.time}>
        <span className="score-text font-medium">{formatHourRange(booking.starts_at, booking.ends_at)}</span>
      </InfoRow>
      <InfoRow label={t.bookings.duration}>{booking.duration_minutes} min</InfoRow>
      <InfoRow label={t.bookings.court}>
        <span className="truncate font-medium">{booking.court_name}</span>
      </InfoRow>
      {clientRow}
    </div>
  );
}
