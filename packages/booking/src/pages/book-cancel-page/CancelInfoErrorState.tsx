import { AlertTriangle } from 'lucide-react';
import { Button } from '@vibe/ui';
import { StatusHero } from '@vibe/ui';
import { BOOKING_MESSAGES } from '../../messages';

const t = BOOKING_MESSAGES;

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
      <Button size="lg" className="mt-4 min-h-12 rounded-xl" onClick={onRetry}>
        {t.publicBooking.tryAgain}
      </Button>
    </StatusHero>
  );
}
