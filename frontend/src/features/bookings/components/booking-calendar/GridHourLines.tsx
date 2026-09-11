import { HOUR_HEIGHT_PX, SLOTS_PER_DAY } from './gridLayout';

// 01:00–23:00. The day's two edges are left out: their lines coincide with the
// grid's own top and bottom borders, so drawing them repeated the border — and
// the 24:00 one landed 1px past the box, giving the calendar 2px of vertical
// overflow and, since a horizontally scrollable box is never `overflow-y:
// visible`, a vertical scroll axis of its own.
const HOUR_TOPS = Array.from({ length: SLOTS_PER_DAY / 2 - 1 }, (_, i) => (i + 1) * HOUR_HEIGHT_PX);

/**
 * Horizontal gridlines spanning every court column, at each hour only.
 * Half-hour lines doubled the ruling for a boundary the gutter doesn't even
 * label, and turned an empty day into a field of stripes.
 */
export function GridHourLines() {
  return (
    <div className="pointer-events-none absolute inset-0">
      {HOUR_TOPS.map((top) => (
        <div
          key={`hour-${String(top)}`}
          className="absolute inset-x-0 h-px bg-border-subtle"
          style={{ top: `${String(top)}px` }}
        />
      ))}
    </div>
  );
}
