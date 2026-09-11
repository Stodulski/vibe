import type { DashboardStats } from '@/shared/types/api.types';
import { CashControlHeader } from './payment-overview/CashControlHeader';
import { TodayMetrics } from './payment-overview/TodayMetrics';
import { PaymentStatusBreakdown } from './payment-overview/PaymentStatusBreakdown';
import { PaymentMethodBreakdown } from './payment-overview/PaymentMethodBreakdown';
import { Panel } from '@/shared/components/common/Panel';

interface PaymentOverviewProps {
  stats: DashboardStats;
}

/**
 * Folds in what used to be the separate top-of-page stat cards (revenue,
 * bookings, occupancy) — they were duplicating "today's revenue" against
 * this card's own total, and gave this card too little content to fill the
 * height its taller sibling (TodayBookings) sets.
 */
export function PaymentOverview({ stats }: PaymentOverviewProps) {
  const {
    payment_summary: summary,
    today_revenue,
    yesterday_revenue,
    today_bookings,
    yesterday_bookings,
    occupancy_rate,
  } = stats;

  // "unpaid" never carries a collected amount (see PaymentStatusBreakdown's
  // comment on GetPaymentSummary's SQL) and self-resolves within the 15-
  // minute payment-expiry window — not something to show in cash control.
  const statusEntries = Object.entries(summary.by_status)
    .filter(([key]) => key !== 'unpaid')
    .sort((a, b) => b[1].total - a[1].total);
  const methodEntries = Object.entries(summary.by_method).sort((a, b) => b[1] - a[1]);

  return (
    <Panel as="section" size="sm" className="flex h-full flex-col p-4">
      <CashControlHeader todayRevenue={today_revenue} yesterdayRevenue={yesterday_revenue} />
      <TodayMetrics
        todayBookings={today_bookings}
        yesterdayBookings={yesterday_bookings}
        occupancyRate={occupancy_rate}
      />
      <PaymentStatusBreakdown statusEntries={statusEntries} />
      <PaymentMethodBreakdown methodEntries={methodEntries} />
    </Panel>
  );
}
