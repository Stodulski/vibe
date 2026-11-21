import { ES_AR } from '@/shared/i18n/es_AR';
import { useUnscheduleMatch } from '../hooks/useScheduleMatch';
import { categoryView, type CategoryView } from '../lib/categoryView';
import { BracketSection } from './BracketSection';
import { CategoryDialogsHost } from './CategoryDialogs';
import { DrawControls } from './DrawControls';
import { GroupsSection } from './GroupsSection';
import { MatchesSection } from './MatchesSection';
import { PairsSection } from './PairsSection';
import type { Section } from './categorySections';
import type { CategoryDialogs } from './useCategoryDialogs';
import type { TournamentOwnerCategory, TournamentDetailResponse } from '@/shared/types/api.types';

const t = ES_AR.tournaments;

interface CategoryPanelProps {
  detail: TournamentDetailResponse;
  category: TournamentOwnerCategory;
  complexId: string;
  /** False when the tournament is cancelled or over: nothing left to draw, schedule or cancel. */
  live: boolean;
  /** The section picked for this category in the row above the panel. */
  section: Section;
  /** The category's dialogs, held by the page: the header's menu opens them. */
  dialogs: CategoryDialogs;
}

/**
 * One category: the draw while it can still be made, and the section the owner
 * picked, flat on the page without a card at any width. The category and the
 * section are picked in the row above the panel; the category's actions are in
 * the tournament's menu above.
 */
export function CategoryPanel({ detail, category, complexId, live, section, dialogs }: CategoryPanelProps) {
  const view = categoryView(detail, category.id);
  const unschedule = useUnscheduleMatch(complexId, detail.tournament.id);
  const drawn = category.draw_generated;
  const open = live && category.status !== 'cancelled';

  return (
    <div className="space-y-4">
      {category.status === 'cancelled' && <p className="text-error-text text-sm">{t.detail.categoryCancelledNote}</p>}
      {open && section === 'pairs' && (
        <DrawControls
          complexId={complexId}
          tournamentId={detail.tournament.id}
          category={category}
          drawn={drawn}
          view={view}
        />
      )}
      <div className="min-w-0">
        <CategorySection
          section={section}
          category={category}
          view={view}
          open={open}
          dialogs={dialogs}
          onUnschedule={unschedule.mutate}
        />
      </div>
      <CategoryDialogsHost
        complexId={complexId}
        tournament={detail.tournament}
        category={category}
        view={view}
        dialogs={dialogs}
      />
    </div>
  );
}

interface CategorySectionProps {
  section: Section;
  category: TournamentOwnerCategory;
  view: CategoryView;
  /** The category can still change: not cancelled and the tournament is live. */
  open: boolean;
  dialogs: CategoryDialogs;
  onUnschedule: (matchId: string) => void;
}

function CategorySection({ section, category, view, open, dialogs, onUnschedule }: CategorySectionProps) {
  const drawn = category.draw_generated;
  if (section === 'pairs') {
    return (
      <PairsSection
        pairs={view.pairs}
        groupNames={view.groupNames}
        canSwap={open && drawn && !view.locked}
        onSwap={dialogs.setSwapPairId}
        canWithdraw={open}
        onWithdraw={dialogs.setWithdrawPairId}
        price={category.price}
        seeded={drawn}
      />
    );
  }
  if (section === 'groups') {
    return <GroupsSection groups={view.groups} pairs={view.pairsById} qualifiers={category.qualifiers_per_group} />;
  }
  if (section === 'bracket') {
    return <BracketSection matches={view.matches} pairs={view.pairsById} groupNames={view.groupNames} />;
  }
  return (
    <MatchesSection
      matches={view.matches}
      pairs={view.pairsById}
      groupNames={view.groupNames}
      editable={open}
      actions={{ onSchedule: dialogs.setScheduleMatchId, onUnschedule, onResult: dialogs.setResultMatchId }}
      courtFee={{
        fee: category.court_fee,
        actions: { onCharge: dialogs.setChargeTarget, onUndo: dialogs.setUndoTarget },
      }}
    />
  );
}
