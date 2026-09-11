import { lazy, Suspense } from 'react';
import { TodayBookings, PaymentOverview } from '@/features/dashboard';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { DashboardStats, ClientInsights } from '@/shared/types/api.types';

const t = ES_AR;

// Lazy load below-fold components to reduce initial bundle.
const RevenueChart = lazy(() => import('@/features/dashboard/components/RevenueChart'));
const ClientInsightsCard = lazy(() => import('@/features/dashboard/components/ClientInsightsCard'));
const OccupancyHeatmap = lazy(() => import('@/features/dashboard/components/OccupancyHeatmap'));

function ChartSkeleton() {
  return <div className="h-80 animate-pulse rounded-xl bg-bg-subtle" />;
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
      {/* 1. Upcoming bookings + Payment overview (folds in today's key metrics) — "Operaciones del día" */}
      <div className="grid grid-cols-1 gap-4 md:gap-6 lg:grid-cols-2">
        <PaymentOverview stats={stats} />
        <TodayBookings bookings={stats.upcoming_bookings} complexId={selectedComplexId} />
      </div>

      {/* 2. Trends — revenue chart, client insights, occupancy heatmap
          (lazy, desktop only). Grouped under one heading and separated from
          the operational block above so the squint test reads two clear
          zones (Practical UI ch.4 p.164-178, p.187). */}
      <div className="mt-8 hidden md:flex md:flex-col md:gap-4 lg:gap-6">
        <h2 className="text-sm font-semibold text-text-tertiary">{t.dashboard.trends}</h2>

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
