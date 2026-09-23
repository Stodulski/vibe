import { formatPrice } from '@/shared/lib/utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import { ComparisonBadge } from '../stats-cards/ComparisonBadge';

const t = ES_AR;

interface TodayRevenueHeaderProps {
  todayRevenue: number;
  yesterdayRevenue: number;
}

/**
 * Renamed from `CashControlHeader` (pos-cashbox T6): this card totals
 * today's booking revenue across every method, which is not a cash control —
 * the dashboard's actual cash control is the new `CashboxPanel` (open/closed
 * till, expected cash), fed by `useCashSession`. "Ingresos de hoy" says what
 * this figure actually is.
 */
export function TodayRevenueHeader({ todayRevenue, yesterdayRevenue }: TodayRevenueHeaderProps) {
  return (
    <>
      <div className="mb-1 flex items-center gap-2">
        <h3 className="text-text-primary text-sm font-semibold">{t.dashboard.todayRevenue}</h3>
        <ComparisonBadge current={todayRevenue} previous={yesterdayRevenue} versus={t.dashboard.versusYesterday} />
      </div>
      <p className="score-text text-text-primary mb-4 text-lg font-bold">{formatPrice(todayRevenue)}</p>
    </>
  );
}
