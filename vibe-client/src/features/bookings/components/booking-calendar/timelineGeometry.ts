/**
 * Pure maths for the resource timeline: converting minute-of-day values into
 * percentage geometry within the visible hour range. No DOM, no React — kept
 * separate so it can be unit-tested without rendering anything.
 */

export interface BarPosition {
  leftPct: number;
  widthPct: number;
  /**
   * The minutes of the bar that actually fall inside the range, after clamping.
   * Less than the booking's own duration whenever it runs off either edge.
   *
   * Returned rather than left to the caller to re-derive from `widthPct`,
   * because that round-trip does not survive: a 60-minute bar in a 1440-minute
   * day is 4.166666666666666%, and multiplying that back by the grid's 1344px
   * gives 55.999999999999986 — which failed a `>= 56` test and quietly dropped
   * the time label from every one-hour booking on the grid.
   */
  visibleMin: number;
}

const MIN_END_OFFSET = 30;

/**
 * Positions a bar as a percentage of the plot area's width. Clamps a start
 * before `rangeStartMin` to the range's left edge instead of a negative
 * `left`, and guards `endMin <= startMin` (bad data) by treating the end as
 * `startMin + 30` so the bar still renders instead of collapsing to nothing.
 */
export function barPosition(startMin: number, endMin: number, rangeStartMin: number, rangeEndMin: number): BarPosition {
  const rangeSpan = rangeEndMin - rangeStartMin;
  if (rangeSpan <= 0) return { leftPct: 0, widthPct: 0, visibleMin: 0 };

  const safeEndMin = endMin <= startMin ? startMin + MIN_END_OFFSET : endMin;
  const clampedStart = Math.max(startMin, rangeStartMin);
  const clampedEnd = Math.min(safeEndMin, rangeEndMin);

  const visibleMin = Math.max(0, clampedEnd - clampedStart);
  const leftPct = ((clampedStart - rangeStartMin) / rangeSpan) * 100;
  const widthPct = (visibleMin / rangeSpan) * 100;

  return { leftPct, widthPct, visibleMin };
}

/** Rounds the day's open/close minutes out to the nearest full hour. */
export function visibleRange(openMin: number, closeMin: number): { rangeStartMin: number; rangeEndMin: number } {
  return {
    rangeStartMin: Math.floor(openMin / 60) * 60,
    rangeEndMin: Math.ceil(closeMin / 60) * 60,
  };
}

/** Snaps a minute-of-day value down to the nearest 30-minute slot, clamped inside `[rangeStartMin, rangeEndMin]`. */
export function snapDownToSlot(min: number, rangeStartMin: number, rangeEndMin: number): number {
  const clamped = Math.min(Math.max(min, rangeStartMin), rangeEndMin);
  return Math.floor(clamped / 30) * 30;
}

/** Formats minutes-since-midnight as an "HH:MM" string, for hour-axis labels and slot keys. */
export function minutesToHHMM(min: number): string {
  const h = Math.floor(min / 60);
  const m = min % 60;
  return `${String(h).padStart(2, '0')}:${String(m).padStart(2, '0')}`;
}

/**
 * Same `endMin <= startMin` guard as `barPosition`, exposed separately so
 * callers that need the actual duration (e.g. to decide how much content a
 * block can fit) don't have to re-derive it from a percentage.
 */
export function clampedDurationMin(startMin: number, endMin: number): number {
  return endMin <= startMin ? MIN_END_OFFSET : endMin - startMin;
}
