import { formatPrice } from '@/shared/lib/utils';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

const METHOD_CONFIG: Record<string, string> = {
  cash: t.bookings.paymentMethods.cash,
  transfer: t.bookings.paymentMethods.transfer,
  mercadopago: t.bookings.paymentMethods.mercadopago,
};

export function PaymentMethodBreakdown({ methodEntries }: { methodEntries: [string, number][] }) {
  if (methodEntries.length === 0) return null;

  return (
    <div className="mt-4 space-y-1.5">
      <p className="text-sm font-semibold uppercase tracking-wider text-text-tertiary">
        {t.dashboard.paymentMethodLabel}
      </p>
      {methodEntries.map(([key, amount]) => (
        <div key={key} className="flex items-center justify-between text-sm">
          <span className="text-text-secondary">{METHOD_CONFIG[key] ?? key}</span>
          <span className="score-text font-medium text-text-primary">{formatPrice(amount)}</span>
        </div>
      ))}
    </div>
  );
}
