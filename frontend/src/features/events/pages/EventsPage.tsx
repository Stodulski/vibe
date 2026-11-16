import { Plus } from 'lucide-react';
import { useSelectedComplex } from '@/features/complex';
import { EventsSectionTabs } from '@/shared/components/common/EventsSectionTabs';
import { PageHeader } from '@/shared/components/common/PageHeader';
import { usePageTitle } from '@/shared/hooks/usePageTitle';
import { ES_AR } from '@/shared/i18n/es_AR';
import { CreateEventModal } from '../components/CreateEventModal';
import { EventDetail } from '../components/EventDetail';
import { EventList } from '../components/EventList';
import { UpcomingPastSwitch } from '@/shared/components/common/UpcomingPastSwitch';
import { useEventsPage } from './events/useEventsPage';
import { EVENTS_PAGE_CLASS } from '../lib/eventLayout';

const t = ES_AR.events;

/** The "Canchas libres" tab of Eventos: upcoming and past events, a create dialog and a detail sheet per event. */
export default function EventsPage() {
  usePageTitle(ES_AR.navigation.events);
  const { selectedComplexId } = useSelectedComplex();
  if (!selectedComplexId) return null;
  return <EventsPageContent complexId={selectedComplexId} />;
}

function EventsPageContent({ complexId }: { complexId: string }) {
  const state = useEventsPage(complexId);

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

      <div className={EVENTS_PAGE_CLASS}>
        <UpcomingPastSwitch value={state.tab} onChange={state.setTab} />
        <EventList
          events={state.events}
          isLoading={state.isLoading}
          isError={state.isError}
          onRetry={() => {
            void state.refetch();
          }}
          onSelect={state.selectEvent}
          kind={state.tab}
          onCreate={() => {
            state.setCreateOpen(true);
          }}
        />
      </div>

      <CreateEventModal
        open={state.createOpen}
        onClose={() => {
          state.setCreateOpen(false);
        }}
        complexId={complexId}
        courts={state.courts}
      />
      <EventDetail
        open={!!state.selectedEventId}
        onClose={() => {
          state.selectEvent(null);
        }}
        complexId={complexId}
        eventId={state.selectedEventId}
      />
    </div>
  );
}
