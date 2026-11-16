import { useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { useCourts } from '@/features/courts';
import { pastEventsRange, upcomingEventsRange, useEvents } from '@/features/events';
import type { CreateEventKind } from '@/features/events';
import { pastTournamentsRange, upcomingTournamentsRange, useTournaments } from '@/features/tournaments';
import { todayInArgentina } from '@/shared/lib/today';
import { buildAgenda, type AgendaTab } from './eventsHubAgenda';

/** The `?event=` the grid's "Canchas abiertas" block links to; it opens the event's detail sheet. */
const EVENT_PARAM = 'event';

/**
 * Everything the events page reads: the complex's courts (for the open-play form), the one
 * list of both kinds for the open tab, which create form is open, and which event's sheet is open.
 * The past windows are only fetched once the Pasados tab is opened.
 */
export function useEventsHub(complexId: string) {
  const { data: courts = [] } = useCourts(complexId);
  const [tab, setTab] = useState<AgendaTab>('upcoming');
  const [pickerOpen, setPickerOpen] = useState(false);
  const [createKind, setCreateKind] = useState<CreateEventKind | null>(null);
  const [params, setParams] = useSearchParams();

  const today = todayInArgentina();
  const eventRange = tab === 'upcoming' ? upcomingEventsRange(today) : pastEventsRange(today);
  const tournamentRange = tab === 'upcoming' ? upcomingTournamentsRange(today) : pastTournamentsRange(today);
  const events = useEvents(complexId, eventRange.from, eventRange.to);
  const tournaments = useTournaments(complexId, tournamentRange.from, tournamentRange.to);

  const selectedEventId = params.get(EVENT_PARAM);
  const selectEvent = (eventId: string | null) => {
    setParams(
      (prev) => {
        const next = new URLSearchParams(prev);
        if (eventId) next.set(EVENT_PARAM, eventId);
        else next.delete(EVENT_PARAM);
        return next;
      },
      { replace: true },
    );
  };

  return {
    courts,
    tab,
    setTab,
    items: buildAgenda(events.data ?? [], tournaments.data ?? [], tab),
    isLoading: events.isLoading || tournaments.isLoading,
    isError: events.isError || tournaments.isError,
    refetch: () => Promise.all([events.refetch(), tournaments.refetch()]),
    pickerOpen,
    setPickerOpen,
    createKind,
    pickKind: (kind: CreateEventKind) => {
      setPickerOpen(false);
      setCreateKind(kind);
    },
    closeCreate: () => {
      setCreateKind(null);
    },
    selectedEventId,
    selectEvent,
  };
}
