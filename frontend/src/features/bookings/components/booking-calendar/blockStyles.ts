/**
 * The shape every block on the court grid shares: a button filling its slot,
 * rounded, padded and left aligned, with the focus ring and brightness the grid
 * uses. Bookings and fixed weekly bookings both draw from it, so they cannot
 * drift apart in size or spacing.
 */
export const BLOCK_BASE_CLASSES =
  'focus-self absolute inset-x-[3px] flex cursor-pointer flex-col overflow-hidden rounded-md border px-1.5 py-0.5 text-left transition-[filter] focus-visible:brightness-150';

/** The time range on a block's first line: dimmed white, in the mono score face. */
export const BLOCK_TIME_CLASSES = 'score-text text-text-primary/80 text-[0.6875rem]';

/** The client name on a block's second line. */
export const BLOCK_NAME_CLASSES = 'text-text-primary truncate text-xs font-medium';

// A block needs both lines once it is at least this tall; below that only the
// name fits. Derived from the duration, never from a measured box: jsdom has no layout.
export const TWO_LINE_MIN_HEIGHT_PX = 56;
