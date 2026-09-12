import { cn } from '@/shared/lib/utils';
import { GRID_HEIGHT_PX, GUTTER_CLASS, HOUR_HEIGHT_PX } from './gridLayout';

// 01:00–23:00. The day's two edges are skipped: their gridlines are the grid's
// own top and bottom borders, so a label centred on either sits half outside.
const HOURS = Array.from({ length: 23 }, (_, i) => i + 1);

/** Sticky-left hour ruler: one label per hour (not per 30-minute row), each aligned to its hour's gridline. */
export function GridHourGutter() {
  return (
    <div className={cn('relative', GUTTER_CLASS)} style={{ height: `${String(GRID_HEIGHT_PX)}px` }}>
      {HOURS.map((h) => (
        // Flush left, at every width. The gutter's own left edge is the page's
        // content edge — the same line the page title and the date strip start
        // on — so the hours read as part of that column rather than as a label
        // hanging off the grid. Right-aligning them closed the gap to the grid
        // but broke that line, which is the more visible of the two.
        //
        // The gap that remains is the gutter's unused width, ten pixels. It
        // used to be forty-six: an arrow shortcut held a lane of its own to
        // the gutter's right, and that lane is gone with the arrows.
        <div
          key={h}
          className="score-text text-text-tertiary absolute left-0 -translate-y-1/2 text-[0.6875rem]"
          style={{ top: `${String(h * HOUR_HEIGHT_PX)}px` }}
        >
          {String(h).padStart(2, '0')}:00
        </div>
      ))}
    </div>
  );
}
