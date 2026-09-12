import { useCallback, useEffect, useRef, useState } from 'react';
import { drawChart, type ChartDatum, type ChartLayout } from './chart-draw';

/**
 * Owns the canvas element, redraws `data` on mount and on wrapper resize
 * (debounced 150ms), and exposes the refs needed for pointer hit-testing.
 * `resizeTick` increments on every debounced resize-triggered redraw — the
 * caller can depend on it (e.g. in a `useEffect`) to react to a resize, such
 * as dismissing a stale tooltip, without reading a ref during render.
 *
 * `redraw` keeps its `useCallback` where most of the app dropped theirs
 * (PERF-04): the effect below lists it as a dependency, and an effect's firing
 * is not something to hand to the React Compiler's inferred memoization.
 */
export function useCanvasChart(data: ChartDatum[]) {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const wrapperRef = useRef<HTMLDivElement>(null);
  const layoutRef = useRef<ChartLayout | undefined>(undefined);
  const rectRef = useRef<DOMRect | null>(null);
  const [resizeTick, setResizeTick] = useState(0);

  const redraw = useCallback(() => {
    const canvas = canvasRef.current;
    if (!canvas || data.length === 0) return;
    layoutRef.current = drawChart(canvas, data);
  }, [data]);

  useEffect(() => {
    redraw();
    rectRef.current = canvasRef.current?.getBoundingClientRect() ?? null;
    const el = wrapperRef.current;
    if (!el) return;
    let timer: ReturnType<typeof setTimeout>;
    const ro = new ResizeObserver(() => {
      clearTimeout(timer);
      timer = setTimeout(() => {
        redraw();
        setResizeTick((n) => n + 1);
        rectRef.current = canvasRef.current?.getBoundingClientRect() ?? null;
      }, 150);
    });
    ro.observe(el);
    return () => {
      clearTimeout(timer);
      ro.disconnect();
    };
  }, [redraw]);

  return { canvasRef, wrapperRef, layoutRef, rectRef, resizeTick };
}
