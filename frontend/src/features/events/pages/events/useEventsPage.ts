import { useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { useCourts } from '@/features/courts';
import { todayInArgentina } from '@/shared/lib/today';
import { useEvents } from '../../hooks/useEvents';
import { pastRange, upcomingRange } from '../../lib/eventRanges';

export type EventsTab = 'upcoming' | 'past';

/**
 * The `?event=` the grid's "Canchas libres" block links to (`BookingsPage`
 * writes it as a literal: features do not import each other); it opens the
 * detail sheet.
 */
const EVENT_PARAM = 'event';

/**
 * Everything the page reads: the complex's courts (for the create form), the
 * list of the open tab, and which event's sheet is open. The past list is only
 * fetched once its tab is opened.
 */
export function useEventsPage(complexId: string) {
  const { data: courts = [] } = useCourts(complexId);
  const [tab, setTab] = useState<EventsTab>('upcoming');
  const [createOpen, setCreateOpen] = useState(false);
  const [params, setParams] = useSearchParams();

  const today = todayInArgentina();
  const range = tab === 'upcoming' ? upcomingRange(today) : pastRange(today);
  const list = useEvents(complexId, range.from, range.to);

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
    // The API sends soonest first; the past reads best with the latest on top.
    events: tab === 'upcoming' ? (list.data ?? []) : [...(list.data ?? [])].reverse(),
    isLoading: list.isLoading,
    isError: list.isError,
    refetch: list.refetch,
    createOpen,
    setCreateOpen,
    selectedEventId,
    selectEvent,
  };
}
