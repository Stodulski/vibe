import { Plus } from 'lucide-react';
import { useSelectedComplex } from '@/features/complex';
import { CreateEventModal, EVENTS_PAGE_CLASS, EventDetail, EventTypePicker } from '@/features/events';
import { CreateTournamentModal } from '@/features/tournaments';
import { PageHeader } from '@/shared/components/common/PageHeader';
import { UpcomingPastSwitch } from '@/shared/components/common/UpcomingPastSwitch';
import { usePageTitle } from '@/shared/hooks/usePageTitle';
import { ES_AR } from '@/shared/i18n/es_AR';
import { EventsHubList } from './EventsHubList';
import { useEventsHub } from './useEventsHub';

const t = ES_AR.events;

/**
 * Eventos: open-play events (canchas abiertas) and tournaments in one list, with Próximos and
 * Pasados. Crear evento asks which kind to create, then opens that form. A tournament is
 * opened on its own detail page; an event opens its sheet here.
 */
export default function EventsHubPage() {
  usePageTitle(ES_AR.navigation.events);
  const { selectedComplexId } = useSelectedComplex();
  if (!selectedComplexId) return null;
  return <EventsHubContent complexId={selectedComplexId} />;
}

function EventsHubContent({ complexId }: { complexId: string }) {
  const state = useEventsHub(complexId);

  return (
    <div>
      <PageHeader
        title={ES_AR.navigation.events}
        action={{
          label: t.create,
          icon: Plus,
          onClick: () => {
            state.setPickerOpen(true);
          },
        }}
      />

      <div className={EVENTS_PAGE_CLASS}>
        <UpcomingPastSwitch value={state.tab} onChange={state.setTab} />
        <EventsHubList
          items={state.items}
          isLoading={state.isLoading}
          isError={state.isError}
          onRetry={() => {
            void state.refetch();
          }}
          onSelect={state.selectEvent}
          kind={state.tab}
          onCreate={() => {
            state.setPickerOpen(true);
          }}
        />
      </div>

      <EventTypePicker
        open={state.pickerOpen}
        onClose={() => {
          state.setPickerOpen(false);
        }}
        onPick={state.pickKind}
      />
      <CreateEventModal
        open={state.createKind === 'openPlay'}
        onClose={state.closeCreate}
        complexId={complexId}
        courts={state.courts}
      />
      <CreateTournamentModal
        open={state.createKind === 'tournament'}
        onClose={state.closeCreate}
        complexId={complexId}
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
