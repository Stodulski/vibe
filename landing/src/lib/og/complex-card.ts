/**
 * The words a complex's share image draws: the name, the place and the sports line. Pure, so
 * node:test can run it; the layout that places these words lives in render.ts.
 */
import { normalizeText } from '../copy.ts';
import type { ShareFacts } from '../complex-share.ts';

export interface ComplexCardText {
  name: string;
  /** "City, Province", with whichever of the two the complex has. */
  place: string;
  /** The sports joined with a middle dot, in the order the complex lists them. */
  sports: string;
}

export function complexCardText(facts: ShareFacts): ComplexCardText {
  return {
    name: normalizeText(facts.name),
    place: [normalizeText(facts.city), normalizeText(facts.province)].filter((part) => part !== '').join(', '),
    sports: facts.sports
      .map(normalizeText)
      .filter((sport) => sport !== '')
      .join(' · '),
  };
}
