import { cn } from '@/shared/lib/utils';
import type { attendanceTone } from './attendance';

/**
 * The percentage and its bar, as one unit — shared by the card and the `xl`
 * table row so the two never draw the same number in two different shapes.
 */
export function AttendanceMeter({ pct, tone }: { pct: number; tone: ReturnType<typeof attendanceTone> }) {
  return (
    <div className="flex items-center gap-2">
      <span className={cn('score-text text-sm font-bold', tone.text)}>{pct}%</span>
      <div className="bg-bg-base h-1 min-w-0 flex-1 overflow-hidden rounded-full">
        <div className={cn('h-full rounded-full', tone.bar)} style={{ width: `${String(pct)}%` }} />
      </div>
    </div>
  );
}
