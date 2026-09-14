import { useCallback } from 'react';
import { useElementWidth } from '@/shared/hooks/useElementWidth';
import { useMediaQuery } from '@/shared/hooks/useMediaQuery';
import { MIN_COLUMN_WIDTH_PX, resolveColumnWidthPx } from './gridLayout';
import type { RefObject } from 'react';

/**
 * How wide each court column should be drawn, measured from the plot area
 * itself rather than guessed from a breakpoint.
 *
 * The frame it measures is the box the columns live in; the hour gutter is
 * that box's sibling, not its child (see `CourtTimeGrid`), so the width read
 * here is already the space the columns actually have — there is no gutter to
 * subtract, and nothing to keep in step with the gutter's own responsive
 * class.
 *
 * It returns a callback ref because the frame already carries one from
 * `useColumnScroller`, which needs the node for the edge fades. Both are fed
 * from here so the grid's JSX keeps a single `ref`.
 *
 * `useElementWidth` reports 0 until a ResizeObserver measures the node —
 * on the server, and in a DOM without one — and `resolveColumnWidthPx` reads
 * that as "fall back to the minimum".
 *
 * Only a phone stretches. Below `sm` the frame is the whole viewport and a
 * lone 200px column beside 40% of dead space reads as broken; from `sm` up
 * the owner wants one width per court, always — a single court drawn 1500px
 * wide on a desktop is the same mistake in the other direction, and the
 * columns are what makes a day with three courts scan as three.
 */
const STRETCH_BELOW_QUERY = '(max-width: 639.98px)';
export function useColumnWidth(frameRef: RefObject<HTMLDivElement | null>, columnCount: number) {
  const { ref: measureFrame, width: frameWidth } = useElementWidth();
  const stretch = useMediaQuery(STRETCH_BELOW_QUERY);

  const attachFrame = useCallback(
    (node: HTMLDivElement | null) => {
      frameRef.current = node;
      measureFrame(node);
    },
    [frameRef, measureFrame],
  );

  return { attachFrame, columnWidth: stretch ? resolveColumnWidthPx(frameWidth, columnCount) : MIN_COLUMN_WIDTH_PX };
}
