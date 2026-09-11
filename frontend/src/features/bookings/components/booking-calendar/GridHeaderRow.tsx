import { cn } from '@/shared/lib/utils';
import { COLUMN_WIDTH_PX, HEADER_HEIGHT_PX } from './gridLayout';
import type { TimelineColumnData } from './useTimelineColumns';

/**
 * Court names, one fixed-width cell per column, sized to match the columns
 * they label so the two stay aligned as the grid scrolls sideways.
 *
 * `stuck` only changes the backing: pinned, each name needs something opaque
 * behind it or the bookings scroll through the text — but the backing is a
 * pill around the name itself, not a band across the whole strip.
 */
export function GridHeaderRow({ columns, stuck }: { columns: TimelineColumnData[]; stuck: boolean }) {
  return (
    <div className="flex" style={{ height: `${String(HEADER_HEIGHT_PX)}px` }}>
      {columns.map((column) => (
        <div
          key={column.courtId}
          style={{ width: `${String(COLUMN_WIDTH_PX)}px` }}
          className="flex shrink-0 items-center justify-center px-2"
        >
          <span
            className={cn(
              'truncate rounded-full px-3 py-0.5 text-sm font-medium text-text-primary transition-colors',
              stuck && 'bg-bg-elevated',
            )}
          >
            {column.courtName}
          </span>
        </div>
      ))}
    </div>
  );
}
