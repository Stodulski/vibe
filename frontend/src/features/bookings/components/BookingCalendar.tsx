import { ES_AR } from '@/shared/i18n/es_AR';
import { CourtTimeGrid } from './booking-calendar/CourtTimeGrid';
import { useBookingCalendarData } from './booking-calendar/useBookingCalendarData';
import type { ReactNode } from 'react';
import type { Booking, BlockedSlot, CourtWithPrices, FixedBlock, Schedule } from '@/shared/types/api.types';

const t = ES_AR;

// A stable empty list: a fresh `[]` per render would defeat the grid's memoised columns.
const NO_FIXED_BLOCKS: FixedBlock[] = [];

interface BookingCalendarProps {
  bookings: Booking[];
  blockedSlots: BlockedSlot[];
  /** Defaults to none: callers that do not draw fixed weekly bookings need not pass it. */
  fixedBlocks?: FixedBlock[] | undefined;
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
  onSelectFixedBlock?: ((block: FixedBlock) => void) | undefined;
}

export function BookingCalendar({
  bookings,
  blockedSlots,
  fixedBlocks = NO_FIXED_BLOCKS,
  courts,
  date,
  schedules,
  onSelectBooking,
  onCreateBooking,
  onSelectBlockedSlot,
  onSelectFixedBlock,
}: BookingCalendarProps) {
  const data = useBookingCalendarData({ courts, date, schedules });

  if (data.isClosedToday) {
    return (
      <div className="border-border-subtle bg-bg-subtle flex items-center justify-center rounded-2xl border py-16">
        <p className="text-text-tertiary text-sm">{t.bookings.closedToday}</p>
      </div>
    );
  }

  return (
    <div>
      <CourtTimeGrid
        bookings={bookings}
        blockedSlots={blockedSlots}
        fixedBlocks={fixedBlocks}
        activeCourts={data.activeCourts}
        date={date}
        slots={data.slots}
        onSelectBooking={onSelectBooking}
        onCreateBooking={onCreateBooking}
        onSelectBlockedSlot={onSelectBlockedSlot}
        onSelectFixedBlock={onSelectFixedBlock}
      />
    </div>
  );
}
