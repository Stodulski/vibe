import { useState } from 'react';
import { Panel } from '@/shared/components/common/Panel';
import { Button } from '@/shared/components/ui/button';
import { ES_AR } from '@/shared/i18n/es_AR';
import { useUnscheduleMatch } from '../hooks/useScheduleMatch';
import { categoryView, type CategoryView } from '../lib/categoryView';
import { courtFeeSummary } from '../lib/courtFee';
import { BracketSection } from './BracketSection';
import { CategoryDialogsHost } from './CategoryDialogs';
import { CategoryFacts } from './CategoryFacts';
import { DrawControls } from './DrawControls';
import { GroupsSection } from './GroupsSection';
import { MatchesSection } from './MatchesSection';
import { PairsSection } from './PairsSection';
import { SegmentedTabs } from './SegmentedTabs';
import { useCategoryDialogs, type CategoryDialogs } from './useCategoryDialogs';
import type { TournamentOwnerCategory, TournamentDetailResponse } from '@/shared/types/api.types';

const t = ES_AR.tournaments;

type Section = 'pairs' | 'groups' | 'bracket' | 'matches';

interface CategoryPanelProps {
  detail: TournamentDetailResponse;
  category: TournamentOwnerCategory;
  complexId: string;
  /** False when the tournament is cancelled or over: nothing left to draw, schedule or cancel. */
  live: boolean;
}

function sectionItems(category: TournamentOwnerCategory): { value: Section; label: string }[] {
  const s = t.detail.sections;
  return [
    { value: 'pairs', label: s.pairs },
    ...(category.format === 'groups' ? [{ value: 'groups' as const, label: s.groups }] : []),
    { value: 'bracket', label: s.bracket },
    { value: 'matches', label: s.matches },
  ];
}

/** What the owner can do to the category itself: change its court fee, or cancel it. Sits at the foot of the category card. */
function CategoryActions({ dialogs, courtFee }: { dialogs: CategoryDialogs; courtFee: number }) {
  return (
    <div className="border-border-subtle flex flex-wrap gap-2 border-t pt-4">
      <Button
        type="button"
        variant="outline"
        className="rounded-xl"
        onClick={() => {
          dialogs.setFeeOpen(true);
        }}
      >
        {courtFee > 0 ? t.courtFee.edit : t.courtFee.add}
      </Button>
      <Button
        type="button"
        variant="destructive-outline"
        onClick={() => {
          dialogs.setCancelOpen(true);
        }}
      >
        {t.cancel.categoryAction}
      </Button>
    </div>
  );
}

/**
 * One category: its facts and actions in a card, then the section switch, the
 * draw while it can still be made, and the chosen section inside its own card.
 */
export function CategoryPanel({ detail, category, complexId, live }: CategoryPanelProps) {
  const view = categoryView(detail, category.id);
  const dialogs = useCategoryDialogs();
  const unschedule = useUnscheduleMatch(complexId, detail.tournament.id);
  const [section, setSection] = useState<Section>('pairs');
  const open = live && category.status !== 'cancelled';
  const drawn = category.draw_generated;

  return (
    <div className="space-y-4">
      <Panel size="md" mobile="card" className="space-y-4">
        <CategoryFacts category={category} courtFee={courtFeeSummary(view.matches)} />
        {category.status === 'cancelled' && <p className="text-error-text text-sm">{t.detail.categoryCancelledNote}</p>}
        {open && <CategoryActions dialogs={dialogs} courtFee={category.court_fee} />}
      </Panel>
      <SegmentedTabs
        items={sectionItems(category)}
        active={section}
        onChange={setSection}
        ariaLabel={t.detail.sectionsLabel}
      />
      {open && section === 'pairs' && (
        <DrawControls
          complexId={complexId}
          tournamentId={detail.tournament.id}
          category={category}
          drawn={drawn}
          view={view}
        />
      )}
      <Panel size="md" mobile="card" className="min-w-0">
        <CategorySection
          section={section}
          category={category}
          view={view}
          open={open}
          dialogs={dialogs}
          onUnschedule={unschedule.mutate}
        />
      </Panel>
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
      <>
        {/* The one line that says the draw and the swaps are closed: the draw card itself is gone once a result is loaded. */}
        {drawn && view.locked && <p className="text-text-tertiary mb-2 text-xs">{t.pairs.swapLocked}</p>}
        <PairsSection
          pairs={view.pairs}
          groupNames={view.groupNames}
          canSwap={open && drawn && !view.locked}
          onSwap={dialogs.setSwapPairId}
        />
      </>
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
