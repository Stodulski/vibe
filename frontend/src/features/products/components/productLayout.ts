/** Structure shared by the loaded product views and their skeletons, so a layout change touches both. */
export const PRODUCT_GRID_CLASS = 'grid grid-cols-1 gap-3 sm:grid-cols-2';
export const PRODUCT_CARD_CLASS =
  'border-border-subtle bg-bg-subtle flex w-full flex-col items-stretch rounded-2xl border p-4';
export const PRODUCT_TABLE_COLUMNS = ['w-32', 'w-24', 'w-16', 'w-12', 'w-16', 'w-16'] as const;
/** Back link, title and body of the detail page. */
export const PRODUCT_DETAIL_PAGE_CLASS = 'space-y-4';
export const PRODUCT_DETAIL_BODY_CLASS = 'space-y-4';
export const PRODUCT_DETAIL_FIGURES_CLASS = 'mt-4 grid grid-cols-2 gap-3';
export const PRODUCT_DETAIL_TOP_ROW_CLASS = 'flex items-start justify-between gap-3';
export const STOCK_MOVEMENT_ROW_CLASS =
  'border-border-subtle flex items-center justify-between gap-3 border-b py-3 last:border-b-0 sm:px-3.5';
