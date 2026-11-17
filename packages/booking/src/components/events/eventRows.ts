import { dayChips, matchesAgendaFilter, modalityOptions, type AgendaFilter } from '../agenda/agendaModel';
import type { AgendaDetail } from '../agenda/AgendaDetailDialog';
import type { AgendaRow as TournamentRow } from '../tournaments/agendaModel';
import { tournamentDetail } from '../tournaments/agendaDetail';
import { openPlayDetail, type OpenPlayRow } from './openPlayAgendaModel';

/**
 * The Eventos list: the open play and the tournament categories of a complex in
 * one list, told apart by their labels.
 */
export type EventRow = { kind: 'open-play'; row: OpenPlayRow } | { kind: 'tournament'; row: TournamentRow };

function idOf(item: EventRow): string {
  return item.kind === 'open-play' ? item.row.event.id : item.row.category.id;
}

function timeOf(item: EventRow): string | null {
  return item.kind === 'open-play' ? item.row.event.start_time : item.row.time;
}

function modalityOf(item: EventRow): string | null | undefined {
  return item.kind === 'open-play' ? item.row.event.modality : item.row.category.modality;
}

function sportOf(item: EventRow): string {
  return item.kind === 'open-play' ? item.row.event.sport : item.row.sport;
}

/** A row without a time sorts after the timed rows of its day. */
function compareTimes(a: string | null, b: string | null): number {
  if (a === b) return 0;
  if (a === null) return 1;
  if (b === null) return -1;
  return a.localeCompare(b);
}

/** Soonest first: by day, then by start time, then by title, so the order never depends on the input. */
function compareStart(a: EventRow, b: EventRow): number {
  return (
    a.row.date.localeCompare(b.row.date) ||
    compareTimes(timeOf(a), timeOf(b)) ||
    a.row.title.localeCompare(b.row.title, 'es-AR') ||
    a.kind.localeCompare(b.kind) ||
    idOf(a).localeCompare(idOf(b))
  );
}

/** The one list of a complex's events: its open play and its tournament categories, soonest first. */
export function eventRows(openPlay: readonly OpenPlayRow[], tournaments: readonly TournamentRow[]): EventRow[] {
  const merged: EventRow[] = [
    ...openPlay.map((row): EventRow => ({ kind: 'open-play', row })),
    ...tournaments.map((row): EventRow => ({ kind: 'tournament', row })),
  ];
  return merged.sort(compareStart);
}

/** The rows that match a modality and a day, across both kinds. A row without a modality is only in "all". */
export function filterEventRows(rows: readonly EventRow[], filter: AgendaFilter): EventRow[] {
  return rows.filter((item) => matchesAgendaFilter(modalityOf(item), item.row.date, filter));
}

/** The modality options of the rows, each named in the words of its sport. */
export function eventModalityOptions(rows: readonly EventRow[]) {
  return modalityOptions(rows.map((item) => ({ modality: modalityOf(item), sport: sportOf(item) })));
}

/** One day option per day the rows fall on, of either kind. */
export function eventDayChips(rows: readonly EventRow[]) {
  return dayChips(rows.map((item) => ({ date: item.row.date })));
}

/** A key unique across the list, whichever kind a row is. */
export function eventRowKey(item: EventRow): string {
  return `${item.kind}:${idOf(item)}`;
}

/** The detail of a row, whichever kind it is. */
export function eventDetailOf(item: EventRow, slug: string): AgendaDetail {
  return item.kind === 'open-play' ? openPlayDetail(item.row, slug) : tournamentDetail(item.row, slug);
}
