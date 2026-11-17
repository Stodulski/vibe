import { parseISO } from 'date-fns/parseISO';
import { ES_AR } from '@/shared/i18n/es_AR';

/**
 * The one place that turns a category's, an open-play event's or a player's
 * characteristics into words. Tournaments and canchas abiertas have no typed
 * name (S10): every screen composes its title here, so no two screens name the
 * same thing differently. Screens never show a stored `name`, except a legacy
 * category that has no modality (see `categoryTitle`).
 *
 * The modality is every sport's (S15); its words follow the sport: caballeros and
 * damas, or masculino and femenino. The level and the suma are padel's only.
 */

export type Modality = 'men' | 'women' | 'mixed';
/** Every modality, in the order a list offers them. */
export const MODALITIES: readonly Modality[] = ['men', 'women', 'mixed'];
type PlayerGender = 'men' | 'women';

const PADEL = 'padel';
/** The one sport whose open play and categories take a level range or a suma (S15). */
export const PADEL_SPORT = PADEL;
/** The sports whose players read masculino and femenino; every other sport says caballeros and damas. */
const MASCULINE_SPORTS: ReadonlySet<string> = new Set(['soccer', 'basketball', 'volleyball', 'hockey']);

const t = ES_AR.tournaments.traits;
const e = ES_AR.events;

/** Every level a player or a category can be, strongest (1ra) to weakest (9na). */
export const LEVELS = [1, 2, 3, 4, 5, 6, 7, 8, 9] as const;

/** "Caballeros", "Damas" or "Mixto" on padel, tennis and pickleball; "Masculino", "Femenino" or "Mixto" on the others. */
export function modalityLabel(modality: Modality, sport: string = PADEL): string {
  return MASCULINE_SPORTS.has(sport) ? t.modalityMasculine[modality] : t.modality[modality];
}

function playerGenderLabel(gender: PlayerGender, sport: string): string {
  return MASCULINE_SPORTS.has(sport) ? t.playerMasculine[gender] : t.player[gender];
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
  /** The tournament's sport; padel when omitted. */
  sport?: string | null | undefined;
  modality?: Modality | null | undefined;
  level?: number | null | undefined;
  suma?: number | null | undefined;
  /** The stored name, read only for a legacy category without a modality. */
  name?: string | undefined;
}

/**
 * Padel: "Caballeros 8va", "Damas 5ta", "Mixto Suma 15". Any other sport: the
 * modality's label alone, "Masculino". A category without a modality (created
 * before the traits) keeps the name it was given.
 */
export function categoryTitle(category: CategoryTraits): string {
  const { modality, level, suma } = category;
  if (!modality) return category.name ?? '';
  const sport = category.sport ?? PADEL;
  const label = modalityLabel(modality, sport);
  if (sport !== PADEL) return label;
  if (level != null) return `${label} ${levelLabel(level)}`;
  if (suma != null) return `${label} ${t.suma} ${String(suma)}`;
  return label;
}

export interface OpenPlayTraits {
  sport: string;
  modality?: Modality | null | undefined;
  level_min?: number | null | undefined;
  level_max?: number | null | undefined;
}

/**
 * "Cancha abierta de pádel · Caballeros · 6ta a 8va", or "· solo 4ta" for a
 * single level. Another sport's open play has its modality and no range. An event
 * created before the traits has only the sport.
 */
/** What an open play is and where: "Cancha abierta de pádel". */
export function openPlayKind(event: Pick<OpenPlayTraits, 'sport'>): string {
  return e.openPlayTitle(sportTitleLabel(event.sport));
}

/** Who an open play is for: "Damas · 6ta a 8va", "Masculino", or "Abierta a todos". */
export function openPlayAudience(event: OpenPlayTraits): string {
  const parts: string[] = [];
  if (event.modality) parts.push(modalityLabel(event.modality, event.sport));
  if (event.level_min != null && event.level_max != null) {
    const from = levelLabel(event.level_min);
    const to = levelLabel(event.level_max);
    parts.push(event.level_min === event.level_max ? e.levelSolo(from) : e.levelRange(from, to));
  }
  return parts.length > 0 ? parts.join(' · ') : e.openToEveryone;
}

export function openPlayTitle(event: OpenPlayTraits): string {
  const audience = openPlayAudience(event);
  return audience === e.openToEveryone ? openPlayKind(event) : `${openPlayKind(event)} · ${audience}`;
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
  const date = `${ES_AR.common.weekdaysShort[day.getDay()] ?? ''} ${String(day.getDate())} ${ES_AR.common.monthsShort[day.getMonth()] ?? ''}`;
  return startsAt ? `${date} · ${startsAt}` : date;
}

export interface PlayerTraits {
  /** The sport of the tournament or event the player signed up to; padel when omitted. */
  sport?: string | null | undefined;
  gender?: PlayerGender | null | undefined;
  level?: number | null | undefined;
}

/** "Caballero · 7ma": what a player declared at sign-up, showing only what was declared. */
export function playerTraitsLabel(player: PlayerTraits): string | null {
  const parts: string[] = [];
  if (player.gender) parts.push(playerGenderLabel(player.gender, player.sport ?? PADEL));
  if (player.level != null) parts.push(levelLabel(player.level));
  return parts.length > 0 ? parts.join(' · ') : null;
}
