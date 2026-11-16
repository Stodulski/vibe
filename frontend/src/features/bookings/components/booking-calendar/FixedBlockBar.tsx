import { Repeat } from 'lucide-react';
import { cn } from '@/shared/lib/utils';
import { keepTogether } from '@/shared/lib/keepTogether';
import { formatEventTimeRange } from '@/shared/lib/eventDates';
import { ES_AR } from '@/shared/i18n/es_AR';
import { barPosition } from './timelineGeometry';
import { heldBlockRange } from './fixedBlocks';
import { HELD_BLOCK_STYLES } from './heldBlockStyles';
import { BLOCK_BASE_CLASSES, BLOCK_NAME_CLASSES, BLOCK_TIME_CLASSES, TWO_LINE_MIN_HEIGHT_PX } from './blockStyles';
import { DAY_START_MIN, DAY_END_MIN, SLOT_HEIGHT_PX, SLOT_MINUTES } from './gridLayout';
import type { FixedBlock } from '@/shared/types/api.types';

const t = ES_AR;

/**
 * A date a fixed weekly booking holds that has no booking yet. It is drawn
 * like a booking (same shape, confirmed colours, time range over client name)
 * and only the repeat icon tells it apart; its accessible name says it is a
 * turno fijo. On the day after it started it draws the tail of hours that ran
 * past midnight.
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
  const hours = formatEventTimeRange(block.start_time, block.end_time);
  const ariaLabel = `${keepTogether(courtName)}, ${hours}, ${label}`;

  return (
    <button
      type="button"
      aria-label={ariaLabel}
      title={ariaLabel}
      onClick={onSelect}
      className={cn(
        BLOCK_BASE_CLASSES,
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
            <Repeat className="text-text-primary/80 size-3 shrink-0" aria-hidden="true" />
            <span data-line className={BLOCK_TIME_CLASSES}>
              {hours}
            </span>
          </span>
          <p data-line className={BLOCK_NAME_CLASSES}>
            {block.client_name}
          </p>
        </>
      ) : (
        <span className="flex min-w-0 items-center gap-1">
          <Repeat className="text-text-primary/80 size-3 shrink-0" aria-hidden="true" />
          <span className={BLOCK_NAME_CLASSES}>{block.client_name}</span>
        </span>
      )}
    </button>
  );
}
