import type { DashboardStats } from '@/shared/types/api.types';
import { TodayRevenueHeader } from './payment-overview/TodayRevenueHeader';
import { TodayMetrics } from './payment-overview/TodayMetrics';
import { PaymentStatusBreakdown } from './payment-overview/PaymentStatusBreakdown';
import { PaymentMethodBreakdown } from './payment-overview/PaymentMethodBreakdown';
import { PaymentOverviewShell } from './PaymentOverviewShell';
import { cn } from '@/shared/lib/utils';

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
  const { payment_summary: summary, today_money, today_bookings, yesterday_bookings, occupancy_rate } = stats;

  // "unpaid" never carries a collected amount (see PaymentStatusBreakdown's
  // comment on GetPaymentSummary's SQL) and self-resolves within the 15-
  // minute payment-expiry window — not something to show in cash control.
  const statusEntries = Object.entries(summary.by_status)
    .filter(([key]) => key !== 'unpaid')
    .sort((a, b) => b[1].total - a[1].total);
  // By method over today_money, so the breakdown adds up to the header's
  // total (bookings plus bar and other till income), not just bookings.
  const methodEntries = Object.entries(today_money.by_method).sort((a, b) => b[1] - a[1]);

  return (
    // The whole card is solid brand green at every width (owner decision,
    // 2026-11-09), not just the key-figures block. The theme's text tokens are
    // re-pointed to the near-black ink inside it, so the status and method
    // breakdowns below follow without their own variants.
    <PaymentOverviewShell
      className={cn(
        'border-primary-500 bg-primary-500 text-text-inverse',
        '[--color-text-primary:var(--color-text-inverse)]',
        '[--color-text-secondary:var(--color-text-inverse)]',
        '[--color-text-tertiary:var(--color-text-inverse)]',
      )}
    >
      <div className="mb-4">
        <TodayRevenueHeader totals={today_money} />
        <TodayMetrics
          todayBookings={today_bookings}
          yesterdayBookings={yesterday_bookings}
          occupancyRate={occupancy_rate}
        />
      </div>
      <PaymentStatusBreakdown statusEntries={statusEntries} />
      <PaymentMethodBreakdown methodEntries={methodEntries} />
    </PaymentOverviewShell>
  );
}
