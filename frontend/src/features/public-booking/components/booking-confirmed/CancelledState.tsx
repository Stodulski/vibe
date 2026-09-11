import { XCircle, Ban, Phone } from 'lucide-react';
import { Button } from '@/shared/components/ui/button';
import { StatusHero } from '@/shared/components/common/StatusHero';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { CollectionStatus } from '@/shared/types/api.types';
import type { BookingInfo } from './types';

const t = ES_AR;

interface CancelledStateProps {
  bookingInfo: BookingInfo | null;
  collectionStatus: CollectionStatus | undefined;
  onRetry: () => void;
}

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
function wasRejectedPayment(collectionStatus: CollectionStatus | undefined): boolean {
  return collectionStatus === undefined || collectionStatus === 'unpaid';
}

export function CancelledState({ bookingInfo, collectionStatus, onRetry }: CancelledStateProps) {
  const rejected = wasRejectedPayment(collectionStatus);

  return (
    <StatusHero
      icon={rejected ? XCircle : Ban}
      tone="error"
      title={rejected ? t.publicBooking.paymentFailed : t.publicBooking.bookingCancelled}
      titleClassName="text-2xl"
      description={rejected ? t.publicBooking.paymentFailedDescription : t.publicBooking.bookingCancelledDescription}
    >
      <Button onClick={onRetry} className="mt-2 rounded-xl">
        {rejected ? t.publicBooking.tryAgain : t.publicBooking.makeAnother}
      </Button>
      {bookingInfo?.complexPhone && (
        <a
          href={`tel:${bookingInfo.complexPhone}`}
          className="mt-1 flex min-h-12 items-center gap-1.5 text-sm text-primary-400 transition-colors hover:text-primary-300"
        >
          <Phone className="size-3.5" />
          {bookingInfo.complexPhone}
        </a>
      )}
    </StatusHero>
  );
}
