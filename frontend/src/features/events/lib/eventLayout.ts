/** Structure shared by the loaded event list and its skeleton, so a layout change touches both. */
export const EVENT_LIST_CLASS = 'grid grid-cols-1 gap-3 sm:grid-cols-2';
export const EVENT_CARD_CLASS =
  'border-border-subtle bg-bg-subtle flex w-full flex-col items-stretch gap-1.5 rounded-2xl border p-4 text-left';
export const EVENT_CARD_TOP_ROW_CLASS = 'flex items-start justify-between gap-2';
export const EVENT_CARD_BOTTOM_ROW_CLASS = 'mt-1.5 flex items-center justify-between gap-2';
/** The scrolling body of the event sheet and its skeleton. */
export const EVENT_DETAIL_BODY_CLASS = 'min-h-0 flex-1 space-y-4 overflow-y-auto px-4 pt-4 pb-8 sm:px-6';
/** Tabs, then the list: shared by the page and its skeleton. */
export const EVENTS_PAGE_CLASS = 'space-y-4';
