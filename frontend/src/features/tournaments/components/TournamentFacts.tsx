import { useState } from 'react';
import { InfoRow } from '@/shared/components/common/InfoRow';
import { ES_AR } from '@/shared/i18n/es_AR';
import { tournamentDays } from '../lib/tournamentView';
import { CancelTournamentDialog } from './CancelDialogs';
import { MoreMenu } from './MoreMenu';
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

/** The tournament's days and state, and the overflow menu with the action that cancels all of it. */
export function TournamentFacts({ tournament, complexId, paidCount, canCancel }: TournamentFactsProps) {
  const [cancelOpen, setCancelOpen] = useState(false);
  return (
    <div className="space-y-2">
      {tournament.status === 'cancelled' && <p className="text-error-text text-sm">{t.detail.cancelledNote}</p>}
      <div className="flex items-start justify-between gap-3">
        <InfoRow label={ES_AR.events.detail.when}>
          <span className="block font-medium first-letter:uppercase">
            {tournamentDays(tournament.start_date, tournament.end_date)}
          </span>
        </InfoRow>
        {canCancel && (
          <MoreMenu
            label={t.detail.moreTournament}
            items={[
              {
                label: t.cancel.action,
                destructive: true,
                onSelect: () => {
                  setCancelOpen(true);
                },
              },
            ]}
          />
        )}
      </div>
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
