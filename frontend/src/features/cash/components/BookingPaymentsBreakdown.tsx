import { Panel } from '@/shared/components/common/Panel';
import { Badge } from '@/shared/components/ui/badge';
import { ES_AR } from '@/shared/i18n/es_AR';
import { formatPrice } from '@/shared/lib/utils';
import type { CashBookingPaymentTotal } from '@/shared/types/api.types';

const t = ES_AR;

/**
 * Booking payments collected during the session's window, per method —
 * `mercadopago` is included and clearly marked informational: only cash
 * booking payments feed `expected_cash` (see `CashSessionSummary.expected_cash`'s
 * own doc comment in openapi.yaml), the rest are shown for the full picture.
 */
export function BookingPaymentsBreakdown({ bookingPayments }: { bookingPayments: CashBookingPaymentTotal[] }) {
  if (bookingPayments.length === 0) return null;
  const sorted = [...bookingPayments].sort((a, b) => b.amount - a.amount);

  return (
    <Panel size="sm" className="space-y-2">
      <p className="text-text-tertiary text-xs font-semibold tracking-wider uppercase">{t.cash.bookingPaymentsTitle}</p>
      {sorted.map((entry) => (
        <div key={entry.method} className="flex items-center justify-between text-sm">
          <span className="text-text-secondary flex items-center gap-1.5">
            {t.bookings.paymentMethods[entry.method]}
            {/* Only cash booking payments feed `expected_cash` — every other
                method here is shown for the full picture, not the till. */}
            {entry.method !== 'cash' && (
              <Badge variant="outline" className="text-micro">
                {t.cash.informational}
              </Badge>
            )}
          </span>
          <span className="font-medium">{formatPrice(entry.amount)}</span>
        </div>
      ))}
      <p className="text-text-tertiary text-micro pt-1">{t.cash.bookingPaymentsOnlineNote}</p>
    </Panel>
  );
}
