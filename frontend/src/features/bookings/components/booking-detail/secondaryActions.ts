/** Past this many secondary actions the detail keeps its header menu instead of footer buttons. */
const MENU_THRESHOLD = 3;

export interface BookingSecondaryActionFlags {
  canMarkNoShow: boolean;
  canCancel: boolean;
  /** Present while the fixed series can still be ended from this booking's date. */
  onEndSeries?: (() => void) | undefined;
}

function countSecondaryActions({ canMarkNoShow, canCancel, onEndSeries }: BookingSecondaryActionFlags) {
  return [canMarkNoShow, canCancel, !!onEndSeries].filter(Boolean).length;
}

/** One or two secondary actions are visible buttons; three or more go in the ⋯ menu. */
export function usesActionsMenu(flags: BookingSecondaryActionFlags) {
  return countSecondaryActions(flags) >= MENU_THRESHOLD;
}

/** Whether the footer carries the secondary actions as buttons: one or two of them. */
export function showsSecondaryButtons(flags: BookingSecondaryActionFlags) {
  const count = countSecondaryActions(flags);
  return count > 0 && count < MENU_THRESHOLD;
}
