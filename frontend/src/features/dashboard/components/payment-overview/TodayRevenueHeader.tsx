import { formatPrice } from '@/shared/lib/utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { DayMoneyTotals } from '@/shared/types/api.types';

const t = ES_AR;

interface TodayRevenueHeaderProps {
  totals: DayMoneyTotals;
}

/**
 * "Ingresos de hoy": everything collected today — bookings (online and at the
 * counter, every method) plus what the till took in (bar sales and other
 * manual income), read from `today_money`. The till itself (open/closed,
 * expected cash) lives on the Caja screen.
 *
 * No vs-yesterday badge: the backend's yesterday figure is bookings only, so
 * comparing it against this total would compare two different quantities.
 */
export function TodayRevenueHeader({ totals }: TodayRevenueHeaderProps) {
  const parts = [
    { label: t.dashboard.todayIncomeBookings, value: totals.bookings },
    { label: t.dashboard.todayIncomeBar, value: totals.bar_sales },
    { label: t.dashboard.todayIncomeOther, value: totals.other_income },
  ];

  return (
    <div className="mb-4">
      <h3 className="text-text-primary mb-1 text-sm font-semibold">{t.dashboard.todayRevenue}</h3>
      <p className="score-text text-text-primary text-lg font-bold">{formatPrice(totals.total_income)}</p>
      <p className="text-text-tertiary mt-1 flex flex-wrap gap-x-2 text-xs">
        {parts.map((part, index) => (
          <span key={part.label} className="whitespace-nowrap">
            {index > 0 && <span aria-hidden="true">· </span>}
            {part.label} <span className="score-text text-text-secondary">{formatPrice(part.value)}</span>
          </span>
        ))}
      </p>
    </div>
  );
}
