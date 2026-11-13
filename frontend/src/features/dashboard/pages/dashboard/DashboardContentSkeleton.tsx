import { Skeleton } from '@/shared/components/ui/skeleton';
import { ES_AR } from '@/shared/i18n/es_AR';
import { OccupancyHeatmapSkeleton } from '../../components/OccupancyHeatmapSkeleton';
import { PaymentOverviewSkeleton } from '../../components/PaymentOverviewSkeleton';
import { TodayBookingsSkeleton } from '../../components/TodayBookingsSkeleton';
import { TrendsRowSkeleton } from '../../components/TrendsRowSkeleton';
import { ProfileChecklistSkeleton } from '../../components/profile-checklist/ProfileChecklistSkeleton';
import { DASHBOARD_STACK_CLASS, OPERATIONS_GRID_CLASS, TRENDS_SECTION_CLASS } from '../../components/dashboardLayout';

const t = ES_AR;

/** Mirrors `DashboardContent`'s desktop-only trends block (heading bar, then the cards). */
function TrendsSkeleton() {
  return (
    <div className={TRENDS_SECTION_CLASS}>
      <Skeleton className="h-4 w-20 rounded-lg" />
      <TrendsRowSkeleton />
      <OccupancyHeatmapSkeleton />
    </div>
  );
}

/**
 * Matches `DashboardContent` in the order of what loads: the unboxed profile
 * checklist (low-stock renders nothing while loading, so it has no
 * placeholder), the payment card next to the today-bookings panel, then the
 * `hidden md:flex` trends block.
 */
export function DashboardContentSkeleton() {
  return (
    <div role="status" aria-label={t.common.loading} className={DASHBOARD_STACK_CLASS}>
      <ProfileChecklistSkeleton />
      <div className={OPERATIONS_GRID_CLASS}>
        <PaymentOverviewSkeleton />
        <TodayBookingsSkeleton />
      </div>
      <TrendsSkeleton />
    </div>
  );
}
