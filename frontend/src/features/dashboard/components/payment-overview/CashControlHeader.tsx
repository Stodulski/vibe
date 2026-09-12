import { formatPrice } from '@/shared/lib/utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import { ComparisonBadge } from '../stats-cards/ComparisonBadge';

const t = ES_AR;

interface CashControlHeaderProps {
  todayRevenue: number;
  yesterdayRevenue: number;
}

export function CashControlHeader({ todayRevenue, yesterdayRevenue }: CashControlHeaderProps) {
  return (
    <>
      <div className="mb-1 flex items-center gap-2">
        <h3 className="text-text-primary text-sm font-semibold">{t.dashboard.cashControl}</h3>
        <ComparisonBadge current={todayRevenue} previous={yesterdayRevenue} versus={t.dashboard.versusYesterday} />
      </div>
      <p className="score-text text-text-primary mb-4 text-lg font-bold">{formatPrice(todayRevenue)}</p>
    </>
  );
}
