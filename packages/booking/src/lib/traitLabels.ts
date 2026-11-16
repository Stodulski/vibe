import { BOOKING_MESSAGES } from '../messages';
import type { Modality, PublicOpenPlayEvent, PublicTournamentCategory } from '../api/types';

/**
 * The one place the traits of a category or an open-play event are written as
 * words: who plays (modality), at what level, and the titles composed from
 * them. Screens never show a stored tournament or event name; they compose
 * from these. The words follow the sport: pádel, tenis and pickleball say
 * Caballeros and Damas, fútbol, básquet, vóley and hockey say Masculino and
 * Femenino. Only pádel has levels and Sumas.
 */

/** The ordinal name of each level, 1ra (1) to 9na (9). */
const LEVEL_LABELS = ['1ra', '2da', '3ra', '4ta', '5ta', '6ta', '7ma', '8va', '9na'] as const;

const SPORT_LABELS: Record<string, string> = BOOKING_MESSAGES.courts.sportTypes;

/** The words a sport family uses for a modality, for the player who declares it and for the pair it asks for. */
export interface DeclarationWords {
  /** The modality as a title: "Caballeros", "Masculino". */
  modality: Record<Modality, string>;
  /** One player's gender as the choice names it. */
  gender: Record<'men' | 'women', string>;
  /** The legend over that choice: "Caballero / Dama". */
  genderLabel: string;
  /** What completes "Elegí si es ___": "caballero o dama". */
  genderOr: string;
  /** Both players of a pair, in a sentence: "los dos jugadores tienen que ser ___". */
  playersOf: Record<'men' | 'women', string>;
  /** A mixed pair, in a sentence: "tiene que ser ___". */
  mixedPair: string;
}

const RACKET_WORDS: DeclarationWords = {
  modality: { men: 'Caballeros', women: 'Damas', mixed: 'Mixto' },
  gender: { men: 'Caballero', women: 'Dama' },
  genderLabel: 'Caballero / Dama',
  genderOr: 'caballero o dama',
  playersOf: { men: 'caballeros', women: 'damas' },
  mixedPair: 'un caballero y una dama',
};

const TEAM_WORDS: DeclarationWords = {
  modality: { men: 'Masculino', women: 'Femenino', mixed: 'Mixto' },
  gender: { men: 'Masculino', women: 'Femenino' },
  genderLabel: 'Masculino / Femenino',
  genderOr: 'masculino o femenino',
  playersOf: { men: 'masculinos', women: 'femeninos' },
  mixedPair: 'un jugador masculino y una jugadora femenina',
};

/** The sports that say Caballeros and Damas. Every other sport says Masculino and Femenino. */
const RACKET_SPORTS: ReadonlySet<string> = new Set(['padel', 'tennis', 'pickleball']);

/** The words of a sport: its modality names and what a declaration asks in them. */
export function declarationWords(sport: string): DeclarationWords {
  return RACKET_SPORTS.has(sport) ? RACKET_WORDS : TEAM_WORDS;
}

/** Only pádel has levels and Sumas. The other sports name their categories and events by their modality alone. */
export function hasLevels(sport: string): boolean {
  return sport === 'padel';
}

/** Every modality, in the order the filters offer them. */
export const MODALITIES: readonly Modality[] = ['men', 'women', 'mixed'];

/** Whether a value is one of the modalities this build knows. A stored value it does not know reads as no modality. */
export function isModality(value: string | null | undefined): value is Modality {
  return value === 'men' || value === 'women' || value === 'mixed';
}

/** The modality in the words of the sport: "Caballeros" or "Damas" for pádel, "Masculino" or "Femenino" for fútbol. */
export function modalityLabel(modality: Modality, sport: string): string {
  return declarationWords(sport).modality[modality];
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
 * A category's title: "Caballeros 8va" or "Mixto Suma 15" for pádel, and the
 * modality alone ("Masculino") for a sport without levels. A category that
 * predates the traits keeps the name it was given, as does a pádel one whose
 * traits do not name it.
 */
export function categoryTitle(
  category: Pick<PublicTournamentCategory, 'name' | 'modality' | 'level' | 'suma'>,
  sport: string,
): string {
  if (!isModality(category.modality)) return category.name;
  const who = modalityLabel(category.modality, sport);
  if (!hasLevels(sport)) return who;
  if (typeof category.level === 'number') return `${who} ${levelLabel(category.level)}`;
  if (typeof category.suma === 'number') return `${who} Suma ${String(category.suma)}`;
  return category.name;
}

/**
 * An open-play event's title, which replaces its stored name everywhere:
 * "Cancha abierta de pádel", then its modality and level range when it has them,
 * e.g. "Cancha abierta de pádel · Caballeros · 6ta a 8va". A sport without
 * levels is titled by its modality alone, e.g. "Femenino".
 */
export function openPlayTitle(
  event: Pick<PublicOpenPlayEvent, 'sport' | 'modality' | 'level_min' | 'level_max'>,
): string {
  const modality = isModality(event.modality) ? event.modality : null;
  if (modality && !hasLevels(event.sport)) return modalityLabel(modality, event.sport);
  const parts = [`Cancha abierta de ${sportNoun(event.sport)}`];
  if (modality) parts.push(modalityLabel(modality, event.sport));
  const { level_min: min, level_max: max } = event;
  if (typeof min === 'number' && typeof max === 'number') parts.push(levelRangeLabel(min, max));
  return parts.join(' · ');
}
