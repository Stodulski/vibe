import { XCircle } from 'lucide-react';
import { Button } from '@/shared/components/ui/button';
import { StatusHero } from '@/shared/components/common/StatusHero';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

interface LinkNotFoundStateProps {
  onBack: () => void;
}

/**
 * Shown when `resolveLink` (`internal/bookings/public.go`) answers 404: the
 * token never existed, as opposed to `LinkExpiredState`'s 410 (it existed but
 * is no longer live). In practice this token was just handed to the client by
 * the booking flow or an MP redirect, so a 404 here means a corrupted or
 * tampered URL rather than a link that simply ran out.
 */
export function LinkNotFoundState({ onBack }: LinkNotFoundStateProps) {
  return (
    <StatusHero
      icon={XCircle}
      tone="error"
      animated={false}
      title={t.publicBooking.bookingNotFound}
      description={t.publicBooking.linkNotFoundDescription}
    >
      <Button size="lg" className="min-h-12 mt-4 rounded-xl" onClick={onBack}>
        {t.publicBooking.makeAnother}
      </Button>
    </StatusHero>
  );
}
