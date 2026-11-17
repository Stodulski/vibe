import { useState } from 'react';
import { useParams } from 'react-router-dom';
import { AlertTriangle } from 'lucide-react';
import { useSelectedComplex } from '@/features/complex';
import { BackLink } from '@/shared/components/common/BackLink';
import { EmptyState } from '@/shared/components/common/EmptyState';
import { StaleDataNotice } from '@/shared/components/common/StaleDataNotice';
import { usePageTitle } from '@/shared/hooks/usePageTitle';
import { ES_AR } from '@/shared/i18n/es_AR';
import { SportContext } from '@/shared/lib/sportContext';
import { todayInArgentina } from '@/shared/lib/today';
import { categoryTitle, tournamentTitle } from '@/shared/lib/traitLabels';
import { CategoryPanel } from '../components/CategoryPanel';
import { LabeledSelect } from '../components/LabeledSelect';
import { TournamentDetailBodySkeleton } from '../components/TournamentDetailBodySkeleton';
import { TournamentHeader } from '../components/TournamentHeader';
import { openingSection, sectionItems, type Section } from '../components/categorySections';
import { useCategoryDialogs } from '../components/useCategoryDialogs';
import { useTournament } from '../hooks/useTournament';
import { paidPairCount } from '../lib/tournamentView';
import { TOURNAMENT_DETAIL_CLASS, TOURNAMENT_SWITCH_ROW_CLASS } from '../lib/tournamentLayout';
import type { TournamentDetailResponse, TournamentOwnerCategory } from '@/shared/types/api.types';

const t = ES_AR;

/** One tournament: its facts, a selector for its categories and the panel of the chosen one. */
export default function TournamentDetailPage() {
  const { tournamentId } = useParams<{ tournamentId: string }>();
  const { selectedComplexId } = useSelectedComplex();
  usePageTitle(t.tournaments.title);
  if (!selectedComplexId || !tournamentId) return null;
  return <TournamentDetailContent complexId={selectedComplexId} tournamentId={tournamentId} />;
}

export function TournamentDetailContent({ complexId, tournamentId }: { complexId: string; tournamentId: string }) {
  const query = useTournament(complexId, tournamentId);

  return (
    <div className={TOURNAMENT_DETAIL_CLASS}>
      <BackLink to="/events">{t.tournaments.backToList}</BackLink>
      {query.isLoading ? (
        <TournamentDetailBodySkeleton />
      ) : !query.data ? (
        <EmptyState
          icon={AlertTriangle}
          title={t.tournaments.detailLoadError}
          description={t.common.loadErrorDescription}
          actionLabel={t.layout.retry}
          onAction={() => {
            void query.refetch();
          }}
        />
      ) : (
        <>
          {query.isError && (
            <StaleDataNotice
              onRetry={() => {
                void query.refetch();
              }}
            />
          )}
          <TournamentDetailBody detail={query.data} complexId={complexId} />
        </>
      )}
    </div>
  );
}

/** The section picked for one category. Moving to another category opens that one on its own section. */
interface SectionPick {
  categoryId: string;
  section: Section;
}

function pickOf(category: TournamentOwnerCategory): SectionPick {
  return { categoryId: category.id, section: openingSection(category) };
}

/** The category select's items: a cancelled category keeps its title and says so after it. */
function categoryItems(categories: TournamentOwnerCategory[], sport: string) {
  return categories.map((c) => ({
    value: c.id,
    label:
      c.status === 'cancelled'
        ? `${categoryTitle({ ...c, sport })} · ${t.tournaments.categoryStatus.cancelled}`
        : categoryTitle({ ...c, sport }),
  }));
}

function TournamentDetailBody({ detail, complexId }: { detail: TournamentDetailResponse; complexId: string }) {
  const { tournament, categories } = detail;
  const first = categories.find((c) => c.status !== 'cancelled') ?? categories[0];
  const [pick, setPick] = useState<SectionPick | null>(() => (first ? pickOf(first) : null));
  const category = categories.find((c) => c.id === pick?.categoryId) ?? first;
  // The pick holds for the category it was made on, so a draw made here does not move the view.
  const section = category && (pick?.categoryId === category.id ? pick.section : openingSection(category));
  const live = tournament.status === 'active' && tournament.end_date >= todayInArgentina();
  // The category's actions sit in the header's menu, so the dialogs they open are held here, above both.
  const dialogs = useCategoryDialogs();
  const openCategory = category && live && category.status !== 'cancelled' ? category : undefined;

  const chooseCategory = (categoryId: string) => {
    const next = categories.find((c) => c.id === categoryId);
    if (next) setPick(pickOf(next));
  };
  const chooseSection = (next: Section) => {
    if (category) setPick({ categoryId: category.id, section: next });
  };
  const sectionSwitch =
    category && section ? (
      <LabeledSelect
        label={t.tournaments.detail.sectionLabel}
        items={sectionItems(category)}
        active={section}
        onChange={chooseSection}
      />
    ) : null;

  return (
    <SportContext.Provider value={tournament.sport}>
      <TournamentHeader
        title={tournamentTitle(tournament.sport)}
        tournament={tournament}
        complexId={complexId}
        paidCount={paidPairCount(detail.pairs)}
        canCancel={live}
        openCategory={openCategory}
        dialogs={dialogs}
      />
      <div className={TOURNAMENT_SWITCH_ROW_CLASS}>
        <LabeledSelect
          label={t.tournaments.detail.categoriesLabel}
          items={categoryItems(categories, tournament.sport)}
          active={category?.id ?? ''}
          onChange={chooseCategory}
        />
        {sectionSwitch}
      </div>
      {category && section && (
        <CategoryPanel
          key={category.id}
          detail={detail}
          category={category}
          complexId={complexId}
          live={live}
          section={section}
          dialogs={dialogs}
        />
      )}
    </SportContext.Provider>
  );
}
