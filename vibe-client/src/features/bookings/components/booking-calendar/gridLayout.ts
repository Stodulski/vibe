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
 * Every court column is this wide, at every viewport. Whatever doesn't fit
 * is reached by scrolling the columns sideways, so a booking occupies the
 * same width on a phone as on a desktop instead of being squeezed.
 */
export const COLUMN_WIDTH_PX = 200;
/** Height of the court-name row above the grid. */
export const HEADER_HEIGHT_PX = 36;

/** Converts minutes-since-midnight into a pixel offset within the grid's full height. */
export function minutesToPx(min: number): number {
  return (min / (DAY_END_MIN - DAY_START_MIN)) * GRID_HEIGHT_PX;
}
