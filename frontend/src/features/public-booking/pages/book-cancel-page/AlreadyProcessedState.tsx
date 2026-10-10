import { XCircle } from 'lucide-react';
import { Button } from '@/shared/components/ui/button';
import { StatusHero } from '@/shared/components/common/StatusHero';
import { BOOKING_MESSAGES } from '../../messages';
import type { CancelInfoResponse } from '@/shared/types/api.types';

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
