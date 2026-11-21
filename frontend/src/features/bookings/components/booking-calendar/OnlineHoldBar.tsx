import { Globe } from 'lucide-react';
import { cn, formatInstantTime } from '@/shared/lib/utils';
import { keepTogether } from '@/shared/lib/keepTogether';
import { formatEventTimeRange } from '@/shared/lib/eventDates';
import { ES_AR } from '@/shared/i18n/es_AR';
import { barPosition } from './timelineGeometry';
import { heldBlockRange } from './fixedBlocks';
import { HELD_BLOCK_STYLES } from './heldBlockStyles';
import { BLOCK_TIME_CLASSES, TWO_LINE_MIN_HEIGHT_PX } from './blockStyles';
import { DAY_START_MIN, DAY_END_MIN, SLOT_HEIGHT_PX, SLOT_MINUTES } from './gridLayout';
import type { OnlineHold } from '../../schemas/onlineHold.schema';

const t = ES_AR;

/**
 * Hours a visitor is booking online right now: held from the storefront and
 * not paid yet. Not a button: there is nothing of the visitor's to open, and
 * nothing the owner can do with the cell until it frees up. It says when that
 * is, so a cell that would otherwise look free explains itself.
 */
export function OnlineHoldBar({ hold, courtName, date }: { hold: OnlineHold; courtName: string; date: string }) {
  const range = heldBlockRange(hold, date);
  if (!range) return null;
  const { startMin, endMin, isTail, runsOn } = range;
  const {
    leftPct: topPct,
    widthPct: heightPct,
    visibleMin,
  } = barPosition(startMin, endMin, DAY_START_MIN, DAY_END_MIN);
  const tall = (visibleMin / SLOT_MINUTES) * SLOT_HEIGHT_PX >= TWO_LINE_MIN_HEIGHT_PX;
  const hours = formatEventTimeRange(hold.start_time, hold.end_time);
  const freesAt = t.bookings.onlineHoldFreesAt(formatInstantTime(hold.frees_at));
  const label = `${keepTogether(courtName)}, ${hours}, ${t.bookings.onlineHold}. ${freesAt}`;

  return (
    <div
      data-online-hold
      role="img"
      aria-label={label}
      title={label}
      className={cn(
        'absolute inset-x-[3px] flex flex-col overflow-hidden rounded-md border border-dashed px-1.5 py-0.5 text-left',
        HELD_BLOCK_STYLES.online,
        runsOn && 'rounded-b-none border-b-0',
        isTail && 'rounded-t-none border-t-0',
      )}
      style={{ top: `calc(${String(topPct)}% + 1px)`, height: `calc(${String(heightPct)}% - 2px)` }}
    >
      {tall && (
        <span data-line className={BLOCK_TIME_CLASSES}>
          {hours}
        </span>
      )}
      <span className="flex min-w-0 items-center gap-1">
        <Globe className="text-text-primary size-3 shrink-0" aria-hidden="true" />
        <span data-line className="text-text-primary truncate text-xs font-medium">
          {t.bookings.onlineHold}
        </span>
      </span>
      {tall && (
        <span data-line className="text-text-primary/90 truncate text-xs">
          {freesAt}
        </span>
      )}
    </div>
  );
}
