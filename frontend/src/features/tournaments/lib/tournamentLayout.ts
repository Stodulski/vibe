/** Structure shared by the loaded tournament list and its skeleton, so a layout change touches both. */
export const TOURNAMENT_LIST_CLASS = 'grid grid-cols-1 gap-3 sm:grid-cols-2';
export const TOURNAMENT_CARD_CLASS =
  'border-border-subtle bg-bg-subtle flex w-full flex-col items-stretch gap-1.5 rounded-2xl border p-4 text-left';
export const TOURNAMENT_CARD_TOP_ROW_CLASS = 'flex items-start justify-between gap-2';
/** Tabs, then the list: shared by the page and its skeleton. */
export const TOURNAMENTS_PAGE_CLASS = 'space-y-4';
/** The detail page body: header facts, category selector, section tabs, the section. */
export const TOURNAMENT_DETAIL_CLASS = 'space-y-4';
/** A row of choices that scrolls sideways inside itself, never the page, when it does not fit. */
export const SEGMENTED_TABS_CLASS = 'flex gap-2 overflow-x-auto pb-1';
