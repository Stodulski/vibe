import { AlertTriangle } from 'lucide-react';
import { Button } from '@/shared/components/ui/button';
import { StatusHero } from '@/shared/components/common/StatusHero';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

interface ComplexErrorStateProps {
  onRetry: () => void;
}

/**
 * Shown when `useComplexBySlug` fails with anything other than a 404 — a 500
 * or a network error. `NotFoundState` used to render for every failure here,
 * which told a player the club did not exist when the real problem was a
 * dropped connection they could just retry.
 */
export function ComplexErrorState({ onRetry }: ComplexErrorStateProps) {
  return (
    <StatusHero
      icon={AlertTriangle}
      tone="error"
      animated={false}
      title={t.publicBooking.complexLoadError}
      description={t.publicBooking.complexLoadErrorDescription}
    >
      <Button size="lg" className="min-h-12 mt-4 rounded-xl" onClick={onRetry}>
        {t.publicBooking.tryAgain}
      </Button>
    </StatusHero>
  );
}
