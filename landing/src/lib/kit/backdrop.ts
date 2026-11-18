/**
 * The photo behind a kit picture: the complex's sport, so a padel club gets a padel court and
 * a football club a pitch. The first sport the complex lists that has a photo wins; a complex
 * with none of them gets the padel photo. Pure, so node:test can run it.
 */
import { normalizeText } from '../copy.ts';

export type KitBackdrop = 'padel' | 'futbol' | 'tenis';

/** Spanish sport labels (as the share facts carry them) that have a photo of their own. */
const BACKDROP_OF: Readonly<Record<string, KitBackdrop>> = {
  pádel: 'padel',
  fútbol: 'futbol',
  tenis: 'tenis',
};

export function kitBackdrop(sports: readonly string[]): KitBackdrop {
  for (const sport of sports) {
    const backdrop = BACKDROP_OF[normalizeText(sport).toLowerCase()];
    if (backdrop) return backdrop;
  }
  return 'padel';
}
