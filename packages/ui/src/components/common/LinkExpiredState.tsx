import { Clock } from 'lucide-react';
import { Button } from '../ui/button';
import { messages } from '../../messages';
import { StatusHero } from './StatusHero';

interface LinkExpiredStateProps {
  onBack: () => void;
  /** Heading. Defaults to the es_AR copy. */
  title?: string | undefined;
  /** Body text. Defaults to the es_AR copy. */
  description?: string | undefined;
  /** Label of the back button. Defaults to "Volver". */
  backLabel?: string | undefined;
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
export function LinkExpiredState({
  onBack,
  title = messages.linkExpired,
  description = messages.linkExpiredDescription,
  backLabel = messages.back,
}: LinkExpiredStateProps) {
  return (
    <StatusHero icon={Clock} tone="warning" animated={false} title={title} description={description}>
      <Button variant="outline" className="mt-4 rounded-xl" onClick={onBack}>
        {backLabel}
      </Button>
    </StatusHero>
  );
}
