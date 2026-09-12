import { Clock, Phone } from 'lucide-react';
import { Button } from '@/shared/components/ui/button';
import { StatusHero } from '@/shared/components/common/StatusHero';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { BookingInfo } from './types';

const t = ES_AR;

interface PaymentUnderReviewStateProps {
  bookingInfo: BookingInfo | null;
  onRetry: () => void;
}

// Shown when MercadoPago's own back_url carries `?status=pending`
// (booklink.SuccessPending on the server): the payment is under review on
// their end, not just waiting on our webhook. That review can take days, so
// this never times out into TimeoutState's "a few minutes" copy — it stays
// on screen until the poll resolves to a terminal status.
export function PaymentUnderReviewState({ bookingInfo, onRetry }: PaymentUnderReviewStateProps) {
  return (
    <StatusHero
      icon={Clock}
      tone="warning"
      title={t.publicBooking.paymentUnderReview}
      description={t.publicBooking.paymentUnderReviewDescription}
      descriptionClassName="max-w-sm"
    >
      {bookingInfo?.complexPhone && (
        <a
          href={`tel:${bookingInfo.complexPhone}`}
          className="text-primary-400 hover:text-primary-300 mt-1 flex min-h-12 items-center gap-1.5 text-sm transition-colors"
        >
          <Phone className="size-3.5" />
          {bookingInfo.complexPhone}
        </a>
      )}
      <Button variant="outline" className="mt-2 min-h-12 rounded-xl" onClick={onRetry}>
        {t.publicBooking.makeAnother}
      </Button>
    </StatusHero>
  );
}
