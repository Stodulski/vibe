/** Structure shared by the loaded dashboard and its skeleton, so a layout change touches both. */
export const DASHBOARD_STACK_CLASS = 'flex flex-col gap-4 md:gap-6';
export const OPERATIONS_GRID_CLASS = 'grid grid-cols-1 gap-4 md:gap-6 lg:grid-cols-2';
/** Desktop-only trends block: revenue + client insights row, then the occupancy heatmap. */
export const TRENDS_SECTION_CLASS = 'mt-8 hidden md:flex md:flex-col md:gap-4 lg:gap-6';
export const TRENDS_ROW_CLASS = 'grid grid-cols-1 gap-4 md:gap-6 xl:grid-cols-5';
export const TRENDS_MAIN_CELL_CLASS = 'xl:col-span-3';
export const TRENDS_SIDE_CELL_CLASS = 'xl:col-span-2';
export const TREND_CARD_CLASS = 'border-border-subtle bg-bg-subtle rounded-2xl border p-4 sm:p-5';
/** A trend card that fills its grid cell. */
export const TREND_CARD_FILL_CLASS = `${TREND_CARD_CLASS} flex h-full flex-col`;
export const PUBLIC_LINK_BAR_CLASS = 'mb-3 flex items-center gap-2 sm:mb-4';
