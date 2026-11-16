import type { Booking } from '@/shared/types/api.types';

/** Past this many secondary actions the detail keeps its header menu instead of footer buttons. */
const MENU_THRESHOLD = 3;

export interface BookingSecondaryActionFlags {
  canMarkNoShow: boolean;
  /** The hours are over and nobody marked yet whether the client came. */
  canMarkAttended: boolean;
  canCancel: boolean;
  /** Present while the fixed series can still be ended from this booking's date. */
  onEndSeries?: (() => void) | undefined;
}

/** The secondary actions with their handlers, as the footer buttons and the header menu both take them. */
export interface BookingSecondaryActionProps extends BookingSecondaryActionFlags {
  booking: Booking;
  onNoShow: (booking: Booking) => void;
  onAttended: (booking: Booking) => void;
  onCancel: (booking: Booking) => void;
  /** The booking is one occurrence of a fixed weekly series: cancelling only covers this date. */
  isSeries?: boolean | undefined;
}

function countSecondaryActions({
  canMarkNoShow,
  canMarkAttended,
  canCancel,
  onEndSeries,
}: BookingSecondaryActionFlags) {
  return [canMarkNoShow, canMarkAttended, canCancel, !!onEndSeries].filter(Boolean).length;
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
