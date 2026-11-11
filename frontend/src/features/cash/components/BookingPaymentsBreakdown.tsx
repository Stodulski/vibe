import { ES_AR } from '@/shared/i18n/es_AR';
import { formatPrice } from '@/shared/lib/utils';
import type { CashBookingPaymentTotal } from '@/shared/types/api.types';

const t = ES_AR;

/**
 * Booking payments collected during the session by any method other than
 * cash (transfer, card, MercadoPago). Cash booking payments are a term of
 * expected cash and live in `ExpectedCashCard`; these never touch the drawer
 * and are shown only for the full picture.
 */
export function BookingPaymentsBreakdown({ bookingPayments }: { bookingPayments: CashBookingPaymentTotal[] }) {
  const others = bookingPayments.filter((p) => p.method !== 'cash').sort((a, b) => b.amount - a.amount);
  if (others.length === 0) return null;

  return (
    <section className="space-y-2">
      <div>
        <h2 className="text-text-primary text-base font-semibold">{t.cash.otherMethodPaymentsTitle}</h2>
        <p className="text-text-tertiary text-sm">{t.cash.otherMethodPaymentsNote}</p>
      </div>
      <dl className="grid grid-cols-2 gap-x-4 gap-y-3 sm:grid-cols-4">
        {others.map((entry) => (
          <div key={entry.method}>
            <dt className="text-text-secondary text-xs">{t.bookings.paymentMethods[entry.method]}</dt>
            <dd className="text-text-primary font-semibold tabular-nums">{formatPrice(entry.amount)}</dd>
          </div>
        ))}
      </dl>
    </section>
  );
}
