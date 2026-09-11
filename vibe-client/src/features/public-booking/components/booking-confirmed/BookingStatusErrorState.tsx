import { AlertTriangle } from 'lucide-react';
import { Button } from '@/shared/components/ui/button';
import { StatusHero } from '@/shared/components/common/StatusHero';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

interface BookingStatusErrorStateProps {
  onRetry: () => void;
}

/**
 * Shown when `GET /book/status` fails with anything other than 404/410 — a
 * network drop or a 500 — and there is no prior answer to fall back on.
 *
 * `onRetry` here refetches the same status check rather than sending the
 * visitor off to book again (`BookingConfirmed`'s other `onRetry`): the
 * payment itself may have gone through, and starting a new booking could
 * charge someone twice.
 */
export function BookingStatusErrorState({ onRetry }: BookingStatusErrorStateProps) {
  return (
    <StatusHero
      icon={AlertTriangle}
      tone="error"
      animated={false}
      title={t.publicBooking.paymentStatusError}
      description={t.publicBooking.paymentStatusErrorDescription}
    >
      <Button size="lg" className="min-h-12 mt-4 rounded-xl" onClick={onRetry}>
        {t.publicBooking.tryAgain}
      </Button>
    </StatusHero>
  );
}
