/** The detail page body: header facts, the category and section selects, the section. */
export const TOURNAMENT_DETAIL_CLASS = 'space-y-4';
/**
 * The row of the category select and the section select. On a phone they stack, so
 * the longest section ("Posiciones por zona") and category names are never cut; from
 * `sm` they sit side by side at a fixed width.
 */
export const TOURNAMENT_SWITCH_ROW_CLASS = 'grid grid-cols-1 gap-3 sm:grid-cols-[16rem_12rem]';
/** One select of that row with its label; the row's columns set its width. */
export const TOURNAMENT_SWITCH_FIELD_CLASS = 'min-w-0 space-y-1.5';
/** The section switch: two segments to a row on a phone, one row of content-sized segments from `sm`. Never scrolls. */
export const SEGMENTED_TABS_CLASS = 'grid grid-cols-2 gap-2 sm:flex sm:flex-wrap';
