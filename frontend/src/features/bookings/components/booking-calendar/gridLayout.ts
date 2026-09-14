/**
 * Shared layout constants for the vertical court-time grid: time runs
 * top-to-bottom over the whole day (00:00–24:00) independent of the
 * complex's opening hours — a deliberate product decision — and each court
 * is a column instead of a row.
 */
export const SLOT_MINUTES = 30;
export const SLOTS_PER_DAY = 48; // 24h / 30min
export const SLOT_HEIGHT_PX = 28;
export const HOUR_HEIGHT_PX = SLOT_HEIGHT_PX * 2;
export const GRID_HEIGHT_PX = SLOTS_PER_DAY * SLOT_HEIGHT_PX; // 1344

export const DAY_START_MIN = 0;
export const DAY_END_MIN = 24 * 60;

/**
 * Width of the time gutter on the left, as a class so both the gutter and the
 * spacer above it stay in step. Narrower on phones, where those pixels are the
 * difference between seeing a booking's details and not.
 */
export const GUTTER_CLASS = 'w-10 shrink-0 sm:w-11';
/**
 * The narrowest a court column is ever drawn. Whatever doesn't fit at this
 * width is reached by scrolling the columns sideways, so a booking is never
 * squeezed below the width that makes its details readable.
 *
 * It is a floor, not the width: see `resolveColumnWidthPx`.
 */
export const MIN_COLUMN_WIDTH_PX = 200;

/**
 * How wide each court column is drawn, given the space the columns have.
 *
 * A club with one court used to get a 200px strip against the side of a phone
 * and dead space for the rest of the viewport, because the width was a
 * constant. Few columns now share the space out between them; many still take
 * the minimum each and overflow into the horizontal scroller, unchanged.
 *
 * `availableWidth` is the plot area's own width — the hour gutter is a sibling
 * outside it (see `CourtTimeGrid`), so there is nothing to subtract here.
 * It is 0 before the element is measured, and on a server or in a DOM without
 * `ResizeObserver`; every one of those falls back to the minimum.
 *
 * Floored so the columns can never total more than the space they were given:
 * a fraction of a pixel over is enough to arm the scroller on a grid that has
 * nothing more to show.
 */
export function resolveColumnWidthPx(availableWidth: number, columnCount: number): number {
  if (availableWidth <= 0 || columnCount <= 0) return MIN_COLUMN_WIDTH_PX;
  return Math.max(MIN_COLUMN_WIDTH_PX, Math.floor(availableWidth / columnCount));
}
/** Height of the court-name row above the grid. */
export const HEADER_HEIGHT_PX = 36;

/** Converts minutes-since-midnight into a pixel offset within the grid's full height. */
export function minutesToPx(min: number): number {
  return (min / (DAY_END_MIN - DAY_START_MIN)) * GRID_HEIGHT_PX;
}
