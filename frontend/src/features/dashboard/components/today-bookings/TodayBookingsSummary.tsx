import { ES_AR } from '@/shared/i18n/es_AR';
import { ComparisonBadge } from '../stats-cards/ComparisonBadge';

const t = ES_AR;

interface TodayBookingsSummaryProps {
  todayBookingsCount: number;
  yesterdayBookingsCount: number;
  occupancyRate: number;
}

/**
 * "Reservas hoy" (every confirmed booking today, not just the ones still
 * upcoming) and the occupancy bar, moved in from PaymentOverview's
 * TodayMetrics (odd/tasks/dashboard-today-card.md).
 */
export function TodayBookingsSummary({
  todayBookingsCount,
  yesterdayBookingsCount,
  occupancyRate,
}: TodayBookingsSummaryProps) {
  return (
    <div className="mb-4 flex flex-col gap-4 sm:flex-row sm:gap-6">
      <div>
        <div className="flex items-center gap-2">
          <span className="text-text-tertiary text-sm font-medium">{t.dashboard.bookingsToday}</span>
          <ComparisonBadge
            current={todayBookingsCount}
            previous={yesterdayBookingsCount}
            versus={t.dashboard.versusYesterday}
          />
        </div>
        <p className="score-text text-text-primary mt-1 text-lg font-bold">{todayBookingsCount}</p>
      </div>
      <div className="flex-1">
        <span className="text-text-tertiary text-sm font-medium">{t.dashboard.occupancyRate}</span>
        <p className="score-text text-text-primary mt-1 text-lg font-bold">{occupancyRate}%</p>
        <div
          className="bg-bg-base mt-2 h-1 w-full overflow-hidden rounded-full"
          role="progressbar"
          aria-valuenow={occupancyRate}
          aria-valuemin={0}
          aria-valuemax={100}
          aria-label={`${t.dashboard.occupancyRate}: ${String(occupancyRate)}%`}
        >
          <div
            className="bg-primary-500 h-1 rounded-full transition-[width] duration-700 ease-out"
            style={{ width: `${String(Math.min(occupancyRate, 100))}%` }}
          />
        </div>
      </div>
    </div>
  );
}
