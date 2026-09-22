import { AlertTriangle } from 'lucide-react';
import { Button } from '@/shared/components/ui/button';
import { ES_AR } from '@/shared/i18n/es_AR';
import { StatusHero } from './StatusHero';

const t = ES_AR;

interface ComplexLoadErrorProps {
  onRetry: () => void;
}

/**
 * Full-screen retry state for the owner-facing complexes query (network,
 * 5xx, timeout). Without this, the dashboard and onboarding pages read a
 * failed query the same as "no complex yet" and sent the owner to the
 * create-complex form, where submitting it 403'd with "the account already
 * owns a complex".
 */
export function ComplexLoadError({ onRetry }: ComplexLoadErrorProps) {
  return (
    <div className="bg-bg-base flex min-h-dvh items-center justify-center px-4">
      <StatusHero
        icon={AlertTriangle}
        tone="error"
        animated={false}
        title={t.complex.loadError}
        description={t.complex.loadErrorDescription}
      >
        <Button size="lg" className="mt-4 min-h-12 rounded-xl" onClick={onRetry}>
          {t.complex.retry}
        </Button>
      </StatusHero>
    </div>
  );
}
