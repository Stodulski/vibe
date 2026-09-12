import { useMemo, useState } from 'react';
import { ES_AR } from '@/shared/i18n/es_AR';
import { formatPrice, formatDateShort } from '@/shared/lib/utils';
import { useRevenueChart } from '../hooks/useRevenueChart';
import { PeriodToggle, type RevenuePeriod } from './revenue-chart/PeriodToggle';
import { ChartBody } from './revenue-chart/ChartBody';
import type { ChartBodyState } from './revenue-chart/chartBodyState';

const t = ES_AR;

interface RevenueChartProps {
  complexId: string;
}

export function RevenueChart({ complexId }: RevenueChartProps) {
  const [period, setPeriod] = useState<RevenuePeriod>('week');
  const { data: revenue, isLoading, isError, refetch } = useRevenueChart(complexId, period);

  const chartData = useMemo(
    () =>
      revenue?.map((d) => ({
        date: formatDateShort(d.date + 'T12:00:00').slice(0, 5),
        amount: d.amount,
      })) ?? [],
    [revenue],
  );

  const total = useMemo(() => chartData.reduce((sum, d) => sum + d.amount, 0), [chartData]);

  const state: ChartBodyState = isLoading
    ? { status: 'loading' }
    : isError
      ? {
          status: 'error',
          onRetry: () => {
            void refetch();
          },
        }
      : { status: 'ready', chartData };

  return (
    <section
      className="border-border-subtle bg-bg-subtle flex h-72 flex-col rounded-2xl border p-4 sm:p-5 xl:h-full"
      aria-label={t.dashboard.revenue}
    >
      <div className="mb-4 flex items-start justify-between gap-3 sm:mb-5">
        <div className="min-w-0">
          <h3 className="text-text-primary text-sm font-semibold whitespace-nowrap">{t.dashboard.revenue}</h3>
          {state.status !== 'loading' && (
            <p className="score-text text-text-primary mt-0.5 text-lg font-bold whitespace-nowrap sm:text-xl">
              {formatPrice(total)}
            </p>
          )}
        </div>
        <PeriodToggle period={period} onChange={setPeriod} />
      </div>

      <div className="min-h-0 flex-1">
        <ChartBody state={state} period={period} />
      </div>
    </section>
  );
}

export default RevenueChart;
