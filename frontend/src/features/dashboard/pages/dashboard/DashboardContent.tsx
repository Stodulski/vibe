import { lazy, Suspense } from 'react';
import { TodayBookings, PaymentOverview } from '@/features/dashboard';
import { LowStockAlert } from '../../components/LowStockAlert';
import { OccupancyHeatmapSkeleton } from '../../components/OccupancyHeatmapSkeleton';
import { TrendsRowSkeleton } from '../../components/TrendsRowSkeleton';
import {
  DASHBOARD_STACK_CLASS,
  OPERATIONS_GRID_CLASS,
  TRENDS_MAIN_CELL_CLASS,
  TRENDS_ROW_CLASS,
  TRENDS_SECTION_CLASS,
  TRENDS_SIDE_CELL_CLASS,
} from '../../components/dashboardLayout';
import { ProfileChecklist } from '../../components/profile-checklist/ProfileChecklist';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { Complex, DashboardStats, ClientInsights } from '@/shared/types/api.types';

const t = ES_AR;

// Lazy load below-fold components to reduce initial bundle.
const RevenueChart = lazy(() => import('@/features/dashboard/components/RevenueChart'));
const ClientInsightsCard = lazy(() => import('@/features/dashboard/components/ClientInsightsCard'));
const OccupancyHeatmap = lazy(() => import('@/features/dashboard/components/OccupancyHeatmap'));

interface DashboardContentProps {
  stats: DashboardStats;
  selectedComplexId: string;
  complex: Complex | null;
  clientInsights: ClientInsights | undefined;
  clientsLoading: boolean;
  clientsError: boolean;
  onRetryClients: () => void;
}

export function DashboardContent({
  stats,
  selectedComplexId,
  complex,
  clientInsights,
  clientsLoading,
  clientsError,
  onRetryClients,
}: DashboardContentProps) {
  return (
    <div className={DASHBOARD_STACK_CLASS}>
      {/* 1. Low-stock alert (pos-cashbox T6); hides itself when there is
          nothing to flag. The till lives on the Caja screen, not here (owner
          request, 2026-09-25). */}
      <LowStockAlert complexId={selectedComplexId} />

      {/* 1c. Public-profile checklist: what the storefront still lacks. Sits
          below the critical alert and disappears once the profile is complete. */}
      {complex && <ProfileChecklist complex={complex} />}

      {/* 1b. Upcoming bookings + Payment overview (folds in today's key metrics) — "Operaciones del día" */}
      <div className={OPERATIONS_GRID_CLASS}>
        <PaymentOverview stats={stats} />
        <TodayBookings bookings={stats.upcoming_bookings} complexId={selectedComplexId} />
      </div>

      {/* 2. Trends — revenue chart, client insights, occupancy heatmap
          (lazy, desktop only). Grouped under one heading and separated from
          the operational block above so the squint test reads two clear
          zones (Practical UI ch.4 p.164-178, p.187). */}
      <div className={TRENDS_SECTION_CLASS}>
        <h2 className="text-text-tertiary text-sm font-semibold">{t.dashboard.trends}</h2>

        <div className="offscreen-section">
          <Suspense fallback={<TrendsRowSkeleton />}>
            <div className={TRENDS_ROW_CLASS}>
              <div className={TRENDS_MAIN_CELL_CLASS}>
                <RevenueChart complexId={selectedComplexId} />
              </div>
              <div className={TRENDS_SIDE_CELL_CLASS}>
                <ClientInsightsCard
                  data={clientInsights}
                  isLoading={clientsLoading}
                  isError={clientsError}
                  onRetry={onRetryClients}
                  complexId={selectedComplexId}
                />
              </div>
            </div>
          </Suspense>
        </div>

        <div className="offscreen-section">
          <Suspense fallback={<OccupancyHeatmapSkeleton />}>
            <OccupancyHeatmap complexId={selectedComplexId} />
          </Suspense>
        </div>
      </div>
    </div>
  );
}
