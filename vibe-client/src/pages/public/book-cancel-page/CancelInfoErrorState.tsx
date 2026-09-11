import { AlertTriangle } from 'lucide-react';
import { Button } from '@/shared/components/ui/button';
import { StatusHero } from '@/shared/components/common/StatusHero';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

interface CancelInfoErrorStateProps {
  onRetry: () => void;
}

/**
 * Shown when `GET /book/cancel-info` fails with anything other than 404/410 —
 * a 500 or a network error. `InvalidLinkState` used to render for every
 * failure here (the comment above the old check called it a "deliberate
 * fallback"), which told the player their link was invalid when the real
 * problem was recoverable.
 */
export function CancelInfoErrorState({ onRetry }: CancelInfoErrorStateProps) {
  return (
    <StatusHero
      icon={AlertTriangle}
      tone="error"
      animated={false}
      title={t.publicBooking.cancelInfoLoadError}
      description={t.publicBooking.cancelInfoLoadErrorDescription}
    >
      <Button size="lg" className="min-h-12 mt-4 rounded-xl" onClick={onRetry}>
        {t.publicBooking.tryAgain}
      </Button>
    </StatusHero>
  );
}
