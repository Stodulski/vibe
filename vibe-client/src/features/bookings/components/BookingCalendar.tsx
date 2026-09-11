import { ES_AR } from '@/shared/i18n/es_AR';
import { CourtTimeGrid } from './booking-calendar/CourtTimeGrid';
import { useBookingCalendarData } from './booking-calendar/useBookingCalendarData';
import type { ReactNode } from 'react';
import type { Booking, BlockedSlot, CourtWithPrices, Schedule } from '@/shared/types/api.types';

const t = ES_AR;

interface BookingCalendarProps {
  bookings: Booking[];
  blockedSlots: BlockedSlot[];
  courts: CourtWithPrices[];
  date: string;
  schedules: Schedule[];
  /**
   * Ignored by the grid: an empty day still has its courts, and showing
   * their free time is the entire point. Kept in the signature because
   * `BookingContent` still passes it and its test asserts it's rendered.
   */
  emptyState?: ReactNode;
  onSelectBooking: (booking: Booking) => void;
  onCreateBooking: (prefill: { court_id: string; date: string; start_time: string }) => void;
  onSelectBlockedSlot?: ((slot: BlockedSlot) => void) | undefined;
}

export function BookingCalendar({
  bookings,
  blockedSlots,
  courts,
  date,
  schedules,
  onSelectBooking,
  onCreateBooking,
  onSelectBlockedSlot,
}: BookingCalendarProps) {
  const data = useBookingCalendarData({ courts, date, schedules });

  if (data.isClosedToday) {
    return (
      <div className="flex items-center justify-center rounded-2xl border border-border-subtle bg-bg-subtle py-16">
        <p className="text-sm text-text-tertiary">{t.bookings.closedToday}</p>
      </div>
    );
  }

  return (
    <div>
      <CourtTimeGrid
        bookings={bookings}
        blockedSlots={blockedSlots}
        activeCourts={data.activeCourts}
        date={date}
        slots={data.slots}
        onSelectBooking={onSelectBooking}
        onCreateBooking={onCreateBooking}
        onSelectBlockedSlot={onSelectBlockedSlot}
      />
    </div>
  );
}
