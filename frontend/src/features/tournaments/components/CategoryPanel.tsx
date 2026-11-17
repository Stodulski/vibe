import { useState } from 'react';
import { MoreHorizontal } from 'lucide-react';
import { Button } from '@/shared/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/shared/components/ui/dropdown-menu';
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

/** What the owner can do to the category itself: change its court fee, or cancel it. Sits beside the category's summary. */
function CategoryMenu({ dialogs, courtFee }: { dialogs: CategoryDialogs; courtFee: number }) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon-sm" aria-label={t.detail.categoryActionsLabel}>
          <MoreHorizontal className="size-4" aria-hidden="true" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuItem
          onClick={() => {
            dialogs.setFeeOpen(true);
          }}
        >
          {courtFee > 0 ? t.courtFee.edit : t.courtFee.add}
        </DropdownMenuItem>
        <DropdownMenuItem
          variant="destructive"
          onClick={() => {
            dialogs.setCancelOpen(true);
          }}
        >
          {t.cancel.categoryAction}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

/**
 * One category: its summary line with its menu, the section switch, the draw
 * while it can still be made, and the chosen section flat on the page, without a
 * card at any width. A drawn category opens on its matches, one before the draw
 * on its pairs.
 */
export function CategoryPanel({ detail, category, complexId, live }: CategoryPanelProps) {
  const view = categoryView(detail, category.id);
  const dialogs = useCategoryDialogs();
  const unschedule = useUnscheduleMatch(complexId, detail.tournament.id);
  const drawn = category.draw_generated;
  const [section, setSection] = useState<Section>(drawn ? 'matches' : 'pairs');
  const open = live && category.status !== 'cancelled';

  return (
    <div className="space-y-4">
      <div className="flex items-start gap-2">
        <CategoryFacts category={category} courtFee={courtFeeSummary(view.matches)} />
        {open && <CategoryMenu dialogs={dialogs} courtFee={category.court_fee} />}
      </div>
      {category.status === 'cancelled' && <p className="text-error-text text-sm">{t.detail.categoryCancelledNote}</p>}
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
