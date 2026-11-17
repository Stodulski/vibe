/** The detail page body: header facts, the category and section selects, the section. */
export const TOURNAMENT_DETAIL_CLASS = 'space-y-4';
/**
 * The row of the category select and the section select. On a phone the section
 * is a fixed 7.25rem (enough for "Partidos" and its chevron) and the category takes
 * the rest: at 320px that leaves "Caballeros 6ta" untruncated. From `sm` both are compact.
 */
export const TOURNAMENT_SWITCH_ROW_CLASS = 'grid grid-cols-[minmax(0,1fr)_7.25rem] gap-3 sm:grid-cols-[16rem_8.5rem]';
/** One select of that row with its label; the row's columns set its width. */
export const TOURNAMENT_SWITCH_FIELD_CLASS = 'min-w-0 space-y-1.5';
/** The section switch: two segments to a row on a phone, one row of content-sized segments from `sm`. Never scrolls. */
export const SEGMENTED_TABS_CLASS = 'grid grid-cols-2 gap-2 sm:flex sm:flex-wrap';
