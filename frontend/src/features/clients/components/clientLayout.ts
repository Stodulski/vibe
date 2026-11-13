/** Structure shared by the loaded clients list and its skeleton, so a layout change touches both. */
export const CLIENT_GRID_CLASS = 'grid grid-cols-1 gap-3 sm:grid-cols-2';
export const CLIENT_CARD_CLASS =
  'border-border-subtle bg-bg-subtle flex w-full flex-col items-stretch rounded-2xl border p-4';
export const CLIENT_CARD_STATS_CLASS = 'mt-4 grid grid-cols-2 gap-3';
export const CLIENT_TABLE_COLUMNS = ['w-32', 'w-24', 'w-36', 'w-10', 'w-10', 'w-12', 'w-20', 'w-16'] as const;
