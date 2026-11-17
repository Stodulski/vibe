import { useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { useCourts } from '@/features/courts';
import { pastEventsRange, upcomingEventsRange, useEvents } from '@/features/events';
import type { CreateEventKind } from '@/features/events';
import { pastTournamentsRange, upcomingTournamentsRange, useTournaments } from '@/features/tournaments';
import { todayInArgentina } from '@/shared/lib/today';
import {
  ALL_AGENDA_FILTER,
  agendaDayOptions,
  agendaModalityOptions,
  buildAgenda,
  filterAgenda,
  type AgendaFilter,
  type AgendaTab,
} from './eventsHubAgenda';

/** The `?event=` the grid's "Canchas abiertas" block links to; it opens the event's detail sheet. */
const EVENT_PARAM = 'event';

/**
 * Everything the events page reads: the complex's courts (for the open-play form), the one
 * list of both kinds for the open tab, the filters over that list, which create form is open,
 * and which event's sheet is open. The past windows are only fetched once the Pasados tab is
 * opened. Choosing the other tab clears the filters.
 */
export function useEventsHub(complexId: string) {
  const { data: courts = [] } = useCourts(complexId);
  const [tab, setTabState] = useState<AgendaTab>('upcoming');
  const [filter, setFilter] = useState<AgendaFilter>(ALL_AGENDA_FILTER);
  const [pickerOpen, setPickerOpen] = useState(false);
  const [createKind, setCreateKind] = useState<CreateEventKind | null>(null);
  const [params, setParams] = useSearchParams();

  const today = todayInArgentina();
  const eventRange = tab === 'upcoming' ? upcomingEventsRange(today) : pastEventsRange(today);
  const tournamentRange = tab === 'upcoming' ? upcomingTournamentsRange(today) : pastTournamentsRange(today);
  const events = useEvents(complexId, eventRange.from, eventRange.to);
  const tournaments = useTournaments(complexId, tournamentRange.from, tournamentRange.to);

  // The options come from every loaded row, so narrowing one filter never empties another's choices.
  const agenda = buildAgenda(events.data ?? [], tournaments.data ?? [], tab);
  const items = filterAgenda(agenda, filter);

  const setTab = (next: AgendaTab) => {
    if (next === tab) return;
    setTabState(next);
    setFilter(ALL_AGENDA_FILTER);
  };

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
    filter,
    setFilter,
    modalities: agendaModalityOptions(agenda),
    days: agendaDayOptions(agenda),
    items,
    /** Rows are loaded, but the filters hide every one of them. */
    hiddenByFilters: agenda.length > 0 && items.length === 0,
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
