/** Structure shared by the loaded courts list and its skeleton, so a layout change touches both. */
export const COURT_GRID_CLASS = 'grid w-full grid-cols-1 gap-3 lg:grid-cols-2';
export const COURT_CARD_CLASS = 'bg-bg-subtle relative flex h-full flex-col overflow-hidden rounded-2xl border';
export const COURT_CARD_BODY_CLASS = 'flex-1 p-4 sm:p-5';
export const COURT_CARD_ACTIONS_CLASS = 'border-border-subtle flex items-center gap-2 border-t px-4 py-3 sm:px-5';
/** Table widths from `xl`; below it the cards show. */
export const COURT_TABLE_COLUMNS = ['w-24', 'w-16', 'w-40', 'w-20', 'w-10', 'w-16'] as const;
