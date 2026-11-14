import { useState } from 'react';
import { useParams } from 'react-router-dom';
import { AlertTriangle } from 'lucide-react';
import { useSelectedComplex } from '@/features/complex';
import { BackLink } from '@/shared/components/common/BackLink';
import { EmptyState } from '@/shared/components/common/EmptyState';
import { PageHeader } from '@/shared/components/common/PageHeader';
import { StaleDataNotice } from '@/shared/components/common/StaleDataNotice';
import { usePageTitle } from '@/shared/hooks/usePageTitle';
import { ES_AR } from '@/shared/i18n/es_AR';
import { todayInArgentina } from '@/shared/lib/today';
import { CategoryPanel } from '../components/CategoryPanel';
import { SegmentedTabs } from '../components/SegmentedTabs';
import { TournamentDetailBodySkeleton } from '../components/TournamentDetailBodySkeleton';
import { TournamentFacts } from '../components/TournamentFacts';
import { useTournament } from '../hooks/useTournament';
import { paidPairCount } from '../lib/tournamentView';
import { TOURNAMENT_DETAIL_CLASS } from '../lib/tournamentLayout';
import type { TournamentDetailResponse } from '@/shared/types/api.types';

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
      <BackLink to="/events/tournaments">{t.tournaments.backToList}</BackLink>
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

function TournamentDetailBody({ detail, complexId }: { detail: TournamentDetailResponse; complexId: string }) {
  const [chosen, setChosen] = useState<string | null>(null);
  const { tournament, categories } = detail;
  const category =
    categories.find((c) => c.id === chosen) ?? categories.find((c) => c.status !== 'cancelled') ?? categories[0];
  const live = tournament.status === 'active' && tournament.end_date >= todayInArgentina();

  return (
    <>
      <PageHeader title={tournament.name} />
      <TournamentFacts
        tournament={tournament}
        complexId={complexId}
        paidCount={paidPairCount(detail.pairs)}
        canCancel={live}
      />
      <SegmentedTabs
        items={categories.map((c) => ({
          value: c.id,
          label: c.status === 'cancelled' ? `${c.name} · ${t.tournaments.categoryStatus.cancelled}` : c.name,
          muted: c.status === 'cancelled',
        }))}
        active={category?.id ?? ''}
        onChange={setChosen}
        ariaLabel={t.tournaments.detail.categoriesLabel}
      />
      {category && (
        <CategoryPanel key={category.id} detail={detail} category={category} complexId={complexId} live={live} />
      )}
    </>
  );
}
