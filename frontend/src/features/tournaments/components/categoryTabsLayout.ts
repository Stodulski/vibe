import { ROUTE_TABS_STRIP_CLASS } from '@/shared/components/common/routeTabsLayout';

/** The category strip: the Eventos underline strip, scrolling inside itself when the names do not fit at 320 px. */
export const CATEGORY_TABS_STRIP_CLASS = `${ROUTE_TABS_STRIP_CLASS} overflow-x-auto`;
/** One category tab: 44px tall like the Eventos tabs, never shrinking, so a long name keeps its width. */
export const CATEGORY_TAB_CLASS =
  'flex h-11 shrink-0 items-center border-b-2 border-transparent px-3 text-sm font-medium whitespace-nowrap transition-colors';
