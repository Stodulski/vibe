import { useState } from 'react';
import { Button } from '@/shared/components/ui/button';
import { InfoRow } from '@/shared/components/common/InfoRow';
import { Panel } from '@/shared/components/common/Panel';
import { ES_AR } from '@/shared/i18n/es_AR';
import { tournamentDays } from '../lib/tournamentView';
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

/**
 * The header block under the tournament name: its days and state, with the
 * secondary destructive action that cancels all of it on the same card.
 */
export function TournamentFacts({ tournament, complexId, paidCount, canCancel }: TournamentFactsProps) {
  const [cancelOpen, setCancelOpen] = useState(false);
  return (
    <>
      <Panel size="md" mobile="card" className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
        <div className="min-w-0 space-y-2">
          {tournament.status === 'cancelled' && (
            <div className="text-error-text flex flex-col gap-0.5 text-sm">
              <p>{t.detail.cancelledNote}</p>
              <p>{ES_AR.events.detail.cancelledFreeCourts}</p>
            </div>
          )}
          <InfoRow label={ES_AR.events.detail.when}>
            <span className="block font-medium first-letter:uppercase">
              {tournamentDays(tournament.start_date, tournament.end_date)}
            </span>
          </InfoRow>
        </div>
        {canCancel && (
          <Button
            type="button"
            variant="destructive-outline"
            className="shrink-0 self-start rounded-xl"
            onClick={() => {
              setCancelOpen(true);
            }}
          >
            {t.cancel.action}
          </Button>
        )}
      </Panel>
      <CancelTournamentDialog
        open={cancelOpen}
        onClose={() => {
          setCancelOpen(false);
        }}
        complexId={complexId}
        tournamentId={tournament.id}
        paidCount={paidCount}
      />
    </>
  );
}
