import { useState } from 'react';
import { MoreHorizontal } from 'lucide-react';
import { usePageHeading } from '@/shared/components/layout/page-heading/usePageHeading';
import { Button } from '@/shared/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/shared/components/ui/dropdown-menu';
import { ES_AR } from '@/shared/i18n/es_AR';
import { tournamentDays } from '../lib/tournamentView';
import { CancelTournamentDialog } from './CancelDialogs';
import type { Tournament } from '@/shared/types/api.types';

const t = ES_AR.tournaments;

interface TournamentHeaderProps {
  title: string;
  tournament: Tournament;
  complexId: string;
  /** How many pairs paid across the tournament: the ones cancelling refunds. */
  paidCount: number;
  /** False once the tournament is over or cancelled: nothing left to cancel. */
  canCancel: boolean;
}

/**
 * The tournament's name with its days under it, and the one overflow menu that
 * holds its cancel action. The desktop masthead carries the name, as on every
 * owner page, so the name shows here only on a phone.
 */
export function TournamentHeader({ title, tournament, complexId, paidCount, canCancel }: TournamentHeaderProps) {
  usePageHeading(title);
  const [cancelOpen, setCancelOpen] = useState(false);

  return (
    <>
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0 space-y-1">
          <h1 className="font-display text-text-primary text-xl leading-tight font-semibold sm:text-2xl md:hidden">
            {title}
          </h1>
          <p className="text-text-tertiary block text-sm first-letter:uppercase">
            {tournamentDays(tournament.start_date, tournament.end_date)}
          </p>
        </div>
        {canCancel && (
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="ghost" size="icon-sm" aria-label={ES_AR.common.rowActionsLabel}>
                <MoreHorizontal className="size-4" aria-hidden="true" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem
                variant="destructive"
                onClick={() => {
                  setCancelOpen(true);
                }}
              >
                {t.cancel.action}
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        )}
      </div>
      {tournament.status === 'cancelled' && (
        <div className="text-error-text flex flex-col gap-0.5 text-sm">
          <p>{t.detail.cancelledNote}</p>
          <p>{ES_AR.events.detail.cancelledFreeCourts}</p>
        </div>
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
    </>
  );
}
