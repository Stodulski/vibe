import type { BookingStatus, BookingStatusDetails, CollectionStatus } from '@/shared/types/api.types';
import { LinkExpiredState } from '@/shared/components/common/LinkExpiredState';
import { TimeoutState } from './booking-confirmed/TimeoutState';
import { LoadingState } from './booking-confirmed/LoadingState';
import { PendingState } from './booking-confirmed/PendingState';
import { PaymentUnderReviewState } from './booking-confirmed/PaymentUnderReviewState';
import { BookingStatusErrorState } from './booking-confirmed/BookingStatusErrorState';
import { CancelledState } from './booking-confirmed/CancelledState';
import { SuccessState } from './booking-confirmed/SuccessState';
import { LinkNotFoundState } from './booking-confirmed/LinkNotFoundState';
import type { BookingInfo } from './booking-confirmed/types';
import type { BookingStatusView } from './booking-confirmed/statusView';

export type { BookingInfo } from './booking-confirmed/types';

interface BookingConfirmedProps {
  view: BookingStatusView;
  status: BookingStatus | undefined;
  collectionStatus: CollectionStatus | undefined;
  bookingInfo: BookingInfo | null;
  /** `GET /book/status`'s full answer — forwarded to `SuccessState` only; undefined for every other state. */
  bookingDetails?: BookingStatusDetails | undefined;
  token: string | null;
  slug: string;
  onRetry: () => void;
  /**
   * Refetches `GET /book/status` — used only by the `error` view, since a
   * failed status check doesn't mean the booking failed. `onRetry` above
   * sends the visitor off to book again, which the `error` view avoids: the
   * payment may already have gone through.
   */
  onStatusRetry: () => void;
}

export function BookingConfirmed({
  view,
  status,
  collectionStatus,
  bookingInfo,
  bookingDetails,
  token,
  slug,
  onRetry,
  onStatusRetry,
}: BookingConfirmedProps) {
  // The three polling states below share this live region: someone waiting
  // on this screen is looking away between refetches, and a status change
  // (pending -> under review, or under review -> timed out) that only
  // repaints silently would never reach them. `role="status"` + `polite`
  // announces the new copy without interrupting whatever they were doing.
  switch (view.kind) {
    case 'link_expired':
      return <LinkExpiredState onBack={onRetry} />;
    case 'link_not_found':
      return <LinkNotFoundState onBack={onRetry} />;
    case 'error':
      return <BookingStatusErrorState onRetry={onStatusRetry} />;
    case 'under_review':
      return (
        <div role="status" aria-live="polite">
          <PaymentUnderReviewState bookingInfo={bookingInfo} onRetry={onRetry} />
        </div>
      );
    case 'timed_out':
      return (
        <div role="status" aria-live="polite">
          <TimeoutState bookingInfo={bookingInfo} onRetry={onRetry} />
        </div>
      );
    case 'loading':
      return <LoadingState />;
    case 'pending':
      return (
        <div role="status" aria-live="polite">
          <PendingState />
        </div>
      );
    case 'cancelled':
      return <CancelledState bookingInfo={bookingInfo} collectionStatus={collectionStatus} onRetry={onRetry} />;
    case 'confirmed':
      return (
        <SuccessState
          bookingInfo={bookingInfo}
          bookingDetails={bookingDetails}
          token={token}
          status={status}
          slug={slug}
        />
      );
    default:
      return view satisfies never;
  }
}
