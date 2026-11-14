import { useState } from 'react';
import { Button } from '@/shared/components/ui/button';
import { InfoRow } from '@/shared/components/common/InfoRow';
import { ES_AR } from '@/shared/i18n/es_AR';
import { formatDateShort } from '@/shared/lib/utils';
import { CancelTournamentDialog } from './CancelDialogs';
import type { Tournament } from '@/shared/types/api.types';

const t = ES_AR.tournaments;

interface TournamentFactsProps {
  tournament: Tournament;
  complexId: string;
  /** How many pairs paid across the tournament: the ones cancelling refunds. */
  paidCount: number;
  /** False once the tournament is over or cancelled: nothing left to cancel. */
  canCancel: boolean;
}

/** The tournament's days and state, and the action that cancels all of it. */
export function TournamentFacts({ tournament, complexId, paidCount, canCancel }: TournamentFactsProps) {
  const [cancelOpen, setCancelOpen] = useState(false);
  return (
    <div className="space-y-3">
      {tournament.status === 'cancelled' && <p className="text-error-text text-sm">{t.detail.cancelledNote}</p>}
      <InfoRow label={ES_AR.events.detail.when}>
        <span className="score-text font-medium">
          {t.days(formatDateShort(tournament.start_date), formatDateShort(tournament.end_date))}
        </span>
      </InfoRow>
      {canCancel && (
        <Button
          type="button"
          variant="destructive"
          className="w-full rounded-xl sm:w-auto"
          onClick={() => {
            setCancelOpen(true);
          }}
        >
          {t.cancel.action}
        </Button>
      )}
      <CancelTournamentDialog
        open={cancelOpen}
        onClose={() => {
          setCancelOpen(false);
        }}
        complexId={complexId}
        tournamentId={tournament.id}
        paidCount={paidCount}
      />
    </div>
  );
}
