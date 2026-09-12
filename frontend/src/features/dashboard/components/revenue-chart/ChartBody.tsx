import { AlertTriangle } from 'lucide-react';
import { ES_AR } from '@/shared/i18n/es_AR';
import { Skeleton } from '@/shared/components/ui/skeleton';
import { Button } from '@/shared/components/ui/button';
import { CanvasAreaChart } from './CanvasAreaChart';
import type { ChartDatum } from './chart-draw';
import type { RevenuePeriod } from './PeriodToggle';

const t = ES_AR;

interface ChartBodyProps {
  isLoading: boolean;
  isError: boolean;
  onRetry: () => void;
  chartData: ChartDatum[];
  period: RevenuePeriod;
}

export function ChartBody({ isLoading, isError, onRetry, chartData, period }: ChartBodyProps) {
  if (isLoading) {
    return <Skeleton className="h-full min-h-[200px] w-full rounded-xl" />;
  }

  if (isError) {
    return (
      <div className="flex h-full min-h-[200px] flex-col items-center justify-center gap-2 text-center">
        <AlertTriangle className="text-error-icon size-5" aria-hidden="true" />
        <p className="text-text-tertiary text-sm">{t.common.loadError}</p>
        <Button variant="outline" size="sm" onClick={onRetry}>
          {t.layout.retry}
        </Button>
      </div>
    );
  }

  if (chartData.length === 0) {
    return (
      <div className="text-text-tertiary flex h-full min-h-[200px] items-center justify-center text-sm">
        {t.common.noResults}
      </div>
    );
  }

  return (
    <div
      role="img"
      aria-label={period === 'week' ? t.dashboard.revenueChartWeekLabel : t.dashboard.revenueChartMonthLabel}
      className="h-full min-h-[200px]"
    >
      <CanvasAreaChart data={chartData} />
    </div>
  );
}
