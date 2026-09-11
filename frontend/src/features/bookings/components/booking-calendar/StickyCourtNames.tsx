import { GridHeaderRow } from './GridHeaderRow';
import { useStuck } from './useStuck';
import { HEADER_HEIGHT_PX } from './gridLayout';
import type { RefObject } from 'react';
import type { TimelineColumnData } from './useTimelineColumns';

/**
 * The court names, pinned to the top of the page while it scrolls.
 *
 * Deliberately outside the columns' horizontal scroller: an element with
 * `overflow-x: auto` is a scrollport on both axes, so `sticky` inside it
 * resolves against the scroller — which never moves vertically — and the names
 * would scroll away with the grid. Living outside, they keep pace with the
 * columns through a transform the scroll handler writes onto `trackRef`.
 *
 * The strip itself stays transparent. Only once pinned does each name get a
 * backing of its own, and only behind the name: a full-width band cut across
 * the bookings it was floating over.
 */
export function StickyCourtNames({
  columns,
  trackRef,
}: {
  columns: TimelineColumnData[];
  trackRef: RefObject<HTMLDivElement | null>;
}) {
  const { sentinelRef, stuck } = useStuck();

  return (
    <>
      <div ref={sentinelRef} aria-hidden="true" className="h-0" />
      {/* `top-16`, not `top-0`: the app shell's bar is fixed at the very top,
          so parking here would slide the names underneath it. */}
      <div className="sticky top-16 z-20 overflow-hidden" style={{ height: `${String(HEADER_HEIGHT_PX)}px` }}>
        <div ref={trackRef}>
          <GridHeaderRow columns={columns} stuck={stuck} />
        </div>
      </div>
    </>
  );
}
