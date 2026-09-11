import { describe, expect, it } from 'vitest';
import { computeChartLayout, cssVar, type ChartDatum } from './chart-draw';

// Unit tests for the pure layout function extracted from `drawChart` in
// slice 10 (max-lines decomposition). These lock in the exact pixel-space
// math that previously lived inline inside `drawChart`, captured from the
// unmodified implementation before the split.
describe('computeChartLayout', () => {
  const data: ChartDatum[] = [
    { date: '01/08', amount: 100 },
    { date: '02/08', amount: 300 },
    { date: '03/08', amount: 200 },
  ];

  it('returns null for empty data', () => {
    expect(computeChartLayout(200, 100, [])).toBeNull();
  });

  it('computes plot dimensions accounting for margins and the x-axis label band', () => {
    const layout = computeChartLayout(200, 100, data);
    expect(layout).not.toBeNull();
    // ml=4, mr=4, mt=4, xLabelHeight=20, mb=6+20=26
    expect(layout?.plotW).toBe(192);
    expect(layout?.plotH).toBe(70);
    expect(layout?.ml).toBe(4);
    expect(layout?.mt).toBe(4);
    expect(layout?.bottomY).toBe(74);
  });

  it('maps a single datum to the horizontal center of the plot area', () => {
    const layout = computeChartLayout(200, 100, [{ date: '01/08', amount: 50 }]);
    expect(layout?.points).toHaveLength(1);
    expect(layout?.points[0]?.[0]).toBe(4 + 192 / 2);
  });

  // The scale is padded 12% beyond the raw min/max on each side (see
  // computeChartLayout) so a Catmull-Rom curve overshooting a sharp spike or
  // dip has headroom instead of clipping against the canvas edge — so the
  // min/max points land just inside the plot's bottom/top, not flush against them.
  it('places the min-value point near the bottom and max-value point near the top of the plot, padded off the edges', () => {
    const layout = computeChartLayout(200, 100, data);
    const points = layout?.points ?? [];
    expect(points[0]?.[1]).toBeCloseTo(67.2258, 3); // amount=100 (min)
    expect(points[1]?.[1]).toBeCloseTo(10.7742, 3); // amount=300 (max)
  });

  it('spaces points evenly along the x-axis for multi-point data', () => {
    const layout = computeChartLayout(200, 100, data);
    const points = layout?.points ?? [];
    expect(points[0]?.[0]).toBe(4);
    expect(points[2]?.[0]).toBe(4 + 192);
    expect(points[1]?.[0]).toBeCloseTo(4 + 192 / 2, 5);
  });
});

describe('cssVar', () => {
  it('falls back when document is unavailable or the variable is unset', () => {
    // jsdom provides `document`, but the custom property is never defined,
    // so getPropertyValue returns '' and the fallback is used.
    expect(cssVar('--not-a-real-variable', 'fallback-value')).toBe('fallback-value');
  });
});
