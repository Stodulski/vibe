import { ROUTE_TABS_STRIP_CLASS } from '@/shared/components/common/routeTabsLayout';

/** The category strip: the Eventos underline strip, wrapping onto more lines when the names do not fit. It never scrolls sideways. */
export const CATEGORY_TABS_STRIP_CLASS = `${ROUTE_TABS_STRIP_CLASS} flex-wrap`;
/** One category tab: 44px tall like the Eventos tabs. Its label stays on one line; a name wider than the screen is cut with an ellipsis. */
export const CATEGORY_TAB_CLASS =
  'flex h-11 max-w-full items-center border-b-2 border-transparent px-3 text-sm font-medium whitespace-nowrap transition-colors';
/** The label inside a category tab: it may shrink, so the tab never grows past the screen. */
export const CATEGORY_TAB_LABEL_CLASS = 'min-w-0 truncate';
/** The row of the category select and the section select: side by side, the same width each, from four categories. */
export const CATEGORY_SWITCH_ROW_CLASS = 'grid grid-cols-2 gap-3';
