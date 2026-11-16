import { parseISO } from 'date-fns/parseISO';
import { ES_AR } from '@/shared/i18n/es_AR';

/**
 * The one place that turns a category's, an open-play event's or a player's
 * characteristics into words. Tournaments and canchas abiertas have no typed
 * name (S10): every screen composes its title here, so no two screens name the
 * same thing differently. Screens never show a stored `name`, except a legacy
 * category that has no modality (see `categoryTitle`).
 */

export type Modality = 'men' | 'women' | 'mixed';
type PlayerGender = 'men' | 'women';

const t = ES_AR.tournaments.traits;
const e = ES_AR.events;

/** Every level a player or a category can be, strongest (1ra) to weakest (9na). */
export const LEVELS = [1, 2, 3, 4, 5, 6, 7, 8, 9] as const;

export function modalityLabel(modality: Modality): string {
  return t.modality[modality];
}

function playerGenderLabel(gender: PlayerGender): string {
  return t.player[gender];
}

/** "1ra" for 1 up to "9na" for 9. */
export function levelLabel(level: number): string {
  return t.levels[level - 1] ?? String(level);
}

/** The sport as it reads inside a title: "pádel", "fútbol". */
function sportTitleLabel(sport: string): string {
  const label = (ES_AR.courts.sportTypes as Record<string, string>)[sport] ?? sport;
  return label.toLowerCase();
}

export interface CategoryTraits {
  modality?: Modality | null | undefined;
  level?: number | null | undefined;
  suma?: number | null | undefined;
  /** The stored name, read only for a legacy category without a modality. */
  name?: string | undefined;
}

/**
 * "Caballeros 8va", "Damas 5ta", "Mixto Suma 15". A category without a modality
 * (created before the traits) keeps the name it was given.
 */
export function categoryTitle(category: CategoryTraits): string {
  const { modality, level, suma } = category;
  if (modality) {
    if (level != null) return `${modalityLabel(modality)} ${levelLabel(level)}`;
    if (suma != null) return `${modalityLabel(modality)} ${t.suma} ${String(suma)}`;
  }
  return category.name ?? '';
}

export interface OpenPlayTraits {
  sport: string;
  modality?: Modality | null | undefined;
  level_min?: number | null | undefined;
  level_max?: number | null | undefined;
}

/**
 * "Cancha abierta de pádel · Caballeros · 6ta a 8va", or "· solo 4ta" for a
 * single level. An event created before the traits has only the sport.
 */
export function openPlayTitle(event: OpenPlayTraits): string {
  const parts = [e.openPlayTitle(sportTitleLabel(event.sport))];
  if (event.modality) parts.push(modalityLabel(event.modality));
  if (event.level_min != null && event.level_max != null) {
    const from = levelLabel(event.level_min);
    const to = levelLabel(event.level_max);
    parts.push(event.level_min === event.level_max ? e.levelSolo(from) : e.levelRange(from, to));
  }
  return parts.join(' · ');
}

/** "Torneo de pádel". The dates sit beside the title, not inside it. */
export function tournamentTitle(sport: string): string {
  return t.tournamentTitle(sportTitleLabel(sport));
}

/**
 * "Sáb 21 nov · 18:00": when a category starts, its time when it has one. Null
 * for a legacy category without a day.
 */
export function categoryStartLabel(startsOn: string | null | undefined, startsAt?: string | null): string | null {
  if (!startsOn) return null;
  const day = parseISO(startsOn);
  const date = `${t.weekdays[day.getDay()] ?? ''} ${String(day.getDate())} ${t.months[day.getMonth()] ?? ''}`;
  return startsAt ? `${date} · ${startsAt}` : date;
}

export interface PlayerTraits {
  gender?: PlayerGender | null | undefined;
  level?: number | null | undefined;
}

/** "Caballero · 7ma": what a player declared at sign-up, showing only what was declared. */
export function playerTraitsLabel(player: PlayerTraits): string | null {
  const parts: string[] = [];
  if (player.gender) parts.push(playerGenderLabel(player.gender));
  if (player.level != null) parts.push(levelLabel(player.level));
  return parts.length > 0 ? parts.join(' · ') : null;
}
