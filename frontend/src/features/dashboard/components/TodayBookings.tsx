import { memo } from 'react';
import { Calendar } from 'lucide-react';
import { useNavigate } from 'react-router-dom';
import { ES_AR } from '@/shared/i18n/es_AR';
import { EmptyState } from '@/shared/components/common/EmptyState';
import { Tag } from '@/shared/components/common/Tag';
import { Panel } from '@/shared/components/common/Panel';
import { useDashboardBookingDetail } from '../hooks/useDashboardBookingDetail';
import { useDashboardClientDetail } from '../hooks/useDashboardClientDetail';
import { BookingDetailModals } from '@/features/bookings';
import { ClientInsightsDetailModals } from './client-insights/ClientInsightsDetailModals';
import type { Booking } from '@/shared/types/api.types';
import { TodayBookingRow } from './today-bookings/TodayBookingRow';
import { ViewAllBookingsButton } from './today-bookings/ViewAllBookingsButton';

import { MAX_VISIBLE_BOOKINGS, MAX_VISIBLE_BOOKINGS_DESKTOP } from '@/shared/lib/constants';

const t = ES_AR;

interface TodayBookingsProps {
  bookings: Booking[];
  complexId: string;
}

export const TodayBookings = memo(function TodayBookings({ bookings, complexId }: TodayBookingsProps) {
  const navigate = useNavigate();
  const detail = useDashboardBookingDetail(complexId);
  const clientDetail = useDashboardClientDetail(complexId);
  // Defensive: the API (GetUpcomingToday) already only returns confirmed
  // bookings — a pending one is a client mid-checkout, self-resolving within
  // 15 minutes (payment confirms it or it expires), never something the
  // owner needs to act on. Filtering again here costs nothing and keeps this
  // component correct even if that guarantee ever changes upstream.
  const confirmedBookings = bookings.filter((b) => b.status === 'confirmed');
  // Renders up to the desktop cap; rows past the mobile cap stay in the DOM
  // but hidden until `md:` (see TodayBookingRow), so there's one list, not two.
  const visible = confirmedBookings.slice(0, MAX_VISIBLE_BOOKINGS_DESKTOP);

  return (
    <Panel
      as="section"
      size="sm"
      className="flex h-full flex-col py-4 sm:p-4"
      aria-label={t.dashboard.upcomingBookings}
    >
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

      {confirmedBookings.length === 0 ? (
        <EmptyState
          icon={Calendar}
          title={t.dashboard.noUpcoming}
          description=""
          actionLabel={t.dashboard.newBooking}
          onAction={() => {
            void navigate('/bookings');
          }}
        />
      ) : (
        <>
          {/* Tailwind's preflight sets `list-style: none` on every `ul`/`ol`
              (no `list-*` utility restores it here), and Safari/VoiceOver
              drops a list's implicit ARIA role the moment its list-style is
              removed — https://www.scottohara.me/blog/2019/01/12/lists-and-safari.html.
              `role="list"` restores it; jsx-a11y only reasons about markup,
              not the CSS reset that makes the explicit role necessary. */}
          {/* eslint-disable-next-line jsx-a11y/no-redundant-roles -- see comment above */}
          <ul className="flex-1 space-y-1" role="list">
            {visible.map((b, i) => (
              <TodayBookingRow
                key={b.id}
                booking={b}
                onSelect={detail.handleSelectBooking}
                hiddenOnMobile={i >= MAX_VISIBLE_BOOKINGS}
              />
            ))}
          </ul>

          <ViewAllBookingsButton />
          <ViewAllBookingsButton desktopOnly />
        </>
      )}

      <BookingDetailModals detail={detail} complexId={complexId} onOpenClient={clientDetail.handleSelectClient} />
      <ClientInsightsDetailModals detail={clientDetail} complexId={complexId} />
    </Panel>
  );
});
