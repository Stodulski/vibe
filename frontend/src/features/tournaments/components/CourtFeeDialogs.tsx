import type { CourtFeeTarget } from '../hooks/useTournamentChanges';
import type { CategoryView } from '../lib/categoryView';
import { slotLabel } from '../lib/tournamentView';
import { ChargeCourtFeeDialog } from './ChargeCourtFeeDialog';
import { CourtFeeDialog } from './CourtFeeDialog';
import { UndoCourtFeeDialog } from './UndoCourtFeeDialog';
import type { CategoryDialogs } from './useCategoryDialogs';
import type { TournamentOwnerCategory } from '@/shared/types/api.types';

/** The pair a court fee dialog is about and what it owes or paid. */
function feeSubject(view: CategoryView, target: CourtFeeTarget | null): { name: string; amount: number } {
  const match = target ? view.matches.find((m) => m.id === target.matchId) : undefined;
  if (!target || !match) return { name: '', amount: 0 };
  const side = match.court_fee.find((s) => s.side === target.side);
  return { name: slotLabel(match, target.side, view.pairsById, view.groupNames), amount: side?.amount ?? 0 };
}

interface CourtFeeDialogsProps {
  complexId: string;
  tournamentId: string;
  category: TournamentOwnerCategory;
  view: CategoryView;
  dialogs: CategoryDialogs;
}

/** The three court fee dialogs of a category: edit its fee, charge a pair, undo a charge. Each is mounted only while open. */
export function CourtFeeDialogs({ complexId, tournamentId, category, view, dialogs }: CourtFeeDialogsProps) {
  const charge = feeSubject(view, dialogs.chargeTarget);
  const undo = feeSubject(view, dialogs.undoTarget);
  return (
    <>
      <CourtFeeDialog
        open={dialogs.feeOpen}
        onClose={() => {
          dialogs.setFeeOpen(false);
        }}
        complexId={complexId}
        tournamentId={tournamentId}
        category={category}
      />
      <ChargeCourtFeeDialog
        open={!!dialogs.chargeTarget}
        onClose={() => {
          dialogs.setChargeTarget(null);
        }}
        complexId={complexId}
        tournamentId={tournamentId}
        target={dialogs.chargeTarget}
        pairName={charge.name}
        amount={charge.amount}
      />
      <UndoCourtFeeDialog
        open={!!dialogs.undoTarget}
        onClose={() => {
          dialogs.setUndoTarget(null);
        }}
        complexId={complexId}
        tournamentId={tournamentId}
        target={dialogs.undoTarget}
        pairName={undo.name}
        amount={undo.amount}
      />
    </>
  );
}
