import { XCircle } from 'lucide-react';
import { Button } from '@/shared/components/ui/button';
import { StatusHero } from '@/shared/components/common/StatusHero';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

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
      <Button variant="outline" className="min-h-12 mt-4 rounded-xl" onClick={onBack}>
        {t.common.back}
      </Button>
    </StatusHero>
  );
}
