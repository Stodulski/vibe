import { FreeSlotButton } from './FreeSlotButton';
import { fittingDurations } from './helpers';
import type { Span } from '@/shared/lib/instants';
import type { DurationMinutes } from '@/shared/types/api.types';

export function FreeSlots({
  freeSlots,
  courtName,
  date,
  rangeStartMin,
  rangeEndMin,
  obstacles,
  openSlot,
  previewMin,
  onPick,
}: {
  freeSlots: string[];
  courtName: string;
  date: string;
  rangeStartMin: number;
  rangeEndMin: number;
  /** This court's taken stretches, computed once for the column. */
  obstacles: Span[];
  /** The slot on THIS court whose duration menu is open, if any. */
  openSlot: string | null;
  previewMin: number | null;
  onPick: (slot: string, durations: DurationMinutes[]) => void;
}) {
  return (
    <>
      {freeSlots.map((slot) => (
        <FreeSlotButton
          key={slot}
          slot={slot}
          courtName={courtName}
          rangeStartMin={rangeStartMin}
          rangeEndMin={rangeEndMin}
          durations={fittingDurations(obstacles, slot, date)}
          isOpen={openSlot === slot}
          previewMin={openSlot === slot ? previewMin : null}
          onPick={onPick}
        />
      ))}
    </>
  );
}
