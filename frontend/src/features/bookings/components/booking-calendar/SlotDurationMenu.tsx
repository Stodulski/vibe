import { Popover, PopoverAnchor, PopoverContent } from '@/shared/components/ui/popover';
import { formatTime } from '@/shared/lib/utils';
import { timeToMinutes } from '@/shared/lib/time';
import { DurationChoices } from './DurationChoices';
import { minutesToPx, SLOT_HEIGHT_PX } from './gridLayout';
import type { DurationMinutes } from '@/shared/types/api.types';

/** The slot whose duration menu is open, and everything needed to place it. */
export interface OpenSlot {
  columnIndex: number;
  courtId: string;
  courtName: string;
  slot: string;
  durations: DurationMinutes[];
}

/**
 * The grid's one duration menu, lent to whichever free slot was pressed.
 *
 * One, not one per slot. Every free slot used to carry its own Radix popover,
 * and a day holds about a hundred and seventy of them — enough that any
 * re-render of the component owning the scroller took half a second, which is
 * what froze the horizontal drag on its first pixel and stalled the booking
 * drawer on open. Only one menu can ever be open, so only one is built.
 *
 * It mounts inside the plot area, whose box is the coordinate system its anchor
 * is placed in.
 */
export function SlotDurationMenu({
  open,
  columnWidth,
  onPreview,
  onPick,
  onClose,
}: {
  open: OpenSlot | null;
  /**
   * The width the grid gave every column. The anchor is arithmetic on the
   * column index, so this has to be the very number the columns were drawn at.
   */
  columnWidth: number;
  onPreview: (duration: DurationMinutes | null) => void;
  onPick: (duration: DurationMinutes) => void;
  onClose: () => void;
}) {
  return (
    <Popover
      open={open !== null}
      onOpenChange={(next) => {
        if (!next) onClose();
      }}
    >
      {/* An anchor placed on the slot's own cell rather than rendered inside
          it: the columns lay out in order at one shared width, so the cell's
          box is arithmetic the grid already knows. It carries no pointer
          events — it exists only to tell the popover where to point. */}
      <PopoverAnchor asChild>
        <div aria-hidden="true" className="pointer-events-none absolute" style={anchorBox(open, columnWidth)} />
      </PopoverAnchor>
      {/* Above, not below: the preview grows downward, so the default bottom
          placement covered exactly the stretch it was meant to show. Above
          also fits any column width — to the side it ran off a phone screen. */}
      <PopoverContent side="top" align="center" sideOffset={6} collisionPadding={8} className="w-40 p-2">
        {open && (
          <>
            <p className="score-text text-text-tertiary mb-1.5 px-1 text-xs">
              {open.courtName} · {formatTime(open.slot)}
            </p>
            <DurationChoices durations={open.durations} onPreview={onPreview} onPick={onPick} />
          </>
        )}
      </PopoverContent>
    </Popover>
  );
}

/** The open slot's cell, in the plot area's own coordinates. */
function anchorBox(open: OpenSlot | null, columnWidth: number): React.CSSProperties {
  if (!open) return { left: 0, top: 0, width: 0, height: 0 };
  return {
    left: `${String(open.columnIndex * columnWidth)}px`,
    width: `${String(columnWidth)}px`,
    top: `${String(minutesToPx(timeToMinutes(open.slot)))}px`,
    height: `${String(SLOT_HEIGHT_PX)}px`,
  };
}
