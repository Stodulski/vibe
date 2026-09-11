import { Clock, Phone } from 'lucide-react';
import { Button } from '@/shared/components/ui/button';
import { StatusHero } from '@/shared/components/common/StatusHero';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { BookingInfo } from './types';

const t = ES_AR;

interface TimeoutStateProps {
  bookingInfo: BookingInfo | null;
  onRetry: () => void;
}

export function TimeoutState({ bookingInfo, onRetry }: TimeoutStateProps) {
  return (
    <StatusHero
      icon={Clock}
      tone="warning"
      title={t.publicBooking.paymentProcessing}
      description={t.publicBooking.paymentProcessingDescription}
      descriptionClassName="max-w-sm"
    >
      {bookingInfo?.complexPhone && (
        <a
          href={`tel:${bookingInfo.complexPhone}`}
          className="mt-1 flex min-h-12 items-center gap-1.5 text-sm text-primary-400 transition-colors hover:text-primary-300"
        >
          <Phone className="size-3.5" />
          {bookingInfo.complexPhone}
        </a>
      )}
      <Button variant="outline" className="min-h-12 mt-2 rounded-xl" onClick={onRetry}>
        {t.publicBooking.makeAnother}
      </Button>
    </StatusHero>
  );
}
