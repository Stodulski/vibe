import { Calendar } from 'lucide-react';
import { ES_AR } from '@/shared/i18n/es_AR';
import { EmptyState } from '@/shared/components/common/EmptyState';
import { TodayBookingRow } from './TodayBookingRow';
import { ViewAllBookingsButton } from './ViewAllBookingsButton';
import { MAX_VISIBLE_BOOKINGS, MAX_VISIBLE_BOOKINGS_DESKTOP } from '@/shared/lib/constants';
import type { Booking } from '@/shared/types/api.types';

const t = ES_AR;

interface TodayBookingsListProps {
  confirmedBookings: Booking[];
  onSelect: (booking: Booking) => void;
  onCreateBooking: () => void;
}

export function TodayBookingsList({ confirmedBookings, onSelect, onCreateBooking }: TodayBookingsListProps) {
  if (confirmedBookings.length === 0) {
    return (
      <EmptyState
        icon={Calendar}
        title={t.dashboard.noUpcoming}
        description=""
        actionLabel={t.dashboard.newBooking}
        onAction={onCreateBooking}
      />
    );
  }

  // Renders up to the desktop cap; rows past the mobile cap stay in the DOM
  // but hidden until `md:` (see TodayBookingRow), so there's one list, not two.
  const visible = confirmedBookings.slice(0, MAX_VISIBLE_BOOKINGS_DESKTOP);

  return (
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
          <TodayBookingRow key={b.id} booking={b} onSelect={onSelect} hiddenOnMobile={i >= MAX_VISIBLE_BOOKINGS} />
        ))}
      </ul>

      <ViewAllBookingsButton />
      <ViewAllBookingsButton desktopOnly />
    </>
  );
}
