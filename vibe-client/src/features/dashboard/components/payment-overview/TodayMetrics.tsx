import { ES_AR } from '@/shared/i18n/es_AR';
import { ComparisonBadge } from '../stats-cards/ComparisonBadge';

const t = ES_AR;

interface TodayMetricsProps {
  todayBookings: number;
  yesterdayBookings: number;
  occupancyRate: number;
}

export function TodayMetrics({ todayBookings, yesterdayBookings, occupancyRate }: TodayMetricsProps) {
  return (
    <div className="mb-4 flex flex-col gap-4">
      <div>
        <div className="flex items-center gap-2">
          <span className="text-sm font-medium text-text-tertiary">{t.dashboard.bookingsToday}</span>
          <ComparisonBadge current={todayBookings} previous={yesterdayBookings} versus={t.dashboard.versusYesterday} />
        </div>
        <p className="score-text mt-1 text-lg font-bold text-text-primary">{todayBookings}</p>
      </div>
      <div>
        <span className="text-sm font-medium text-text-tertiary">{t.dashboard.occupancyRate}</span>
        <p className="score-text mt-1 text-lg font-bold text-text-primary">{occupancyRate}%</p>
        <div
          className="mt-2 h-1 w-full overflow-hidden rounded-full bg-bg-base"
          role="progressbar"
          aria-valuenow={occupancyRate}
          aria-valuemin={0}
          aria-valuemax={100}
          aria-label={`${t.dashboard.occupancyRate}: ${String(occupancyRate)}%`}
        >
          <div
            className="h-1 rounded-full bg-primary-500 transition-[width] duration-700 ease-out"
            style={{ width: `${String(Math.min(occupancyRate, 100))}%` }}
          />
        </div>
      </div>
    </div>
  );
}
