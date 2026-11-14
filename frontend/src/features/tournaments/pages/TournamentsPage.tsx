import { Plus } from 'lucide-react';
import { useSelectedComplex } from '@/features/complex';
import { EventsSectionTabs } from '@/shared/components/common/EventsSectionTabs';
import { PageHeader } from '@/shared/components/common/PageHeader';
import { usePageTitle } from '@/shared/hooks/usePageTitle';
import { ES_AR } from '@/shared/i18n/es_AR';
import { CreateTournamentModal } from '../components/CreateTournamentModal';
import { SegmentedTabs } from '../components/SegmentedTabs';
import { TournamentList } from '../components/TournamentList';
import { TOURNAMENTS_PAGE_CLASS } from '../lib/tournamentLayout';
import { useTournamentsPage, type TournamentsTab } from './tournaments/useTournamentsPage';

const t = ES_AR.tournaments;

const TABS: { value: TournamentsTab; label: string }[] = [
  { value: 'upcoming', label: t.upcoming },
  { value: 'past', label: t.past },
];

/** The "Torneos" tab of Eventos: upcoming and past tournaments, a create dialog, and a detail page per tournament. */
export default function TournamentsPage() {
  usePageTitle(ES_AR.navigation.events);
  const { selectedComplexId } = useSelectedComplex();
  if (!selectedComplexId) return null;
  return <TournamentsPageContent complexId={selectedComplexId} />;
}

function TournamentsPageContent({ complexId }: { complexId: string }) {
  const state = useTournamentsPage(complexId);

  return (
    <div>
      <PageHeader
        title={ES_AR.navigation.events}
        action={{
          label: t.create,
          icon: Plus,
          onClick: () => {
            state.setCreateOpen(true);
          },
        }}
      />
      <EventsSectionTabs />
      <div className={TOURNAMENTS_PAGE_CLASS}>
        <SegmentedTabs items={TABS} active={state.tab} onChange={state.setTab} ariaLabel={t.title} />
        <TournamentList
          tournaments={state.tournaments}
          isLoading={state.isLoading}
          isError={state.isError}
          onRetry={() => {
            void state.refetch();
          }}
          kind={state.tab}
          onCreate={() => {
            state.setCreateOpen(true);
          }}
        />
      </div>
      <CreateTournamentModal
        open={state.createOpen}
        onClose={() => {
          state.setCreateOpen(false);
        }}
        complexId={complexId}
      />
    </div>
  );
}
