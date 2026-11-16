import { useState } from 'react';
import { todayInArgentina } from '@/shared/lib/today';
import { useTournaments } from '../../hooks/useTournaments';
import { pastRange, upcomingRange } from '../../lib/tournamentRanges';

export type TournamentsTab = 'upcoming' | 'past';

/** The list of the open tab and the create dialog's state. The past list is only fetched once its tab is opened. */
export function useTournamentsPage(complexId: string) {
  const [tab, setTab] = useState<TournamentsTab>('upcoming');
  const [createOpen, setCreateOpen] = useState(false);

  const today = todayInArgentina();
  const range = tab === 'upcoming' ? upcomingRange(today) : pastRange(today);
  const list = useTournaments(complexId, range.from, range.to);

  return {
    tab,
    setTab,
    // The API sends soonest first; the past reads best with the latest on top.
    tournaments: tab === 'upcoming' ? (list.data ?? []) : [...(list.data ?? [])].reverse(),
    isLoading: list.isLoading,
    isError: list.isError,
    refetch: list.refetch,
    createOpen,
    setCreateOpen,
  };
}
