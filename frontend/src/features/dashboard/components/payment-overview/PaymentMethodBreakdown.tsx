import { formatPrice } from '@/shared/lib/utils';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

// Every method a stored payment can carry, not only the counter ones: a
// dashboard covering past periods still shows mercadopago rows.
const METHOD_CONFIG: Record<string, string> = t.bookings.paymentMethods;

export function PaymentMethodBreakdown({ methodEntries }: { methodEntries: [string, number][] }) {
  if (methodEntries.length === 0) return null;

  return (
    <div className="mt-4 space-y-1.5">
      <p className="text-text-tertiary text-sm font-semibold tracking-wider uppercase">
        {t.dashboard.paymentMethodLabel}
      </p>
      {methodEntries.map(([key, amount]) => (
        <div key={key} className="flex items-center justify-between text-sm">
          <span className="text-text-secondary">{METHOD_CONFIG[key] ?? key}</span>
          <span className="score-text text-text-primary font-medium">{formatPrice(amount)}</span>
        </div>
      ))}
    </div>
  );
}
