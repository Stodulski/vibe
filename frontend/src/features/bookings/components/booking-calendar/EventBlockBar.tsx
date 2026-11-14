import { CalendarDays } from 'lucide-react';
import { cn } from '@/shared/lib/utils';
import { keepTogether } from '@/shared/lib/keepTogether';
import { formatEventTimeRange } from '@/shared/lib/eventDates';
import { ES_AR } from '@/shared/i18n/es_AR';
import { barPosition } from './timelineGeometry';
import { heldBlockRange } from './fixedBlocks';
import { HELD_BLOCK_STYLES } from './heldBlockStyles';
import { DAY_START_MIN, DAY_END_MIN, SLOT_HEIGHT_PX, SLOT_MINUTES } from './gridLayout';
import type { CourtEventBlock } from '@/shared/types/api.types';

const t = ES_AR;

// Same rule as `BookingBlock`: the hours above the label once the block is
// this tall, the short label alone below it.
const TWO_LINE_MIN_HEIGHT_PX = 56;

/** The category is what follows the last " · " of a match block's name. */
function categoryOf(name: string): string {
  const at = name.lastIndexOf(' · ');
  return at === -1 ? name : name.slice(at + 3);
}

/**
 * The second line and the accessible name of a block: for a match whose pairs are
 * known "{categoría} · {pareja} vs {pareja}" (the name keeps the tournament too),
 * otherwise "Torneo · {nombre}" or "Canchas libres · {nombre}".
 */
function blockLabels(block: CourtEventBlock): { line: string; accessible: string } {
  if (!('match_id' in block)) {
    const label = t.events.blockLabel(block.name);
    return { line: label, accessible: label };
  }
  if (block.pair1 === null || block.pair2 === null) {
    const label = t.tournaments.blockLabel(block.name);
    return { line: label, accessible: label };
  }
  return {
    line: t.events.matchBlockLine(categoryOf(block.name), block.pair1, block.pair2),
    accessible: t.tournaments.blockLabel(t.events.matchBlockLine(block.name, block.pair1, block.pair2)),
  };
}

/**
 * The hours an open-play event or a tournament match holds on a court. Drawn
 * like a fixed block's bar, dashed, in a colour of its own per kind (blue for
 * an event, teal for a match) with a calendar icon in place of the repeat one.
 * Laid out like a booking's block: the hours on the first line, then "Canchas
 * libres · {nombre}", or "{categoría} · {pareja} vs {pareja}" for a match whose pairs are known
 * (else "Torneo · {nombre}"), truncated on one line. On the day after it started it draws the
 * tail of hours that ran past midnight.
 */
export function EventBlockBar({
  block,
  courtName,
  date,
  onSelect,
}: {
  block: CourtEventBlock;
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
  const isMatch = 'match_id' in block;
  const compactLabel = isMatch ? t.tournaments.blockLabelCompact : t.events.blockLabelCompact;
  const { line, accessible } = blockLabels(block);
  const ariaLabel = `${keepTogether(courtName)}, ${formatEventTimeRange(block.start_time, block.end_time)}, ${accessible}`;

  return (
    <button
      type="button"
      aria-label={ariaLabel}
      title={ariaLabel}
      onClick={onSelect}
      className={cn(
        'focus-self absolute inset-x-[3px] flex cursor-pointer flex-col overflow-hidden rounded-md border border-dashed px-1.5 py-0.5 text-left transition-[filter] focus-visible:brightness-150',
        isMatch ? HELD_BLOCK_STYLES.match : HELD_BLOCK_STYLES.event,
        // Hours that cross midnight are drawn square on the edge they cross.
        runsOn && 'rounded-b-none border-b-0',
        isTail && 'rounded-t-none border-t-0',
      )}
      style={{ top: `calc(${String(topPct)}% + 1px)`, height: `calc(${String(heightPct)}% - 2px)` }}
    >
      {tall ? (
        <>
          <span data-line className="score-text text-text-primary/80 text-[0.6875rem]">
            {formatEventTimeRange(block.start_time, block.end_time)}
          </span>
          <span className="flex min-w-0 items-center gap-1">
            <CalendarDays className="text-text-primary size-3 shrink-0" aria-hidden="true" />
            <span data-line className="text-text-primary truncate text-xs font-medium">
              {line}
            </span>
          </span>
        </>
      ) : (
        <span className="flex min-w-0 items-center gap-1">
          <CalendarDays className="text-text-primary size-3 shrink-0" aria-hidden="true" />
          <span className="text-text-primary truncate text-xs font-medium">{compactLabel}</span>
        </span>
      )}
    </button>
  );
}
