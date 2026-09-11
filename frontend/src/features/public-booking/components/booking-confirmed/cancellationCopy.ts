import { ES_AR } from '@/shared/i18n/es_AR';
import { formatDeadline } from '@/shared/lib/utils';
import type { BookingInfo } from './types';

const t = ES_AR;

/**
 * The cancellation line under the confirmed booking's details — three
 * different sentences depending on `cancellation` (from `GET /book/status`),
 * or the old generic "before X hours" copy when that field isn't there yet
 * (an older server, or a `BookingInfo` built entirely from the sessionStorage
 * cache). Null hides the line entirely: either the API said the booking
 * can't be cancelled any more, or there is nothing to say about it at all.
 */
export function cancellationLineText(bookingInfo: BookingInfo): string | null {
  const cancellation = bookingInfo.cancellation;
  if (cancellation) {
    if (!cancellation.canCancel) return null;
    if (cancellation.canRefundNow) {
      return cancellation.refundDeadline
        ? `${t.publicBooking.cancelRefundUntilPrefix} ${formatDeadline(cancellation.refundDeadline)}${t.publicBooking.cancelRefundUntilSuffix}`
        : t.publicBooking.cancelRefundUntilTurnStart;
    }
    return t.publicBooking.cancelRefundWindowPassed;
  }

  if (bookingInfo.cancellationHours > 0) {
    return `${t.publicBooking.cancellationPolicyBefore} ${String(bookingInfo.cancellationHours)}h ${t.publicBooking.cancellationPolicyAfter}`;
  }

  return null;
}

/**
 * Whether the "Cancelar reserva" button itself should show. False only when
 * the API explicitly said `can_cancel: false` — absent `cancellation` (old
 * server, or no booking info at all) keeps the button visible, same as
 * before this field existed.
 */
export function canCancelBooking(bookingInfo: BookingInfo | null): boolean {
  return bookingInfo?.cancellation ? bookingInfo.cancellation.canCancel : true;
}
