import { Plus } from 'lucide-react';
import { useSelectedComplex } from '@/features/complex';
import { EventsSectionTabs } from '@/shared/components/common/EventsSectionTabs';
import { PageHeader } from '@/shared/components/common/PageHeader';
import { UpcomingPastSwitch } from '@/shared/components/common/UpcomingPastSwitch';
import { usePageTitle } from '@/shared/hooks/usePageTitle';
import { ES_AR } from '@/shared/i18n/es_AR';
import { CreateTournamentModal } from '../components/CreateTournamentModal';
import { TournamentList } from '../components/TournamentList';
import { TOURNAMENTS_PAGE_CLASS } from '../lib/tournamentLayout';
import { useTournamentsPage } from './tournaments/useTournamentsPage';

const t = ES_AR.tournaments;

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
        <UpcomingPastSwitch value={state.tab} onChange={state.setTab} />
        <TournamentList
          tournaments={state.tournaments}
          isLoading={state.isLoading}
          isError={state.isError}
          onRetry={() => {
            void state.refetch();
          }}
          kind={state.tab}
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
