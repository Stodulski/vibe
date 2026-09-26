import { lazy, Suspense } from 'react';
import { TodayBookings, PaymentOverview } from '@/features/dashboard';
import { LowStockAlert } from '../../components/LowStockAlert';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { DashboardStats, ClientInsights } from '@/shared/types/api.types';

const t = ES_AR;

// Lazy load below-fold components to reduce initial bundle.
const RevenueChart = lazy(() => import('@/features/dashboard/components/RevenueChart'));
const ClientInsightsCard = lazy(() => import('@/features/dashboard/components/ClientInsightsCard'));
const OccupancyHeatmap = lazy(() => import('@/features/dashboard/components/OccupancyHeatmap'));

/** Matches `RevenueChart`/`ClientInsightsCard`/`OccupancyHeatmap`'s own `rounded-2xl border` — this only ever renders inside the `hidden md:flex` trends block, where those are always boxed. */
function ChartSkeleton() {
  return <div className="border-border-subtle bg-bg-subtle h-80 animate-pulse rounded-2xl border" />;
}

interface DashboardContentProps {
  stats: DashboardStats;
  selectedComplexId: string;
  clientInsights: ClientInsights | undefined;
  clientsLoading: boolean;
  clientsError: boolean;
  onRetryClients: () => void;
}

export function DashboardContent({
  stats,
  selectedComplexId,
  clientInsights,
  clientsLoading,
  clientsError,
  onRetryClients,
}: DashboardContentProps) {
  return (
    <div className="flex flex-col gap-4 md:gap-6">
      {/* 1. Low-stock alert (pos-cashbox T6); hides itself when there is
          nothing to flag. The till lives on the Caja screen, not here (owner
          request, 2026-09-25). */}
      <LowStockAlert complexId={selectedComplexId} />

      {/* 1b. Upcoming bookings + Payment overview (folds in today's key metrics) — "Operaciones del día" */}
      <div className="grid grid-cols-1 gap-4 md:gap-6 lg:grid-cols-2">
        <PaymentOverview stats={stats} />
        <TodayBookings bookings={stats.upcoming_bookings} complexId={selectedComplexId} />
      </div>

      {/* 2. Trends — revenue chart, client insights, occupancy heatmap
          (lazy, desktop only). Grouped under one heading and separated from
          the operational block above so the squint test reads two clear
          zones (Practical UI ch.4 p.164-178, p.187). */}
      <div className="mt-8 hidden md:flex md:flex-col md:gap-4 lg:gap-6">
        <h2 className="text-text-tertiary text-sm font-semibold">{t.dashboard.trends}</h2>

        <div className="offscreen-section">
          <Suspense fallback={<ChartSkeleton />}>
            <div className="grid grid-cols-1 gap-4 md:gap-6 xl:grid-cols-5">
              <div className="xl:col-span-3">
                <RevenueChart complexId={selectedComplexId} />
              </div>
              <div className="xl:col-span-2">
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
          <Suspense fallback={<ChartSkeleton />}>
            <OccupancyHeatmap complexId={selectedComplexId} />
          </Suspense>
        </div>
      </div>
    </div>
  );
}
