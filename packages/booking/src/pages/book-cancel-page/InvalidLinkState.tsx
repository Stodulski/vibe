import { XCircle } from 'lucide-react';
import { Button } from '@vibe/ui';
import { StatusHero } from '@vibe/ui';
import { BOOKING_MESSAGES } from '../../messages';

const t = BOOKING_MESSAGES;

interface InvalidLinkStateProps {
  onBack: () => void;
}

export function InvalidLinkState({ onBack }: InvalidLinkStateProps) {
  return (
    <StatusHero
      icon={XCircle}
      tone="error"
      animated={false}
      title={t.publicBooking.bookingNotFound}
      description={t.publicBooking.invalidCancelLink}
    >
      <Button variant="outline" className="mt-4 min-h-12 rounded-xl" onClick={onBack}>
        {t.common.back}
      </Button>
    </StatusHero>
  );
}
