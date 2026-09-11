import { ChevronRight } from 'lucide-react';
import { cn, formatTime } from '@/shared/lib/utils';
import type { Booking } from '@/shared/types/api.types';

interface TodayBookingRowProps {
  booking: Booking;
  onSelect: (booking: Booking) => void;
  /** Rows past the mobile cap render but stay hidden until `md:` — lets the
   *  desktop-only extra rows exist in the DOM without a second fetch/list. */
  hiddenOnMobile?: boolean;
}

// Every row here is a confirmed booking — TodayBookings filters out pending
// ones (see its own comment), and a confirmed booking is always paid
// (deposit or full). A payment-status badge would only ever show one of
// those two values, never anything actionable, so it's not worth the row's
// space.
export function TodayBookingRow({ booking, onSelect, hiddenOnMobile }: TodayBookingRowProps) {
  return (
    <li className={cn(hiddenOnMobile && 'hidden md:block')}>
      <button
        type="button"
        onClick={() => {
          onSelect(booking);
        }}
        className="flex w-full items-center gap-2 rounded-lg px-2 py-2.5 text-left transition-colors hover:bg-bg-base/40"
      >
        <div className="min-w-0 flex-1">
          {/* Mobile: name on its own line, time/court below. */}
          <p className="truncate text-sm font-medium text-text-primary md:hidden">{booking.client_name}</p>
          <p className="mt-0.5 truncate text-sm text-text-tertiary md:hidden">
            <span className="score-text">{formatTime(booking.start_time)}</span> · {booking.court_name}
          </p>

          {/* Desktop: time, name, court in one line. */}
          <p className="hidden truncate text-sm md:block">
            <span className="score-text text-text-tertiary">{formatTime(booking.start_time)}</span>
            {' · '}
            <span className="font-medium text-text-primary">{booking.client_name}</span>
            {' · '}
            <span className="text-text-tertiary">{booking.court_name}</span>
          </p>
        </div>
        <ChevronRight className="size-4 shrink-0 text-text-tertiary" aria-hidden="true" />
      </button>
    </li>
  );
}
