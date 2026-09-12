import { useState } from 'react';
import type { OpenSlot } from './SlotDurationMenu';
import type { DurationMinutes } from '@/shared/types/api.types';
import type { TimelineColumnData } from './useTimelineColumns';
import type { CreateBookingPrefill } from './types';

/**
 * Which free slot has the grid's one duration menu, and which duration is
 * being pointed at inside it.
 *
 * Both belong to the grid rather than to a slot, because there is one menu for
 * all of them. A slot used to own its own popover and its own preview state,
 * which meant a day's grid held about a hundred and seventy popover roots —
 * enough that any re-render of the component owning the scroller cost half a
 * second.
 *
 * `openOn` is keyed by court rather than by column so it can keep one identity
 * for the grid's whole life. A callback built per column would change on every
 * render and defeat the columns' memo, which is the point of having one menu:
 * opening it should repaint the column that owns it, not all twelve.
 */
export function useSlotMenu(
  columns: TimelineColumnData[],
  date: string,
  onCreateBooking: (prefill: CreateBookingPrefill) => void,
) {
  const [open, setOpen] = useState<OpenSlot | null>(null);
  const [preview, setPreview] = useState<DurationMinutes | null>(null);

  const close = () => {
    setOpen(null);
    setPreview(null);
  };

  // Identity comes from the React Compiler, which binds it to `columns` — so
  // it keeps one identity across opening and closing the menu, and only
  // changes when the courts or their contents actually do.
  const openOn = (courtId: string, slot: string, durations: DurationMinutes[]) => {
    const columnIndex = columns.findIndex((c) => c.courtId === courtId);
    const column = columns[columnIndex];
    if (!column) return;

    setPreview(null);
    setOpen({ columnIndex, courtId, courtName: column.courtName, slot, durations });
  };

  const pick = (duration: DurationMinutes) => {
    const picked = open;
    close();
    if (picked) {
      onCreateBooking({
        court_id: picked.courtId,
        date,
        start_time: picked.slot,
        duration_minutes: duration,
      });
    }
  };

  return { open, preview, setPreview, openOn, pick, close };
}
