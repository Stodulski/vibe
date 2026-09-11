import { timeToMinutes } from '@/shared/lib/time';
import { cn, formatTime } from '@/shared/lib/utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import { barPosition } from './timelineGeometry';
import { SHORTEST_BOOKING_MINUTES, SLOT_MINUTES } from './helpers';
import { SLOT_HEIGHT_PX } from './gridLayout';
import type { DurationMinutes } from '@/shared/types/api.types';

const t = ES_AR;

interface FreeSlotButtonProps {
  slot: string;
  courtName: string;
  rangeStartMin: number;
  rangeEndMin: number;
  durations: DurationMinutes[];
  /** True while this is the slot whose duration menu is open. */
  isOpen: boolean;
  /** The duration being pointed at in that menu, so the highlight shows it. */
  previewMin: number | null;
  onPick: (slot: string, durations: DurationMinutes[]) => void;
}

/**
 * A free slot on a court. A real focusable control rather than a click handler
 * doing coordinate maths on the column — reachable by keyboard, and its target
 * is data (the slot string), not a pointer position.
 *
 * It used to carry its own Radix popover for choosing a duration. A day's grid
 * holds about a hundred and seventy of these, and a hundred and seventy popover
 * roots made any re-render of the component owning the scroller cost half a
 * second — which is what froze the drag on its first pixel and stalled the
 * booking drawer on open. The grid keeps one popover now and lends it to
 * whichever slot was pressed (GridBody); this is a button again.
 */
export function FreeSlotButton({
  slot,
  courtName,
  rangeStartMin,
  rangeEndMin,
  durations,
  isOpen,
  previewMin,
  onPick,
}: FreeSlotButtonProps) {
  const startMin = timeToMinutes(slot);
  // The hit area is exactly the slot's own half hour, so consecutive controls
  // never cover one another — an hour-tall button at a half-hour step made the
  // lower half of each one belong to the next, and the highlight jumped a cell.
  //
  // `barPosition`'s `leftPct`/`widthPct` are orientation-agnostic percentages
  // of the range span; aliased here to `top`/`height` for the vertical grid.
  const { leftPct: topPct, widthPct: heightPct } = barPosition(
    startMin,
    startMin + SLOT_MINUTES,
    rangeStartMin,
    rangeEndMin,
  );
  // Covers the shortest booking that could start here, or the duration being
  // pointed at in the menu, so the choice is previewed where it will land.
  const highlightPx = highlightHeightPx(previewMin, startMin, rangeEndMin);
  // The booking runs past the end of this day, so the preview is drawn square
  // on the edge it crosses — the same rule BookingBlock follows once the
  // booking exists. Rounded on all four corners it reads as a session ending
  // at midnight, which is the one thing it does not do: 23:30 for an hour is
  // sold, and the rest of it belongs to tomorrow's grid.
  const runsPastToday = startMin + (previewMin ?? SHORTEST_BOOKING_MINUTES) > rangeEndMin;

  // Not even the shortest booking fits here, so there is nothing to offer.
  if (durations.length === 0) return null;

  return (
    <button
      type="button"
      aria-label={`${t.bookings.timelineFreeSlot} · ${courtName} · ${formatTime(slot)}`}
      aria-haspopup="menu"
      aria-expanded={isOpen}
      onClick={() => {
        onPick(slot, durations);
      }}
      data-open={isOpen ? '' : undefined}
      // Opts out of the global 44px touch-target floor, and needs `!` to do
      // it: that rule is unlayered, and unlayered CSS beats anything in
      // `@layer utilities` whatever the specificity. This control's height
      // *is* data — the half hour it stands for — and inflating it made each
      // slot overlap the next by 16px on touch, stealing taps from its
      // neighbour. At 28px by the full column width it still clears the
      // 24px WCAG 2.5.8 minimum.
      className="group focus-self absolute inset-x-0 !min-h-0"
      style={{ top: `${String(topPct)}%`, height: `${String(heightPct)}%` }}
    >
      <span
        aria-hidden="true"
        className={cn(
          'pointer-events-none absolute inset-x-px top-0 rounded-md border border-transparent transition-all group-hover:border-primary-400/60 group-hover:bg-primary-400/10 group-focus-visible:border-primary-400 group-focus-visible:bg-primary-400/15 group-data-[open]:border-primary-400 group-data-[open]:bg-primary-400/15',
          runsPastToday && 'rounded-b-none border-b-0',
        )}
        style={{ height: `${String(highlightPx)}px` }}
      />
    </button>
  );
}

/**
 * How tall the preview highlight is, in the grid's own pixels.
 *
 * Sized in pixels rather than as a percentage of the button: the global
 * `pointer: coarse` rule lifts every button to a 44px minimum, so a percentage
 * resolved against 44 instead of 28 and the preview ran 31px into the next
 * booking on touch.
 *
 * Clipped at the end of the day. A booking may run into the next one, so
 * previewing 120 minutes from 23:00 would otherwise draw an hour of highlight
 * below the grid's last row — over the page, on hours this column does not
 * represent. The booking is still two hours; what stops at the edge is the
 * drawing of it, the same way the block itself stops there once it exists.
 */
function highlightHeightPx(previewMin: number | null, startMin: number, rangeEndMin: number): number {
  const visibleMin = Math.min(previewMin ?? SHORTEST_BOOKING_MINUTES, rangeEndMin - startMin);
  return (visibleMin / SLOT_MINUTES) * SLOT_HEIGHT_PX;
}
