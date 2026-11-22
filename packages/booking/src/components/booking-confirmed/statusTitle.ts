import { BOOKING_MESSAGES } from '../../messages';
import type { CollectionStatus } from '../../api/types';
import type { BookingStatusView } from './statusView';

const t = BOOKING_MESSAGES.publicBooking;

/**
 * A rejected payment never leaves money collected — `processRejectedPayment`
 * (`internal/payments/process.go`) cancels the booking but leaves the payment
 * columns exactly as they started, because the payments enum has no "rejected"
 * value of its own. So `"unpaid"` here is the one shape an actual card decline
 * can leave; `deposit_paid` or `fully_paid` means a payment WAS collected and
 * is being reversed, which is a cancellation, not a decline — a different
 * message with a different tone.
 *
 * It reads the collection axis alone, and that is the whole question: a refund
 * under way is by definition money that arrived, so it can never be a decline.
 */
export function wasRejectedPayment(collectionStatus: CollectionStatus | undefined): boolean {
  return collectionStatus === undefined || collectionStatus === 'unpaid';
}

/**
 * The tab title of the success page: the same words as the screen's heading,
 * so a payment under review is not announced as a confirmed booking.
 */
export function statusViewTitle(view: BookingStatusView, collectionStatus: CollectionStatus | undefined): string {
  switch (view.kind) {
    case 'link_expired':
      return t.linkExpired;
    case 'link_not_found':
      return t.bookingNotFound;
    case 'error':
      return t.paymentStatusError;
    case 'under_review':
      return t.paymentUnderReview;
    case 'timed_out':
    case 'loading':
    case 'pending':
      return t.paymentProcessing;
    case 'cancelled':
      return wasRejectedPayment(collectionStatus) ? t.paymentFailed : t.bookingCancelled;
    case 'confirmed':
      return t.bookingSuccess;
  }
}
