import { useState } from 'react';
import { Button } from '@/shared/components/ui/button';
import { ES_AR } from '@/shared/i18n/es_AR';
import { useUnscheduleMatch } from '../hooks/useScheduleMatch';
import { categoryView, type CategoryView } from '../lib/categoryView';
import { BracketSection } from './BracketSection';
import { CategoryDialogsHost } from './CategoryDialogs';
import { CategoryFacts } from './CategoryFacts';
import { DrawControls } from './DrawControls';
import { GroupsSection } from './GroupsSection';
import { MatchesSection } from './MatchesSection';
import { PairsSection } from './PairsSection';
import { SegmentedTabs } from './SegmentedTabs';
import { useCategoryDialogs, type CategoryDialogs } from './useCategoryDialogs';
import type { TournamentCategory, TournamentDetailResponse } from '@/shared/types/api.types';

const t = ES_AR.tournaments;

type Section = 'pairs' | 'groups' | 'bracket' | 'matches';

interface CategoryPanelProps {
  detail: TournamentDetailResponse;
  category: TournamentCategory;
  complexId: string;
  /** False when the tournament is cancelled or over: nothing left to draw, schedule or cancel. */
  live: boolean;
}

function sectionItems(category: TournamentCategory): { value: Section; label: string }[] {
  const s = t.detail.sections;
  return [
    { value: 'pairs', label: s.pairs },
    ...(category.format === 'groups' ? [{ value: 'groups' as const, label: s.groups }] : []),
    { value: 'bracket', label: s.bracket },
    { value: 'matches', label: s.matches },
  ];
}

/** One category: its facts, the draw, and the sections for pairs, groups, bracket and matches. */
export function CategoryPanel({ detail, category, complexId, live }: CategoryPanelProps) {
  const view = categoryView(detail, category.id);
  const dialogs = useCategoryDialogs();
  const unschedule = useUnscheduleMatch(complexId, detail.tournament.id);
  const [section, setSection] = useState<Section>('pairs');
  const open = live && category.status !== 'cancelled';
  const drawn = category.draw_generated;

  return (
    <div className="space-y-4">
      <CategoryFacts category={category} />
      {open && (
        <DrawControls
          complexId={complexId}
          tournamentId={detail.tournament.id}
          categoryId={category.id}
          drawn={drawn}
          view={view}
        />
      )}
      {open && (
        <Button
          type="button"
          variant="outline"
          className="text-error-text w-full rounded-xl sm:w-auto"
          onClick={() => {
            dialogs.setCancelOpen(true);
          }}
        >
          {t.cancel.categoryAction}
        </Button>
      )}
      <SegmentedTabs
        items={sectionItems(category)}
        active={section}
        onChange={setSection}
        ariaLabel={t.detail.sectionsLabel}
      />
      <CategorySection
        section={section}
        category={category}
        view={view}
        open={open}
        dialogs={dialogs}
        onUnschedule={unschedule.mutate}
      />
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
  category: TournamentCategory;
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
        {drawn && view.locked && <p className="text-text-tertiary text-xs">{t.pairs.swapLocked}</p>}
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
    />
  );
}
