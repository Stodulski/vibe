/** Shared by `SectionNav`, `SectionTabs`, their skeletons and the pages that place them. */
export const SECTION_LAYOUT_CLASS = 'flex flex-col gap-3 sm:gap-4 lg:flex-row';
/**
 * The section's content: a card from `lg`, bare below it. 39rem = the 36rem
 * content column plus the panel's own 1.5rem of padding on each side.
 */
export const SECTION_PANEL_CLASS =
  'lg:border-border-subtle lg:bg-bg-subtle min-w-0 flex-1 lg:max-w-[39rem] lg:rounded-2xl lg:border lg:p-6';
export const SECTION_NAV_CLASS = 'hidden w-52 shrink-0 lg:block';
export const SECTION_NAV_ITEM_CLASS = 'flex h-12 w-full items-center gap-3 rounded-xl px-3';
/** Full-bleed to the page edge: matches `main`'s `px-3 sm:px-6`. */
export const SECTION_TABS_CLASS = '-mx-3 px-3 sm:-mx-6 sm:px-6 lg:hidden';
/**
 * One tab of the strip. Below `sm` the icon sits above the label so four tabs
 * fit a 320px phone whole; `min-w-0` lets a tab shrink below its label's width
 * instead of pushing the page sideways.
 */
export const SECTION_TAB_CLASS =
  'flex h-12 min-w-0 flex-1 flex-col items-center justify-center gap-0.5 border-b-2 px-1 sm:flex-row sm:gap-2 sm:px-2';
