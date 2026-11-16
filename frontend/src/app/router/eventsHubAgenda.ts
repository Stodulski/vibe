import type { OpenPlayEvent, TournamentOwnerSummary } from '@/shared/types/api.types';

export type AgendaTab = 'upcoming' | 'past';

interface AgendaBase {
  key: string;
  sortKey: string;
}

export type AgendaItem =
  | (AgendaBase & { kind: 'openPlay'; event: OpenPlayEvent })
  | (AgendaBase & { kind: 'tournament'; tournament: TournamentOwnerSummary });

/** ISO-like strings ("2099-01-02T19:00") sort by their text exactly as they sort by date. */
function compareText(a: string, b: string): number {
  if (a === b) return 0;
  return a < b ? -1 : 1;
}

/**
 * The one list of the events page: open-play events and tournaments ordered by when they
 * start. Upcoming reads soonest first; past reads the latest first.
 */
export function buildAgenda(
  events: OpenPlayEvent[],
  tournaments: TournamentOwnerSummary[],
  tab: AgendaTab,
): AgendaItem[] {
  const items: AgendaItem[] = [
    ...events.map((event): AgendaItem => ({
      key: `openPlay-${event.id}`,
      sortKey: `${event.date}T${event.start_time}`,
      kind: 'openPlay',
      event,
    })),
    ...tournaments.map((tournament): AgendaItem => ({
      key: `tournament-${tournament.id}`,
      sortKey: `${tournament.start_date}T00:00`,
      kind: 'tournament',
      tournament,
    })),
  ];
  const direction = tab === 'upcoming' ? 1 : -1;
  return items.sort((a, b) => direction * compareText(a.sortKey, b.sortKey));
}
