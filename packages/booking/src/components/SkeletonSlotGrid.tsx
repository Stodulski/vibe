import { Skeleton } from '@vibe/ui';
import { BOOKING_MESSAGES } from '../messages';

const t = BOOKING_MESSAGES;

/** Ticks drawn in the loading ruler: enough to fill a desktop row, all one height like the ruler's. */
const TICKS = 11;

/**
 * Mirrors `TimeRuler`: the large hour and its price line, the ruler of ticks under
 * a marker, the hint, and the Continuar button, at the same heights, so the page
 * does not jump when the hours arrive.
 */
export function SkeletonSlotGrid() {
  return (
    <div className="flex flex-col gap-4" role="status" aria-label={t.publicBooking.checkingAvailability}>
      <div className="flex flex-col items-center gap-2">
        <Skeleton className="h-9 w-48 rounded-lg" />
        <Skeleton className="h-4 w-36 rounded-full" />
      </div>
      <div data-testid="ruler-skeleton" className="flex justify-center gap-[3.25rem] overflow-hidden py-2">
        {Array.from({ length: TICKS }, (_, index) => (
          <Skeleton key={index} className="h-8 w-0.5 shrink-0" />
        ))}
      </div>
      <Skeleton className="mx-auto h-3 w-40 rounded-full" />
      <Skeleton className="h-12 w-full rounded-full" />
    </div>
  );
}
