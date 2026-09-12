import { memo, useCallback, useMemo } from 'react';
import { cn } from '@/shared/lib/utils';
import { courtObstacles, isSlotOccupied, upcomingSlots } from './helpers';
import { FreeSlots } from './FreeSlots';
import { BlockedBlock } from './BlockedBlock';
import { BookingBlock } from './BookingBlock';
import { COLUMN_WIDTH_PX, DAY_START_MIN, DAY_END_MIN } from './gridLayout';
import type { Booking, BlockedSlot, DurationMinutes } from '@/shared/types/api.types';
import type { TimelineColumnData } from './useTimelineColumns';

/**
 * One court's column: free-slot controls, blocked bars, then the booking
 * blocks, all absolutely positioned by time-of-day.
 *
 * What's drawn and what counts as free come from the same rule — every
 * booking except a cancelled one — so the column can never show a slot as
 * both taken and available.
 *
 * `isFirst` is told, not asked of CSS: the gridlines and the now-line render
 * before the columns, so `:first-child` is one of them and a `first:` variant
 * would leave a stray rule down the grid's left edge.
 */
interface CourtColumnProps {
  column: TimelineColumnData;
  isFirst: boolean;
  date: string;
  allBookings: Booking[];
  allBlockedSlots: BlockedSlot[];
  slots: string[];
  /** Minutes since midnight when the column's date is today, else null. */
  nowMin: number | null;
  onSelectBooking: (booking: Booking) => void;
  /**
   * Opens the grid's one duration menu on this court's slot. Takes the court
   * so the grid can hand every column the same function.
   */
  onPickSlot: (courtId: string, slot: string, durations: DurationMinutes[]) => void;
  /** The slot on this court whose menu is open, if the open one is here. */
  openSlot: string | null;
  previewMin: number | null;
  onSelectBlockedSlot?: ((slot: BlockedSlot) => void) | undefined;
}

function ColumnBlockedSlots({
  blocked,
  courtName,
  onSelectBlockedSlot,
}: {
  blocked: BlockedSlot[];
  courtName: string;
  onSelectBlockedSlot?: ((slot: BlockedSlot) => void) | undefined;
}) {
  return (
    <>
      {blocked.map((slot) => (
        <BlockedBlock
          key={slot.id}
          slot={slot}
          courtName={courtName}
          onSelect={() => {
            onSelectBlockedSlot?.(slot);
          }}
        />
      ))}
    </>
  );
}

function ColumnBookings({
  bookings,
  courtName,
  date,
  onSelectBooking,
}: {
  bookings: Booking[];
  courtName: string;
  date: string;
  onSelectBooking: (booking: Booking) => void;
}) {
  return (
    <>
      {bookings.map((booking) => (
        <BookingBlock
          key={booking.id}
          booking={booking}
          courtName={courtName}
          date={date}
          onSelect={() => {
            onSelectBooking(booking);
          }}
        />
      ))}
    </>
  );
}

function CourtColumnImpl({
  column,
  isFirst,
  date,
  allBookings,
  allBlockedSlots,
  slots,
  nowMin,
  onSelectBooking,
  onPickSlot,
  openSlot,
  previewMin,
  onSelectBlockedSlot,
}: CourtColumnProps) {
  // Once for the column, not once per slot: every free-slot control asks the
  // same question of the same day's bookings and blocks.
  const obstacles = useMemo(
    () => courtObstacles(allBookings, allBlockedSlots, column.courtId, date),
    [allBookings, allBlockedSlots, column.courtId, date],
  );
  const handlePick = useCallback(
    (slot: string, durations: DurationMinutes[]) => {
      onPickSlot(column.courtId, slot, durations);
    },
    [onPickSlot, column.courtId],
  );
  const freeSlots = upcomingSlots(slots, nowMin).filter((slot) => !isSlotOccupied(obstacles, slot, date));

  return (
    <div
      className={cn('relative shrink-0', !isFirst && 'border-border-subtle border-l')}
      // `h-full`, not a fixed 1344: the grid's own height already includes its
      // top and bottom borders, so a column pinned to the full pixel figure
      // stuck 2px out the bottom — and a horizontally scrollable box is never
      // `overflow-y: visible`, so those 2px became a vertical scroll axis.
      style={{ width: `${String(COLUMN_WIDTH_PX)}px`, height: '100%' }}
    >
      <FreeSlots
        freeSlots={freeSlots}
        courtName={column.courtName}
        date={date}
        rangeStartMin={DAY_START_MIN}
        rangeEndMin={DAY_END_MIN}
        obstacles={obstacles}
        openSlot={openSlot}
        previewMin={previewMin}
        onPick={handlePick}
      />
      <ColumnBlockedSlots
        blocked={column.blocked}
        courtName={column.courtName}
        onSelectBlockedSlot={onSelectBlockedSlot}
      />
      <ColumnBookings
        bookings={column.bookings}
        courtName={column.courtName}
        date={date}
        onSelectBooking={onSelectBooking}
      />
    </div>
  );
}

/**
 * Memoized because the grid's menu state lives above it. Opening a duration
 * menu changes one column's props; without this every column re-runs its
 * obstacle pass and rebuilds its blocks, which is most of what that click cost.
 */
export const CourtColumn = memo(CourtColumnImpl);
