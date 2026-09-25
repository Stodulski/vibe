import { memo } from 'react';
import { useNavigate } from 'react-router-dom';
import { ES_AR } from '@/shared/i18n/es_AR';
import { Tag } from '@/shared/components/common/Tag';
import { Panel } from '@/shared/components/common/Panel';
import { useDashboardBookingDetail } from '../hooks/useDashboardBookingDetail';
import { useDashboardClientDetail } from '../hooks/useDashboardClientDetail';
import { BookingDetailModals } from '@/features/bookings';
import { ClientInsightsDetailModals } from './client-insights/ClientInsightsDetailModals';
import { TodayBookingsSummary } from './today-bookings/TodayBookingsSummary';
import { PaymentStatusBreakdown } from './today-bookings/PaymentStatusBreakdown';
import { TodayBookingsList } from './today-bookings/TodayBookingsList';
import type { Booking, PaymentSummary } from '@/shared/types/api.types';

const t = ES_AR;

interface TodayBookingsProps {
  bookings: Booking[];
  complexId: string;
  /** Today's confirmed booking count, and yesterday's for the comparison badge. */
  todayBookingsCount: number;
  yesterdayBookingsCount: number;
  occupancyRate: number;
  /** Moved in from the old PaymentOverview card (odd/tasks/dashboard-today-card.md):
   * collection state is about today's bookings, the same subject this card
   * already covers, not about money entered (the "Hoy" card's own concern). */
  statusEntries: [string, PaymentSummary['by_status'][string]][];
}

export const TodayBookings = memo(function TodayBookings({
  bookings,
  complexId,
  todayBookingsCount,
  yesterdayBookingsCount,
  occupancyRate,
  statusEntries,
}: TodayBookingsProps) {
  const navigate = useNavigate();
  const detail = useDashboardBookingDetail(complexId);
  const clientDetail = useDashboardClientDetail(complexId);
  // Defensive: the API (GetUpcomingToday) already only returns confirmed
  // bookings — a pending one is a client mid-checkout, self-resolving within
  // 15 minutes (payment confirms it or it expires), never something the
  // owner needs to act on. Filtering again here costs nothing and keeps this
  // component correct even if that guarantee ever changes upstream.
  const confirmedBookings = bookings.filter((b) => b.status === 'confirmed');

  return (
    <Panel as="section" size="sm" className="flex h-full flex-col p-4" aria-label={t.dashboard.upcomingBookings}>
      <div className="mb-4 flex items-center justify-between">
        <div className="flex items-baseline gap-2">
          <h3 className="text-text-primary text-sm font-semibold whitespace-nowrap">{t.dashboard.upcomingBookings}</h3>
          {confirmedBookings.length > 0 && (
            <Tag tone="primary" className="px-1.5 text-sm">
              {confirmedBookings.length}
            </Tag>
          )}
        </div>
      </div>

      <TodayBookingsSummary
        todayBookingsCount={todayBookingsCount}
        yesterdayBookingsCount={yesterdayBookingsCount}
        occupancyRate={occupancyRate}
      />

      <PaymentStatusBreakdown statusEntries={statusEntries} />
      <div className="border-border-subtle my-4 border-t" />

      <TodayBookingsList
        confirmedBookings={confirmedBookings}
        onSelect={detail.handleSelectBooking}
        onCreateBooking={() => {
          void navigate('/bookings');
        }}
      />

      <BookingDetailModals detail={detail} complexId={complexId} onOpenClient={clientDetail.handleSelectClient} />
      <ClientInsightsDetailModals detail={clientDetail} complexId={complexId} />
    </Panel>
  );
});
