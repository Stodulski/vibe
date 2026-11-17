import type { OpenPlayEvent, TournamentOwnerSummary } from '@/shared/types/api.types';
import { eventDayParts } from '@/shared/lib/eventDates';
import { MODALITIES, modalityLabel, type Modality } from '@/shared/lib/traitLabels';

export type AgendaTab = 'upcoming' | 'past';

interface AgendaBase {
  key: string;
  sortKey: string;
}

export type AgendaItem =
  | (AgendaBase & { kind: 'openPlay'; event: OpenPlayEvent })
  | (AgendaBase & { kind: 'tournament'; tournament: TournamentOwnerSummary });

/** The selects above the list. `all` on a filter narrows nothing; a `null` day is every day. */
export interface AgendaFilter {
  kind: 'all' | AgendaItem['kind'];
  modality: 'all' | Modality;
  /** A day (`YYYY-MM-DD`), or `null` for every day. */
  day: string | null;
}

export const ALL_AGENDA_FILTER: AgendaFilter = { kind: 'all', modality: 'all', day: null };

export interface AgendaModalityOption {
  value: Modality;
  /** The modality in the words of the sports that have it: "Caballeros", "Masculino". */
  label: string;
}

export interface AgendaDayOption {
  date: string;
  /** "Mié 18 nov" */
  label: string;
}

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

/** The sport a row is played in: its open play's, or its tournament's. */
function sportOf(item: AgendaItem): string {
  return item.kind === 'openPlay' ? item.event.sport : item.tournament.sport;
}

/**
 * The modalities a row has. A tournament has the modalities of its live categories: a
 * cancelled category is not on the row, so it does not count.
 */
function modalitiesOf(item: AgendaItem): Modality[] {
  if (item.kind === 'openPlay') return item.event.modality ? [item.event.modality] : [];
  return item.tournament.categories.flatMap((category) =>
    category.status !== 'cancelled' && category.modality ? [category.modality] : [],
  );
}

/** The first and the last day a row covers: an open play covers its own day, a tournament its days. */
function coverageOf(item: AgendaItem): { first: string; last: string } {
  if (item.kind === 'openPlay') return { first: item.event.date, last: item.event.date };
  return { first: item.tournament.start_date, last: item.tournament.end_date };
}

/** The rows that match every filter that narrows something. */
export function filterAgenda(items: AgendaItem[], filter: AgendaFilter): AgendaItem[] {
  return items.filter((item) => {
    if (filter.kind !== 'all' && item.kind !== filter.kind) return false;
    if (filter.modality !== 'all' && !modalitiesOf(item).includes(filter.modality)) return false;
    if (filter.day === null) return true;
    const { first, last } = coverageOf(item);
    return first <= filter.day && filter.day <= last;
  });
}

/**
 * One option per modality the rows have, in the order of the modalities. Each is named in
 * the words of the sports that have it, and both when two sports name it differently.
 */
export function agendaModalityOptions(items: AgendaItem[]): AgendaModalityOption[] {
  const labels = new Map<Modality, Set<string>>();
  for (const item of items) {
    for (const modality of modalitiesOf(item)) {
      const words = labels.get(modality) ?? new Set<string>();
      words.add(modalityLabel(modality, sportOf(item)));
      labels.set(modality, words);
    }
  }
  return MODALITIES.flatMap((value) => {
    const words = labels.get(value);
    return words ? [{ value, label: [...words].join(' / ') }] : [];
  });
}

/** One option per day a row starts on, soonest first, so a tournament's days are the days it starts. */
export function agendaDayOptions(items: AgendaItem[]): AgendaDayOption[] {
  const dates = [...new Set(items.map((item) => coverageOf(item).first))].sort();
  return dates.map((date) => {
    const parts = eventDayParts(date);
    return { date, label: `${parts.weekday} ${parts.day} ${parts.month}` };
  });
}
