import { SlotDurationMenu } from './SlotDurationMenu';
import { useSlotMenu } from './useSlotMenu';
import { GridHourLines } from './GridHourLines';
import { NowLine } from './NowLine';
import { CourtColumn } from './CourtColumn';
import { COLUMN_WIDTH_PX, GRID_HEIGHT_PX } from './gridLayout';
import type { Booking, BlockedSlot } from '@/shared/types/api.types';
import type { CreateBookingPrefill } from './types';
import type { TimelineColumnData } from './useTimelineColumns';

/**
 * The plot area: gridlines, the now line, every court column at its fixed
 * width, and the one duration menu they share.
 *
 * One, not one per slot. Each free slot used to carry its own Radix popover,
 * and a day holds about a hundred and seventy of them — enough that any
 * re-render of the component owning the scroller took half a second, which is
 * what froze the drag on its first pixel and stalled the booking drawer on
 * open. Only one can ever be open, so only one is built, anchored to whichever
 * slot was pressed.
 */
export function GridBody({
  columns,
  date,
  bookings,
  blockedSlots,
  slots,
  nowMin,
  onSelectBooking,
  onCreateBooking,
  onSelectBlockedSlot,
}: {
  columns: TimelineColumnData[];
  date: string;
  bookings: Booking[];
  blockedSlots: BlockedSlot[];
  slots: string[];
  nowMin: number | null;
  onSelectBooking: (booking: Booking) => void;
  onCreateBooking: (prefill: CreateBookingPrefill) => void;
  onSelectBlockedSlot?: ((slot: BlockedSlot) => void) | undefined;
}) {
  const { open, preview, setPreview, openOn, pick, close } = useSlotMenu(columns, date, onCreateBooking);

  return (
    // The surface is the plot area alone — the hour gutter and the court names
    // read as annotations of the grid, not part of its face. Width is pinned
    // to the columns so the gridlines span all of them, not just the visible
    // slice of the scroller.
    <div
      className="relative flex border-y border-border-subtle bg-bg-base"
      style={{
        height: `${String(GRID_HEIGHT_PX)}px`,
        width: `${String(columns.length * COLUMN_WIDTH_PX)}px`,
      }}
    >
      <GridHourLines />
      {nowMin !== null && <NowLine nowMin={nowMin} />}
      {columns.map((column, index) => (
        <CourtColumn
          key={column.courtId}
          column={column}
          isFirst={index === 0}
          nowMin={nowMin}
          date={date}
          allBookings={bookings}
          allBlockedSlots={blockedSlots}
          slots={slots}
          onSelectBooking={onSelectBooking}
          onPickSlot={openOn}
          openSlot={open?.courtId === column.courtId ? open.slot : null}
          previewMin={preview}
          onSelectBlockedSlot={onSelectBlockedSlot}
        />
      ))}

      <SlotDurationMenu open={open} onPreview={setPreview} onPick={pick} onClose={close} />
    </div>
  );
}
