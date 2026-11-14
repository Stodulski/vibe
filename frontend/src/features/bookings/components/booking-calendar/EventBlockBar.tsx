import { CalendarDays } from 'lucide-react';
import { format } from 'date-fns/format';
import { parseISO } from 'date-fns/parseISO';
import { es } from 'date-fns/locale/es';
import { timeToMinutes } from '@/shared/lib/time';
import { cn, formatTime } from '@/shared/lib/utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import { barPosition } from './timelineGeometry';
import { fixedBlockEndMin } from './fixedBlocks';
import { DAY_START_MIN, DAY_END_MIN, SLOT_HEIGHT_PX, SLOT_MINUTES } from './gridLayout';
import type { EventBlock } from '@/shared/types/api.types';

const t = ES_AR;

// Same rule as `FixedBlockBar`: the event's name under the label once the block
// is this tall, the short label alone below it.
const TWO_LINE_MIN_HEIGHT_PX = 56;

/**
 * The hours an open-play event holds on a court. Drawn like a fixed block's
 * bar, in the same dashed red family because both are hours taken that were
 * not a booking, with a calendar icon in place of the repeat one. The label
 * reads "Canchas libres · {día} {hora}", so it names which event it is by when.
 */
export function EventBlockBar({
  block,
  courtName,
  onSelect,
}: {
  block: EventBlock;
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
  const weekday = format(parseISO(block.date), 'EEEE', { locale: es });
  const label = t.events.blockLabel(weekday, formatTime(block.start_time));
  const ariaLabel = `${courtName}, ${formatTime(block.start_time)}–${formatTime(block.end_time)}, ${label}, ${block.name}`;

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
          <span className="flex min-w-0 items-start gap-1">
            <CalendarDays className="text-text-primary mt-0.5 size-3 shrink-0" aria-hidden="true" />
            <span className="text-text-primary min-w-0 text-xs leading-tight font-medium break-words">{label}</span>
          </span>
          <span className="text-text-primary/80 truncate text-xs">{block.name}</span>
        </>
      ) : (
        <span className="flex min-w-0 items-center gap-1">
          <CalendarDays className="text-text-primary size-3 shrink-0" aria-hidden="true" />
          <span className="text-text-primary truncate text-xs font-medium">{t.events.blockLabelCompact}</span>
        </span>
      )}
    </button>
  );
}
