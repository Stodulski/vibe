import { AlertTriangle, CalendarX } from 'lucide-react';
import { Button, StatusHero } from '@vibe/ui';
import { BOOKING_MESSAGES } from '../../messages';
import { BackToComplexButton } from '../event-page/EventPageStates';

const t = BOOKING_MESSAGES;

export function TournamentNotFoundState({ slug }: { slug: string }) {
  return (
    <StatusHero
      icon={CalendarX}
      tone="neutral"
      title={t.tournaments.notFound}
      description={t.tournaments.notFoundDescription}
    >
      <BackToComplexButton slug={slug} />
    </StatusHero>
  );
}

export function TournamentLoadErrorState({ onRetry }: { onRetry: () => void }) {
  return (
    <StatusHero
      icon={AlertTriangle}
      tone="warning"
      title={t.tournaments.loadError}
      description={t.publicBooking.complexLoadErrorDescription}
    >
      <Button variant="outline" className="min-h-12" onClick={onRetry}>
        {t.publicBooking.tryAgain}
      </Button>
    </StatusHero>
  );
}

export function CancelledTournamentNotice() {
  return (
    <StatusHero
      icon={CalendarX}
      tone="warning"
      title={t.tournaments.cancelledTitle}
      description={t.tournaments.cancelledDescription}
    />
  );
}
