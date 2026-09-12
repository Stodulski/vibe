import { useMemo, useState } from 'react';
import { ES_AR } from '@/shared/i18n/es_AR';
import { formatPrice, formatDateShort } from '@/shared/lib/utils';
import { useRevenueChart } from '../hooks/useRevenueChart';
import { PeriodToggle, type RevenuePeriod } from './revenue-chart/PeriodToggle';
import { ChartBody } from './revenue-chart/ChartBody';

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

  return (
    <section
      className="border-border-subtle bg-bg-subtle flex h-72 flex-col rounded-2xl border p-4 sm:p-5 xl:h-full"
      aria-label={t.dashboard.revenue}
    >
      <div className="mb-4 flex items-start justify-between gap-3 sm:mb-5">
        <div className="min-w-0">
          <h3 className="text-text-primary text-sm font-semibold whitespace-nowrap">{t.dashboard.revenue}</h3>
          {!isLoading && (
            <p className="score-text text-text-primary mt-0.5 text-lg font-bold whitespace-nowrap sm:text-xl">
              {formatPrice(total)}
            </p>
          )}
        </div>
        <PeriodToggle period={period} onChange={setPeriod} />
      </div>

      <div className="min-h-0 flex-1">
        <ChartBody
          isLoading={isLoading}
          isError={isError}
          onRetry={() => {
            void refetch();
          }}
          chartData={chartData}
          period={period}
        />
      </div>
    </section>
  );
}

export default RevenueChart;
