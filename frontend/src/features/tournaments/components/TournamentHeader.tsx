import { useState } from 'react';
import { MoreHorizontal } from 'lucide-react';
import { usePageHeading } from '@/shared/components/layout/page-heading/usePageHeading';
import { Button } from '@/shared/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/shared/components/ui/dropdown-menu';
import { ES_AR } from '@/shared/i18n/es_AR';
import { categoryTitle } from '@/shared/lib/traitLabels';
import { tournamentDays, tournamentDaysShort } from '../lib/tournamentView';
import { CancelTournamentDialog } from './CancelDialogs';
import type { CategoryDialogs } from './useCategoryDialogs';
import type { Tournament, TournamentOwnerCategory } from '@/shared/types/api.types';

const t = ES_AR.tournaments;

interface TournamentActionsMenuProps {
  /** The category picked above the panel, only when it can still change. */
  openCategory: TournamentOwnerCategory | undefined;
  categoryLabel: string;
  /** False once the tournament is over or cancelled: nothing left to cancel. */
  canCancel: boolean;
  dialogs: CategoryDialogs;
  onCancelTournament: () => void;
}

/** The one overflow menu: the picked category's group, a separator, then the tournament's cancel. */
function TournamentActionsMenu({
  openCategory,
  categoryLabel,
  canCancel,
  dialogs,
  onCancelTournament,
}: TournamentActionsMenuProps) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon-sm" aria-label={ES_AR.common.rowActionsLabel}>
          <MoreHorizontal className="size-4" aria-hidden="true" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        {openCategory && (
          <DropdownMenuGroup aria-label={categoryLabel}>
            <DropdownMenuLabel>{categoryLabel}</DropdownMenuLabel>
            <DropdownMenuItem
              onClick={() => {
                dialogs.setFeeOpen(true);
              }}
            >
              {openCategory.court_fee > 0 ? t.courtFee.edit : t.courtFee.add}
            </DropdownMenuItem>
            <DropdownMenuItem
              variant="destructive"
              onClick={() => {
                dialogs.setCancelOpen(true);
              }}
            >
              {t.cancel.categoryAction}
            </DropdownMenuItem>
          </DropdownMenuGroup>
        )}
        {openCategory && canCancel && <DropdownMenuSeparator />}
        {canCancel && (
          <DropdownMenuItem
            variant="destructive"
            onClick={() => {
              onCancelTournament();
            }}
          >
            {t.cancel.action}
          </DropdownMenuItem>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

interface TournamentHeaderProps {
  title: string;
  tournament: Tournament;
  complexId: string;
  /** How many pairs paid across the tournament: the ones cancelling refunds. */
  paidCount: number;
  /** False once the tournament is over or cancelled: nothing left to cancel. */
  canCancel: boolean;
  /** The category picked above the panel, only when it can still change: its actions join the menu. */
  openCategory: TournamentOwnerCategory | undefined;
  /** The dialogs of the category, which the category's actions open. */
  dialogs: CategoryDialogs;
}

/**
 * The tournament's name with its days under it, and the one overflow menu that
 * holds the actions of the picked category, then the tournament's cancel. The
 * desktop masthead carries the name, as on every owner page, so the name shows
 * here only on a phone.
 */
export function TournamentHeader({
  title,
  tournament,
  complexId,
  paidCount,
  canCancel,
  openCategory,
  dialogs,
}: TournamentHeaderProps) {
  usePageHeading(title);
  const [cancelOpen, setCancelOpen] = useState(false);
  const days = tournamentDays(tournament.start_date, tournament.end_date);
  const daysShort = tournamentDaysShort(tournament.start_date, tournament.end_date);
  const categoryLabel = openCategory ? categoryTitle({ ...openCategory, sport: tournament.sport }) : '';
  const hasActions = openCategory !== undefined || canCancel;

  return (
    <>
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0 space-y-1">
          <h1 className="font-display text-text-primary truncate text-xl leading-tight font-semibold sm:text-2xl md:hidden">
            {title}
          </h1>
          <p className="text-text-tertiary block truncate text-sm first-letter:uppercase" title={days}>
            <span className="sm:hidden">{daysShort}</span>
            <span className="hidden sm:inline">{days}</span>
          </p>
        </div>
        {hasActions && (
          <TournamentActionsMenu
            openCategory={openCategory}
            categoryLabel={categoryLabel}
            canCancel={canCancel}
            dialogs={dialogs}
            onCancelTournament={() => {
              setCancelOpen(true);
            }}
          />
        )}
      </div>
      {tournament.status === 'cancelled' && (
        <div className="text-error-text flex min-w-0 flex-col gap-0.5 text-sm">
          <p className="truncate">{t.detail.cancelledNote}</p>
          <p className="truncate" title={ES_AR.events.detail.cancelledFreeCourts}>
            {ES_AR.events.detail.cancelledFreeCourts}
          </p>
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
