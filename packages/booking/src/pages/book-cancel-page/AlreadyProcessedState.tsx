import { XCircle } from 'lucide-react';
import { Button } from '@vibe/ui';
import { StatusHero } from '@vibe/ui';
import { BOOKING_MESSAGES } from '../../messages';
import type { CancelInfoResponse } from '../../api/types';

const t = BOOKING_MESSAGES;

interface AlreadyProcessedStateProps {
  cancelInfo: CancelInfoResponse;
  onBack: () => void;
}

export function AlreadyProcessedState({ cancelInfo, onBack }: AlreadyProcessedStateProps) {
  return (
    <StatusHero
      icon={XCircle}
      tone="error"
      animated={false}
      title={
        cancelInfo.booking.status === 'cancelled' ? t.publicBooking.alreadyCancelled : t.publicBooking.alreadyCompleted
      }
    >
      <Button variant="outline" className="mt-4 min-h-12 rounded-xl" onClick={onBack}>
        {t.publicBooking.makeAnother}
      </Button>
    </StatusHero>
  );
}
