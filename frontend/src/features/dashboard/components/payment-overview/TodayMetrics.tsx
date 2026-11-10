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
    <div className="flex flex-col gap-4">
      <div>
        <div className="flex items-center gap-2">
          <span className="text-text-inverse text-sm font-medium">{t.dashboard.bookingsToday}</span>
          <ComparisonBadge
            current={todayBookings}
            previous={yesterdayBookings}
            versus={t.dashboard.versusYesterday}
            className="bg-bg-base"
          />
        </div>
        <p className="score-text text-text-inverse mt-1 text-lg font-bold">{todayBookings}</p>
      </div>
      <div>
        <span className="text-text-inverse text-sm font-medium">{t.dashboard.occupancyRate}</span>
        <p className="score-text text-text-inverse mt-1 text-lg font-bold">{occupancyRate}%</p>
        <div
          className="bg-text-inverse/20 mt-2 h-1 w-full overflow-hidden rounded-full"
          role="progressbar"
          aria-valuenow={occupancyRate}
          aria-valuemin={0}
          aria-valuemax={100}
          aria-label={`${t.dashboard.occupancyRate}: ${String(occupancyRate)}%`}
        >
          <div
            className="bg-text-inverse h-1 rounded-full transition-[width] duration-700 ease-out"
            style={{ width: `${String(Math.min(occupancyRate, 100))}%` }}
          />
        </div>
      </div>
    </div>
  );
}
