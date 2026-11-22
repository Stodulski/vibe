import { XCircle, Ban, Phone } from 'lucide-react';
import { Button } from '@vibe/ui';
import { StatusHero } from '@vibe/ui';
import { BOOKING_MESSAGES } from '../../messages';
import type { CollectionStatus } from '../../api/types';
import type { BookingInfo } from './types';
import { wasRejectedPayment } from './statusTitle';

const t = BOOKING_MESSAGES;

interface CancelledStateProps {
  bookingInfo: BookingInfo | null;
  collectionStatus: CollectionStatus | undefined;
  onRetry: () => void;
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
      <Button onClick={onRetry} className="mt-2">
        {rejected ? t.publicBooking.tryAgain : t.publicBooking.makeAnother}
      </Button>
      {bookingInfo?.complexPhone && (
        <a
          href={`tel:${bookingInfo.complexPhone}`}
          className="text-primary-400 hover:text-primary-300 mt-1 flex min-h-12 items-center gap-1.5 text-sm transition-colors"
        >
          <Phone className="size-3.5" />
          {bookingInfo.complexPhone}
        </a>
      )}
    </StatusHero>
  );
}
