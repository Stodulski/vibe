import { lazy, Suspense } from 'react';
import { TodayBookings } from '@/features/dashboard';
import { TodayCard } from '../../components/TodayCard';
import { LowStockAlert } from '../../components/LowStockAlert';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { DashboardStats, ClientInsights } from '@/shared/types/api.types';

const t = ES_AR;

// Lazy load below-fold components to reduce initial bundle.
const RevenueChart = lazy(() => import('@/features/dashboard/components/RevenueChart'));
const ClientInsightsCard = lazy(() => import('@/features/dashboard/components/ClientInsightsCard'));
const OccupancyHeatmap = lazy(() => import('@/features/dashboard/components/OccupancyHeatmap'));

function ChartSkeleton() {
  return <div className="bg-bg-subtle h-80 animate-pulse rounded-xl" />;
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
  // "unpaid" never carries a collected amount (see PaymentStatusBreakdown's
  // comment on GetPaymentSummary's SQL) and self-resolves within the 15-
  // minute payment-expiry window — not something to show in cash control.
  const statusEntries = Object.entries(stats.payment_summary.by_status)
    .filter(([key]) => key !== 'unpaid')
    .sort((a, b) => b[1].total - a[1].total);

  return (
    <div className="flex flex-col gap-4 md:gap-6">
      {/* 1. The "Hoy" card (odd/tasks/dashboard-today-card.md): till status +
          expected cash next to the day's income, replacing the old separate
          Caja and "Ingresos de hoy" cards, which never reconciled with each
          other. Full width on every breakpoint (owner request), then the
          low-stock alert, which hides itself when there is nothing to flag. */}
      <TodayCard complexId={selectedComplexId} stats={stats} />
      <LowStockAlert complexId={selectedComplexId} />

      {/* 1b. Upcoming bookings, full width — absorbed "Reservas hoy",
          occupancy and the payment-status breakdown from the old
          PaymentOverview card. */}
      <TodayBookings
        bookings={stats.upcoming_bookings}
        complexId={selectedComplexId}
        todayBookingsCount={stats.today_bookings}
        yesterdayBookingsCount={stats.yesterday_bookings}
        occupancyRate={stats.occupancy_rate}
        statusEntries={statusEntries}
      />

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
