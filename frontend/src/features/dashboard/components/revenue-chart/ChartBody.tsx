import { AlertTriangle } from 'lucide-react';
import { ES_AR } from '@/shared/i18n/es_AR';
import { Skeleton } from '@/shared/components/ui/skeleton';
import { Button } from '@/shared/components/ui/button';
import { CanvasAreaChart } from './CanvasAreaChart';
import type { ChartBodyState } from './chartBodyState';
import type { RevenuePeriod } from './PeriodToggle';

const t = ES_AR;

interface ChartBodyProps {
  state: ChartBodyState;
  period: RevenuePeriod;
}

export function ChartBody({ state, period }: ChartBodyProps) {
  if (state.status === 'loading') {
    return (
      <div role="status" aria-busy="true" aria-label={t.common.loading} className="h-full min-h-[200px]">
        <Skeleton className="h-full w-full rounded-xl" />
      </div>
    );
  }

  if (state.status === 'error') {
    return (
      <div className="flex h-full min-h-[200px] flex-col items-center justify-center gap-2 text-center">
        <AlertTriangle className="text-error-icon size-5" aria-hidden="true" />
        <p className="text-text-tertiary text-sm">{t.common.loadError}</p>
        <Button variant="outline" size="sm" onClick={state.onRetry}>
          {t.layout.retry}
        </Button>
      </div>
    );
  }

  if (state.chartData.length === 0) {
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
      <CanvasAreaChart data={state.chartData} />
    </div>
  );
}
