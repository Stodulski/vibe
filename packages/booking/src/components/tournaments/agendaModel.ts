import type { PublicTournamentCategory, PublicTournamentSummary } from '../../api/types';
import { BOOKING_MESSAGES } from '../../messages';
import { categoryTitle, tournamentLabel } from '../../lib/traitLabels';
import { longDayLabel, spotsNotice as sharedSpotsNotice, type SpotsNotice } from '../agenda/agendaModel';
import { formatLabel, isOpenForSignUp, spotsLeftLabel } from './tournamentModel';

const t = BOOKING_MESSAGES.tournaments;

/** One category on the Eventos list: when it starts, what it is called and which tournament it belongs to. */
export interface AgendaRow {
  category: PublicTournamentCategory;
  tournamentId: string;
  /** The tournament's sport: the words its modality is named in. */
  sport: string;
  /** The small label above the title: "Torneo de pádel". */
  label: string;
  /** The day the category starts (`YYYY-MM-DD`): its own, or its tournament's first day when it has none. */
  date: string;
  /** `HH:MM`, or `null` for a category created before it had a time. */
  time: string | null;
  title: string;
}

function compareTimes(a: string | null, b: string | null): number {
  if (a === b) return 0;
  // Within one day, a category without a time comes after the ones with one.
  if (a === null) return 1;
  if (b === null) return -1;
  return a.localeCompare(b);
}

function byWhen(a: AgendaRow, b: AgendaRow): number {
  return (
    a.date.localeCompare(b.date) ||
    compareTimes(a.time, b.time) ||
    a.title.localeCompare(b.title, 'es-AR') ||
    a.category.id.localeCompare(b.category.id)
  );
}

/**
 * The tournament categories open for sign-up with a place left, of a tournament
 * that is active, sorted by day and time. A category that predates its start
 * takes its tournament's first day and no time.
 */
export function agendaRows(tournaments: readonly PublicTournamentSummary[]): AgendaRow[] {
  const rows: AgendaRow[] = [];
  for (const tournament of tournaments) {
    if (tournament.status !== 'active') continue;
    for (const category of tournament.categories) {
      if (!isOpenForSignUp(category)) continue;
      rows.push({
        category,
        tournamentId: tournament.id,
        sport: tournament.sport,
        label: tournamentLabel(tournament.sport),
        date: category.starts_on ?? tournament.start_date,
        time: category.starts_at,
        title: categoryTitle(category, tournament.sport),
      });
    }
  }
  return rows.sort(byWhen);
}

/** What a row says about its places: how many are left, or that they are the last ones. */
export function spotsNotice(spotsLeft: number): SpotsNotice {
  return sharedSpotsNotice(spotsLeft, { lastSpots: t.lastSpots, spotsLeft: spotsLeftLabel });
}

/**
 * The format as a list, one fact a line: "Zonas y cuadro", "Zonas de 4 parejas", "Pasan 2 por zona".
 * An elimination is its format alone.
 */
export function formatDetailLines(
  category: Pick<PublicTournamentCategory, 'format' | 'group_size' | 'qualifiers_per_group'>,
): string[] {
  const label = formatLabel(category.format);
  if (category.format !== 'groups') return [label];
  return [
    label,
    `${t.zonesOf} ${String(category.group_size)} ${t.zonesPairs}`,
    `${t.qualifiersPass} ${String(category.qualifiers_per_group)} ${t.qualifiersPerZone}`,
  ];
}

/** "Sábado 05/12 · 18:00 hs": the day and, when it has one, the time. */
export function categoryWhen(row: Pick<AgendaRow, 'date' | 'time'>): string {
  const day = longDayLabel(row.date);
  return row.time === null ? day : `${day} · ${row.time} ${t.hoursSuffix}`;
}
