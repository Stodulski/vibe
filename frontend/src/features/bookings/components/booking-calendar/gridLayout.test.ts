import { describe, it, expect } from 'vitest';
import { MIN_COLUMN_WIDTH_PX, resolveColumnWidthPx } from './gridLayout';

/**
 * `availableWidth` here is the plot area's own width. The hour gutter is a
 * sibling of that box rather than a child of it (see `CourtTimeGrid`), so the
 * figure reaching this function has already had the gutter taken out of it by
 * the layout itself — there is no subtraction to get wrong.
 */
describe('resolveColumnWidthPx', () => {
  it('gives a lone court the whole plot area instead of a 200px strip', () => {
    // The bug this exists for: one court on a phone left ~40% of the viewport
    // empty because the width was the constant, whatever the space.
    expect(resolveColumnWidthPx(600, 1)).toBe(600);
  });

  it('shares the space out between a few courts', () => {
    expect(resolveColumnWidthPx(600, 2)).toBe(300);
  });

  it('holds at the minimum once the courts stop fitting, so the grid scrolls', () => {
    // 600 / 5 = 120, below the floor: five columns of 200 overflow a 600px
    // area on purpose, which is what arms the horizontal scroller.
    expect(resolveColumnWidthPx(600, 5)).toBe(MIN_COLUMN_WIDTH_PX);
  });

  it('never totals more than the space it was given', () => {
    // 1000 / 3 is not an integer; rounding up would arm the scroller on a
    // grid with nothing more to show.
    const width = resolveColumnWidthPx(1000, 3);
    expect(width).toBe(333);
    expect(width * 3).toBeLessThanOrEqual(1000);
  });

  it('falls back to the minimum before the element has been measured', () => {
    // Width is 0 on the first render, on the server, and in a DOM without
    // ResizeObserver. None of those may collapse the columns to nothing.
    expect(resolveColumnWidthPx(0, 1)).toBe(MIN_COLUMN_WIDTH_PX);
    expect(resolveColumnWidthPx(-1, 3)).toBe(MIN_COLUMN_WIDTH_PX);
  });

  it('falls back to the minimum when there are no columns to divide by', () => {
    expect(resolveColumnWidthPx(600, 0)).toBe(MIN_COLUMN_WIDTH_PX);
  });
});
