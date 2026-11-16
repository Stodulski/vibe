import { BOOKING_MESSAGES } from '../messages';
import type { Modality, PublicOpenPlayEvent, PublicTournamentCategory } from '../api/types';

/**
 * The one place the traits of a category or an open-play event are written as
 * words: who plays (modality), at what level, and the titles composed from
 * them. Screens never show a stored tournament or event name; they compose
 * from these.
 */

/** The ordinal name of each level, 1ra (1) to 9na (9). */
const LEVEL_LABELS = ['1ra', '2da', '3ra', '4ta', '5ta', '6ta', '7ma', '8va', '9na'] as const;

const MODALITY_LABELS: Record<Modality, string> = { men: 'Caballeros', women: 'Damas', mixed: 'Mixto' };

const SPORT_LABELS: Record<string, string> = BOOKING_MESSAGES.courts.sportTypes;

/** Every modality, in the order the filters offer them. */
export const MODALITIES: readonly Modality[] = ['men', 'women', 'mixed'];

/** Whether a value is one of the modalities this build knows. A stored value it does not know reads as no modality. */
export function isModality(value: string | null | undefined): value is Modality {
  return value === 'men' || value === 'women' || value === 'mixed';
}

/** Caballeros, Damas or Mixto. */
export function modalityLabel(modality: Modality): string {
  return MODALITY_LABELS[modality];
}

/** "1ra" to "9na"; a number outside the nine this build knows is written as it is. */
export function levelLabel(level: number): string {
  return LEVEL_LABELS[level - 1] ?? String(level);
}

/** A range of levels in words: "6ta a 8va", or "solo 4ta" for a single level. */
export function levelRangeLabel(min: number, max: number): string {
  return min === max ? `solo ${levelLabel(min)}` : `${levelLabel(min)} a ${levelLabel(max)}`;
}

/** The sport's name in running text: "pádel", "fútbol". */
function sportNoun(sport: string): string {
  return (SPORT_LABELS[sport] ?? sport).toLocaleLowerCase('es-AR');
}

/** The small label above a tournament's title: "Torneo de pádel". */
export function tournamentLabel(sport: string): string {
  return `Torneo de ${sportNoun(sport)}`;
}

/**
 * A category's title: "Caballeros 8va" or "Mixto Suma 15". A category that
 * predates the traits keeps the name it was given, as does one whose traits do
 * not name it.
 */
export function categoryTitle(
  category: Pick<PublicTournamentCategory, 'name' | 'modality' | 'level' | 'suma'>,
): string {
  if (!isModality(category.modality)) return category.name;
  const who = MODALITY_LABELS[category.modality];
  if (typeof category.level === 'number') return `${who} ${levelLabel(category.level)}`;
  if (typeof category.suma === 'number') return `${who} Suma ${String(category.suma)}`;
  return category.name;
}

/**
 * An open-play event's title, which replaces its stored name everywhere:
 * "Cancha abierta de pádel", then its modality and level range when it has them,
 * e.g. "Cancha abierta de pádel · Caballeros · 6ta a 8va".
 */
export function openPlayTitle(
  event: Pick<PublicOpenPlayEvent, 'sport' | 'modality' | 'level_min' | 'level_max'>,
): string {
  const parts = [`Cancha abierta de ${sportNoun(event.sport)}`];
  if (isModality(event.modality)) parts.push(MODALITY_LABELS[event.modality]);
  const { level_min: min, level_max: max } = event;
  if (typeof min === 'number' && typeof max === 'number') parts.push(levelRangeLabel(min, max));
  return parts.join(' · ');
}
