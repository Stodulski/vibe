import { catmullRom } from './catmullRom';

export interface ChartDatum {
  date: string;
  amount: number;
}

export interface ChartLayout {
  points: [number, number][];
  plotW: number;
  plotH: number;
  ml: number;
  mt: number;
  bottomY: number;
}

const CHART_MARGIN = { top: 4, right: 4, bottom: 6, left: 4 } as const;

export const STROKE_COLOR = '#14B8A6';
const GRID_COLOR = 'rgba(255,255,255,0.04)';

/**
 * Attempt to read a CSS variable value from the document root.
 * Falls back to the provided default when running outside a browser or
 * when the variable is not defined.
 */
export function cssVar(name: string, fallback: string): string {
  if (typeof document === 'undefined') return fallback;
  const v = getComputedStyle(document.documentElement).getPropertyValue(name).trim();
  return v || fallback;
}

/**
 * Compute the pixel-space layout (data points, plot dimensions, margins) for
 * a `w`x`h` canvas rendering `data`. Pure — no canvas/DOM side effects.
 * Returns null when there is no data to lay out.
 */
export function computeChartLayout(w: number, h: number, data: ChartDatum[]): ChartLayout | null {
  if (data.length === 0) return null;

  const ml = CHART_MARGIN.left;
  const mr = CHART_MARGIN.right;
  const mt = CHART_MARGIN.top;
  const xLabelHeight = 20;
  const mb = CHART_MARGIN.bottom + xLabelHeight;

  const plotW = w - ml - mr;
  const plotH = h - mt - mb;

  const amounts = data.map((d) => d.amount);
  const minVal = Math.min(...amounts);
  const maxVal = Math.max(...amounts);
  const rawRange = maxVal - minVal || 1;

  // Pad the scale beyond the actual min/max: the Catmull-Rom curve through a
  // sharp spike (or dip) surrounded by flat points can overshoot past the
  // data's own extremes (it isn't a monotone spline — no tangent clamping),
  // so mapping the raw max straight to the plot's top edge clipped that
  // overshoot against the canvas boundary. This headroom absorbs it.
  const pad = rawRange * 0.12;
  const paddedMin = minVal - pad;
  const range = rawRange + pad * 2;

  const points: [number, number][] = data.map((d, i) => {
    const x = ml + (data.length === 1 ? plotW / 2 : (i / (data.length - 1)) * plotW);
    const y = mt + plotH - ((d.amount - paddedMin) / range) * plotH;
    return [x, y];
  });

  return { points, plotW, plotH, ml, mt, bottomY: mt + plotH };
}

/** Draw the dashed horizontal grid lines (5 rows) across the plot area. */
export function drawGrid(ctx: CanvasRenderingContext2D, layout: ChartLayout): void {
  ctx.save();
  ctx.strokeStyle = GRID_COLOR;
  ctx.setLineDash([3, 3]);
  ctx.lineWidth = 1;
  const gridLines = 5;
  for (let i = 0; i <= gridLines; i++) {
    const y = layout.mt + (layout.plotH / gridLines) * i;
    ctx.beginPath();
    ctx.moveTo(layout.ml, y);
    ctx.lineTo(layout.ml + layout.plotW, y);
    ctx.stroke();
  }
  ctx.restore();
}

/**
 * Trace the smooth Catmull-Rom curve through `points` on `ctx`'s current
 * path (caller owns `beginPath`/`fill`/`stroke`). Degrades gracefully for
 * 0, 1 or 2 points.
 */
export function buildCurvePath(ctx: CanvasRenderingContext2D, points: [number, number][]): void {
  ctx.beginPath();
  const p0 = points[0];
  if (!p0) return;

  if (points.length === 1) {
    ctx.moveTo(p0[0], p0[1]);
    ctx.lineTo(p0[0], p0[1]);
    return;
  }

  const p1 = points[1];
  if (points.length === 2 && p1) {
    ctx.moveTo(p0[0], p0[1]);
    ctx.lineTo(p1[0], p1[1]);
    return;
  }

  ctx.moveTo(p0[0], p0[1]);
  const segments = 16;
  for (let i = 0; i < points.length - 1; i++) {
    const a = points[Math.max(i - 1, 0)];
    const b = points[i];
    const c = points[i + 1];
    const d = points[Math.min(i + 2, points.length - 1)];
    if (!a || !b || !c || !d) continue;
    for (let s = 1; s <= segments; s++) {
      const [cx, cy] = catmullRom(a, b, c, d, s / segments);
      ctx.lineTo(cx, cy);
    }
  }
}

/** Fill the area under the curve with the teal gradient. */
export function fillGradient(ctx: CanvasRenderingContext2D, points: [number, number][], layout: ChartLayout): void {
  const first = points[0];
  const last = points[points.length - 1];
  if (!first || !last) return;

  buildCurvePath(ctx, points);
  ctx.lineTo(last[0], layout.bottomY);
  ctx.lineTo(first[0], layout.bottomY);
  ctx.closePath();
  const grad = ctx.createLinearGradient(0, layout.mt, 0, layout.bottomY);
  grad.addColorStop(0, 'rgba(20,184,166,0.12)');
  grad.addColorStop(1, 'rgba(20,184,166,0)');
  ctx.fillStyle = grad;
  ctx.fill();
}

/** Stroke the curve line itself. */
export function strokeLine(ctx: CanvasRenderingContext2D, points: [number, number][]): void {
  buildCurvePath(ctx, points);
  ctx.strokeStyle = STROKE_COLOR;
  ctx.lineWidth = 1.5;
  ctx.lineJoin = 'round';
  ctx.stroke();
}

/** Draw the first/last date labels below the x-axis. */
export function drawAxisLabels(ctx: CanvasRenderingContext2D, layout: ChartLayout, data: ChartDatum[]): void {
  const first = data[0];
  if (!first) return;

  const textColor = cssVar('--color-text-tertiary', 'rgba(255,255,255,0.4)');
  ctx.fillStyle = textColor;
  ctx.font = '10px sans-serif';
  ctx.textBaseline = 'top';
  const labelY = layout.mt + layout.plotH + 8;

  ctx.textAlign = 'left';
  ctx.fillText(first.date, layout.ml, labelY);

  if (data.length > 1) {
    const last = data[data.length - 1];
    if (last) {
      ctx.textAlign = 'right';
      ctx.fillText(last.date, layout.ml + layout.plotW, labelY);
    }
  }
}

/**
 * Orchestrator: resize the canvas for devicePixelRatio, clear it, then draw
 * grid + gradient fill + curve + axis labels for `data`. Returns the
 * computed layout (used by the caller for hit-testing) or undefined when
 * nothing was drawn.
 */
export function drawChart(canvas: HTMLCanvasElement, data: ChartDatum[]): ChartLayout | undefined {
  const dpr = window.devicePixelRatio || 1;
  const rect = canvas.getBoundingClientRect();
  const w = rect.width;
  const h = rect.height;
  canvas.width = w * dpr;
  canvas.height = h * dpr;
  const ctx = canvas.getContext('2d');
  if (!ctx) return undefined;
  ctx.scale(dpr, dpr);
  ctx.clearRect(0, 0, w, h);

  const layout = computeChartLayout(w, h, data);
  if (!layout) return undefined;

  drawGrid(ctx, layout);
  fillGradient(ctx, layout.points, layout);
  strokeLine(ctx, layout.points);
  drawAxisLabels(ctx, layout, data);

  return layout;
}
