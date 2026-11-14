import { Repeat } from 'lucide-react';
import { timeToMinutes } from '@/shared/lib/time';
import { cn, formatTime } from '@/shared/lib/utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import { barPosition } from './timelineGeometry';
import { fixedBlockEndMin } from './fixedBlocks';
import { DAY_START_MIN, DAY_END_MIN, SLOT_HEIGHT_PX, SLOT_MINUTES } from './gridLayout';
import type { FixedBlock } from '@/shared/types/api.types';

const t = ES_AR;

// Same rule as `BookingBlock`: two lines (label, then client) once the block is
// this tall, one short label below it. Derived from the duration, not measured.
const TWO_LINE_MIN_HEIGHT_PX = 56;

/**
 * A date a fixed weekly booking holds that has no booking yet. Drawn in the
 * blocked slot's red family, but dashed and carrying the repeat icon, so it
 * reads as taken without passing for a block the owner made by hand.
 */
export function FixedBlockBar({
  block,
  courtName,
  onSelect,
}: {
  block: FixedBlock;
  courtName: string;
  onSelect: () => void;
}) {
  const startMin = timeToMinutes(block.start_time);
  const endMin = fixedBlockEndMin(block);
  const {
    leftPct: topPct,
    widthPct: heightPct,
    visibleMin,
  } = barPosition(startMin, endMin, DAY_START_MIN, DAY_END_MIN);
  const heightPx = (visibleMin / SLOT_MINUTES) * SLOT_HEIGHT_PX;
  const tall = heightPx >= TWO_LINE_MIN_HEIGHT_PX;
  // Hours that run into the next day are drawn square on the edge they cross.
  const runsOnPastToday = timeToMinutes(block.end_time) <= startMin;
  const label = t.bookings.series.blockLabel(block.client_name);
  const ariaLabel = `${courtName}, ${formatTime(block.start_time)}–${formatTime(block.end_time)}, ${label}`;

  return (
    <button
      type="button"
      aria-label={ariaLabel}
      title={ariaLabel}
      onClick={onSelect}
      className={cn(
        'focus-self border-error-icon bg-error-border absolute inset-x-[3px] flex cursor-pointer flex-col justify-center overflow-hidden rounded-md border border-dashed px-1.5 py-0.5 text-left transition-[filter] focus-visible:brightness-150',
        runsOnPastToday && 'rounded-b-none border-b-0',
      )}
      style={{ top: `calc(${String(topPct)}% + 1px)`, height: `calc(${String(heightPct)}% - 2px)` }}
    >
      {tall ? (
        <>
          <span className="flex min-w-0 items-center gap-1">
            <Repeat className="text-text-primary size-3 shrink-0" aria-hidden="true" />
            <span className="text-text-primary truncate text-xs font-medium">{t.bookings.series.blockTitle}</span>
          </span>
          <span className="text-text-primary/80 truncate text-xs">{block.client_name}</span>
        </>
      ) : (
        <span className="flex min-w-0 items-center gap-1">
          <Repeat className="text-text-primary size-3 shrink-0" aria-hidden="true" />
          <span className="text-text-primary truncate text-xs font-medium">{t.bookings.series.blockLabelCompact}</span>
        </span>
      )}
    </button>
  );
}
