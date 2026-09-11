import { Clock } from 'lucide-react';
import { Button } from '@/shared/components/ui/button';
import { ES_AR } from '@/shared/i18n/es_AR';
import { StatusHero } from './StatusHero';

const t = ES_AR;

interface LinkExpiredStateProps {
  onBack: () => void;
}

/**
 * Shown when `resolveLink` (`internal/bookings/public.go`) answers 410 Gone:
 * the token resolved, but the link is no longer live. Distinct from a 404
 * (the token never existed) because the recourse is different — there is no
 * self-service reissue path, so the only next step is contacting the venue,
 * and this screen says so instead of reading as a generic dead link.
 *
 * Shared between the public cancel page and the booking-status success page:
 * both routes authorize through the same `resolveLink` call.
 */
export function LinkExpiredState({ onBack }: LinkExpiredStateProps) {
  return (
    <StatusHero
      icon={Clock}
      tone="warning"
      animated={false}
      title={t.publicBooking.linkExpired}
      description={t.publicBooking.linkExpiredDescription}
    >
      <Button variant="outline" className="mt-4 rounded-xl" onClick={onBack}>
        {t.common.back}
      </Button>
    </StatusHero>
  );
}
