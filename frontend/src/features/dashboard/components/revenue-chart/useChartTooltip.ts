import { useCallback, useState, type RefObject } from 'react';
import { formatPrice } from '@/shared/lib/utils';
import type { ChartDatum, ChartLayout } from './chart-draw';
import { measureText } from './measureText';

export interface ChartTooltipState {
  x: number;
  y: number;
  datum: ChartDatum;
  /** Bottom of the plot area at the moment this tooltip was set — read from
   * `layoutRef` inside the pointer event handler (never during render). */
  bottomY: number;
}

export interface ChartTooltipStyle {
  left: number;
  top: number;
}

interface TrackedTooltipState extends ChartTooltipState {
  /** `resizeTick` at the moment this tooltip was set — used to derive
   * staleness after a resize without a dedicated effect (see below). */
  resizeTick: number;
  /** Position computed inside the pointer event handler (never during
   * render) — refs must not be read during render or in `useMemo`. */
  style: ChartTooltipStyle;
}

function findNearestPointIndex(points: [number, number][], mx: number): number {
  let nearest = 0;
  let nearestDist = Infinity;
  for (let i = 0; i < points.length; i++) {
    const p = points[i];
    if (!p) continue;
    const dist = Math.abs(p[0] - mx);
    if (dist < nearestDist) {
      nearestDist = dist;
      nearest = i;
    }
  }
  return nearest;
}

/**
 * Tracks pointer hover over the chart canvas, finds the nearest data point,
 * and computes a viewport-safe tooltip position (clamped so it never
 * overflows the wrapper).
 */
export function useChartTooltip(
  data: ChartDatum[],
  layoutRef: RefObject<ChartLayout | undefined>,
  rectRef: RefObject<DOMRect | null>,
  resizeTick: number,
) {
  const [rawTooltip, setRawTooltip] = useState<TrackedTooltipState | null>(null);

  // A tooltip set before the most recent resize is stale (its pixel
  // coordinates no longer match the redrawn canvas) — derive that instead
  // of clearing it imperatively from an effect, so no state resides for a
  // render longer than it stays valid.
  const tooltip = rawTooltip?.resizeTick === resizeTick ? rawTooltip : null;

  const handlePointerMove = useCallback(
    (e: React.PointerEvent<HTMLCanvasElement>) => {
      // Refs are read here, inside an event handler — never during render.
      const layout = layoutRef.current;
      const rect = rectRef.current;
      if (!layout || !rect) return;
      const mx = e.clientX - rect.left;
      const nearest = findNearestPointIndex(layout.points, mx);
      const pt = layout.points[nearest];
      const datum = data[nearest];
      if (!pt || !datum) return;

      const wrapperW = rect.width;
      const tooltipText = `${datum.date}  ${formatPrice(datum.amount)}`;
      const estW = measureText(tooltipText, '13px sans-serif') + 32; // padding
      let left = pt[0];
      if (left + estW / 2 > wrapperW) left = wrapperW - estW / 2 - 4;
      if (left - estW / 2 < 0) left = estW / 2 + 4;

      setRawTooltip({
        x: pt[0],
        y: pt[1],
        datum,
        bottomY: layout.bottomY,
        resizeTick,
        style: { left, top: pt[1] - 44 },
      });
    },
    [data, layoutRef, rectRef, resizeTick],
  );

  const handlePointerLeave = useCallback(() => {
    setRawTooltip(null);
  }, []);

  const tooltipStyle = tooltip?.style;

  return { tooltip, tooltipStyle, handlePointerMove, handlePointerLeave };
}
