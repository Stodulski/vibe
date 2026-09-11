import { useMemo } from 'react';
import { getNowMinutesInBuenosAires } from './nowIndicator';
import { useTimelineColumns } from './useTimelineColumns';
import { useColumnScroller } from './useColumnScroller';
import { useDragScroll } from './useDragScroll';
import { cn } from '@/shared/lib/utils';
import { StickyCourtNames } from './StickyCourtNames';
import { GridBody } from './GridBody';
import { GridHourGutter } from './GridHourGutter';
import { GUTTER_CLASS, HEADER_HEIGHT_PX } from './gridLayout';
import type { CSSProperties } from 'react';
import type { Booking, BlockedSlot, CourtWithPrices } from '@/shared/types/api.types';
import type { CreateBookingPrefill } from './types';

/**
 * Vertical time grid: the whole day runs top-to-bottom in a left gutter, and
 * each court is a fixed-width column — the classic court-booking layout, at
 * every viewport width. Courts that don't fit are reached by dragging the
 * columns sideways.
 *
 * Renders at its natural full-day height with no vertical scroll container of
 * its own — the page's own scroll carries it instead of nesting a second
 * scrollbar. The horizontal scroller is the columns' alone: the hour gutter
 * sits outside it so the times stay put while the courts move.
 */
interface CourtTimeGridProps {
  bookings: Booking[];
  blockedSlots: BlockedSlot[];
  activeCourts: CourtWithPrices[];
  date: string;
  slots: string[];
  onSelectBooking: (booking: Booking) => void;
  onCreateBooking: (prefill: CreateBookingPrefill) => void;
  onSelectBlockedSlot?: ((slot: BlockedSlot) => void) | undefined;
}

export function CourtTimeGrid({
  bookings,
  blockedSlots,
  activeCourts,
  date,
  slots,
  onSelectBooking,
  onCreateBooking,
  onSelectBlockedSlot,
}: CourtTimeGridProps) {
  // A block means "this court is taken", so the rule is exactly the one
  // `isSlotOccupied` uses for free slots: everything but a cancellation. Any
  // other filter here would let a slot show as free while a booking still
  // holds it — or the reverse — with the two disagreeing on the same screen.
  const occupyingBookings = useMemo(() => bookings.filter((b) => b.status !== 'cancelled'), [bookings]);
  const columns = useTimelineColumns({ activeCourts, bookings: occupyingBookings, blockedSlots });
  const nowMin = getNowMinutesInBuenosAires(date);
  const { ref, trackRef, frameRef } = useColumnScroller();
  const { dragging, dragHandlers } = useDragScroll(ref);

  return (
    // Full bleed to the right on a phone: `-mr-3` cancels the shell's own
    // `px-3` (AppShell.tsx), so the columns and their edge fade reach the side
    // of the screen instead of stopping a gutter short of it. On a phone the
    // grid is the page, and a strip of padding there reads as the grid ending.
    // Only below `sm`, where that padding is 3 — from `sm` up the page's own
    // margins are worth keeping and there is room to spare.
    <div className="relative flex -mr-3 sm:mr-0">
      <div className={GUTTER_CLASS}>
        {/* Pushes the gutter past the court names so its hours line up with
            the grid rows rather than with the header. */}
        <div style={{ height: `${String(HEADER_HEIGHT_PX)}px` }} />
        <GridHourGutter />
      </div>

      {/* No padding on either side any more. Both edges were lanes for the
          arrow shortcuts, and the columns now run to the full width they have.
          The left lane was the one that pushed the hour labels a column's width
          away from the grid they label. */}
      {/* The fades start below the court names: they mark the grid as cut off,
          and a green wash over a court's own label reads as the label's colour
          rather than as an edge. */}
      <div
        ref={frameRef}
        className="scroll-edges relative min-w-0 flex-1"
        style={{ '--scroll-edge-top': `${String(HEADER_HEIGHT_PX)}px` } as CSSProperties}
      >
        <StickyCourtNames columns={columns} trackRef={trackRef} />

        <div
          ref={ref}
          {...dragHandlers}
          data-dragging={dragging ? '' : undefined}
          // `data-scrollable` (written by useColumnScroller) gates both cursors:
          // a hand over a grid already showing every court promises a drag that
          // cannot move. `data-dragging` gates `.drag-surface`'s child override
          // in globals.css, so slots keep their own pointer at rest and only
          // stop arguing about the cursor once a drag is actually under way.
          className={cn(
            'drag-surface scrollbar-none overflow-x-auto select-none',
            dragging ? 'data-scrollable:cursor-grabbing' : 'data-scrollable:cursor-grab',
          )}
        >
          <GridBody
            columns={columns}
            date={date}
            bookings={bookings}
            blockedSlots={blockedSlots}
            slots={slots}
            nowMin={nowMin}
            onSelectBooking={onSelectBooking}
            onCreateBooking={onCreateBooking}
            onSelectBlockedSlot={onSelectBlockedSlot}
          />
        </div>
      </div>
    </div>
  );
}
