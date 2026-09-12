import { cn } from '@/shared/lib/utils';
import { formatPrice } from '@/shared/lib/utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { CollectionStatus, PaymentSummary } from '@/shared/types/api.types';

const t = ES_AR;

// `by_status` is keyed by the booking's collection_status (backend
// the server split the old payment_status enum in two, and GetPaymentSummary groups
// by the money-in axis). Three keys, exhaustively; the `?? key` fallback below
// stays as the guard for a server ahead of this build.
interface StatusDisplay {
  label: string;
  color: string;
}

const STATUS_CONFIG: Record<CollectionStatus, StatusDisplay> = {
  fully_paid: { label: t.dashboard.paymentStatusLabels.fully_paid, color: 'bg-success-text' },
  deposit_paid: { label: t.dashboard.paymentStatusLabels.deposit_paid, color: 'bg-info-text' },
  unpaid: { label: t.dashboard.paymentStatusLabels.unpaid, color: 'bg-warning-text' },
};

export function PaymentStatusBreakdown({
  statusEntries,
}: {
  statusEntries: [string, PaymentSummary['by_status'][string]][];
}) {
  return (
    <div className="space-y-2">
      <p className="text-text-tertiary text-sm font-semibold tracking-wider uppercase">{t.dashboard.paymentStatus}</p>
      {statusEntries.length === 0 && <p className="text-text-tertiary text-sm">{t.dashboard.noBookingsToday}</p>}
      {statusEntries.map(([key, val]) => {
        // Widened on the way in, not on the way out: STATUS_CONFIG stays an
        // exhaustive Record so a new collection status fails to compile here,
        // while the lookup itself admits that `key` is whatever the server
        // sent and may match nothing.
        const cfg: StatusDisplay | undefined = (STATUS_CONFIG as Partial<Record<string, StatusDisplay>>)[key];
        const { label, color } = cfg ?? { label: key, color: 'bg-text-disabled' };
        return (
          <div key={key} className="flex items-center justify-between text-sm">
            <div className="flex items-center gap-1.5">
              <div className={cn('size-2 rounded-full', color)} />
              <span className="text-text-secondary">{label}</span>
              <span className="text-text-tertiary">({val.count})</span>
            </div>
            {/* A status only ever has money against it when a real payment
                was collected today (GetPaymentSummary sums payments.amount,
                joined and filtered the same way as the by-method total below
                it, so the two reconcile) — showing "$0" next to a real
                figure read as "$0 pending" instead of "nothing to show
                here". */}
            {val.total > 0 && (
              <span className="score-text text-text-primary font-medium">{formatPrice(val.total)}</span>
            )}
          </div>
        );
      })}
    </div>
  );
}
