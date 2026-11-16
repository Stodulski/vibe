import { useCourts } from '@/features/courts';
import { categoryTitle } from '@/shared/lib/traitLabels';
import type { CategoryView } from '../lib/categoryView';
import { CancelCategoryDialog } from './CancelDialogs';
import { CourtFeeDialogs } from './CourtFeeDialogs';
import { ResultDialog } from './ResultDialog';
import { ScheduleMatchDialog } from './ScheduleMatchDialog';
import { SwapPairDialog } from './SwapPairDialog';
import type { CategoryDialogs } from './useCategoryDialogs';
import type { Tournament, TournamentOwnerCategory } from '@/shared/types/api.types';

interface CategoryDialogsHostProps {
  complexId: string;
  tournament: Tournament;
  category: TournamentOwnerCategory;
  view: CategoryView;
  dialogs: CategoryDialogs;
}

/** Every dialog the category panel can open, each mounted only while its own target is set. */
export function CategoryDialogsHost({ complexId, tournament, category, view, dialogs }: CategoryDialogsHostProps) {
  const { data: courts = [] } = useCourts(complexId);
  const find = (id: string | null) => view.matches.find((match) => match.id === id) ?? null;

  return (
    <>
      <ScheduleMatchDialog
        open={!!dialogs.scheduleMatchId}
        onClose={() => {
          dialogs.setScheduleMatchId(null);
        }}
        complexId={complexId}
        tournament={tournament}
        category={category}
        match={find(dialogs.scheduleMatchId)}
        courts={courts.filter((court) => court.is_active)}
      />
      <ResultDialog
        open={!!dialogs.resultMatchId}
        onClose={() => {
          dialogs.setResultMatchId(null);
        }}
        complexId={complexId}
        tournamentId={tournament.id}
        category={category}
        match={find(dialogs.resultMatchId)}
        pairs={view.pairsById}
      />
      <SwapPairDialog
        open={!!dialogs.swapPairId}
        onClose={() => {
          dialogs.setSwapPairId(null);
        }}
        complexId={complexId}
        tournamentId={tournament.id}
        categoryId={category.id}
        pair={dialogs.swapPairId ? (view.pairsById.get(dialogs.swapPairId) ?? null) : null}
        pairs={view.pairs}
        needsOtherGroup={category.format === 'groups'}
      />
      <CourtFeeDialogs
        complexId={complexId}
        tournamentId={tournament.id}
        category={category}
        view={view}
        dialogs={dialogs}
      />
      <CancelCategoryDialog
        open={dialogs.cancelOpen}
        onClose={() => {
          dialogs.setCancelOpen(false);
        }}
        complexId={complexId}
        tournamentId={tournament.id}
        categoryId={category.id}
        categoryName={categoryTitle(category)}
        paidCount={view.paidPairs.length}
      />
    </>
  );
}
