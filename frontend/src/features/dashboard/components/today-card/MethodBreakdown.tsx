import { ES_AR } from '@/shared/i18n/es_AR';
import { formatPrice } from '@/shared/lib/utils';
import type { DayMoneyTotals } from '@/shared/types/api.types';

const t = ES_AR;

// Every method a stored payment can carry, not only the counter ones —
// mercadopago shows up here too, the same map PaymentMethodBreakdown used.
const METHOD_CONFIG: Record<string, string> = t.bookings.paymentMethods;

export function MethodBreakdown({ byMethod }: { byMethod: DayMoneyTotals['by_method'] }) {
  const entries = Object.entries(byMethod).sort((a, b) => b[1] - a[1]);
  if (entries.length === 0) return null;

  return (
    <div className="mt-4 space-y-1.5">
      <p className="text-text-tertiary text-sm font-semibold tracking-wider uppercase">
        {t.dashboard.paymentMethodLabel}
      </p>
      {entries.map(([key, amount]) => (
        <div key={key} className="flex items-center justify-between text-sm">
          <span className="text-text-secondary">{METHOD_CONFIG[key] ?? key}</span>
          <span className="score-text text-text-primary font-medium">{formatPrice(amount)}</span>
        </div>
      ))}
    </div>
  );
}
