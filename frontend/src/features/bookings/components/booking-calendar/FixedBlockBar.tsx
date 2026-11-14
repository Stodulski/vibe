import { Repeat } from 'lucide-react';
import { cn, formatTime } from '@/shared/lib/utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import { barPosition } from './timelineGeometry';
import { heldBlockRange } from './fixedBlocks';
import { HELD_BLOCK_STYLES } from './heldBlockStyles';
import { DAY_START_MIN, DAY_END_MIN, SLOT_HEIGHT_PX, SLOT_MINUTES } from './gridLayout';
import type { FixedBlock } from '@/shared/types/api.types';

const t = ES_AR;

// Same rule as `BookingBlock`: two lines (label, then client) once the block is
// this tall, one short label below it. Derived from the duration, not measured.
const TWO_LINE_MIN_HEIGHT_PX = 56;

/**
 * A date a fixed weekly booking holds that has no booking yet. Drawn dashed in
 * neutral grey and carrying the repeat icon, so it reads as taken without
 * passing for a block the owner made by hand or for a status. On the day after
 * it started it draws the tail of hours that ran past midnight.
 */
export function FixedBlockBar({
  block,
  courtName,
  date,
  onSelect,
}: {
  block: FixedBlock;
  courtName: string;
  /** The day the grid shows, `YYYY-MM-DD`. */
  date: string;
  onSelect: () => void;
}) {
  const range = heldBlockRange(block, date);
  if (!range) return null;
  const { startMin, endMin, isTail, runsOn } = range;
  const {
    leftPct: topPct,
    widthPct: heightPct,
    visibleMin,
  } = barPosition(startMin, endMin, DAY_START_MIN, DAY_END_MIN);
  const heightPx = (visibleMin / SLOT_MINUTES) * SLOT_HEIGHT_PX;
  const tall = heightPx >= TWO_LINE_MIN_HEIGHT_PX;
  const label = t.bookings.series.blockLabel(block.client_name);
  const ariaLabel = `${courtName}, ${formatTime(block.start_time)}–${formatTime(block.end_time)}, ${label}`;

  return (
    <button
      type="button"
      aria-label={ariaLabel}
      title={ariaLabel}
      onClick={onSelect}
      className={cn(
        'focus-self absolute inset-x-[3px] flex cursor-pointer flex-col justify-center overflow-hidden rounded-md border border-dashed px-1.5 py-0.5 text-left transition-[filter] focus-visible:brightness-150',
        HELD_BLOCK_STYLES.fixed,
        // Hours that cross midnight are drawn square on the edge they cross.
        runsOn && 'rounded-b-none border-b-0',
        isTail && 'rounded-t-none border-t-0',
      )}
      style={{ top: `calc(${String(topPct)}% + 1px)`, height: `calc(${String(heightPct)}% - 2px)` }}
    >
      {tall ? (
        <>
          <span className="flex min-w-0 items-center gap-1">
            <Repeat className="text-text-primary size-3 shrink-0" aria-hidden="true" />
            <span className="text-text-primary truncate text-xs font-medium">{t.bookings.series.blockTitle}</span>
          </span>
          <span className="text-text-primary truncate text-xs">{block.client_name}</span>
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
