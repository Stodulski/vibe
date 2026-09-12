import { Ban } from 'lucide-react';
import { timeToMinutes } from '@/shared/lib/time';
import { formatTime } from '@/shared/lib/utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import { barPosition } from './timelineGeometry';
import { DAY_START_MIN, DAY_END_MIN } from './gridLayout';
import type { BlockedSlot } from '@/shared/types/api.types';

const t = ES_AR;

export function BlockedBlock({
  slot,
  courtName,
  onSelect,
}: {
  slot: BlockedSlot;
  courtName: string;
  onSelect: () => void;
}) {
  const startMin = timeToMinutes(slot.start_time);
  const endMin = timeToMinutes(slot.end_time);
  const { leftPct: topPct, widthPct: heightPct } = barPosition(startMin, endMin, DAY_START_MIN, DAY_END_MIN);
  const ariaLabel = `${courtName}, ${formatTime(slot.start_time)}–${formatTime(slot.end_time)}, ${t.bookings.blockedSlot}`;

  return (
    // Native button rather than role="button" + a hand-written key handler:
    // Enter and Space are the element's own behaviour (A11Y-02).
    <button
      type="button"
      aria-label={ariaLabel}
      title={ariaLabel}
      onClick={onSelect}
      // Solid, and the same `-border`/`-icon` pair the booking blocks use, so
      // a blocked slot reads as taken rather than as a different kind of
      // thing. `-bg` was tried and rejected there for being near-black: on a
      // dark grid it is indistinguishable from an empty cell.
      className="focus-self absolute inset-x-[3px] flex cursor-pointer items-center justify-center rounded-md border border-error-icon bg-error-border transition-[filter] focus-visible:brightness-150"
      style={{ top: `calc(${String(topPct)}% + 1px)`, height: `calc(${String(heightPct)}% - 2px)` }}
    >
      {/* White, not `error-text`: that salmon was picked to sit on the page's
          near-black, and on this solid red it drops to about 2:1. */}
      <Ban className="size-3.5 text-text-primary" aria-hidden="true" />
    </button>
  );
}
